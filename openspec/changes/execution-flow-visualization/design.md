# Design: Execution Flow Visualization — Phase 1 (Structural Flow)

## Context

nano-brain extracts symbols and a knowledge graph (`contains`/`imports`/`calls`/`references` edges) via per-file tree-sitter extractors, persisted to the `graph_edges` table by the watcher. Search is hybrid BM25 + vector + RRF over documents. Tools `memory_trace`/`memory_graph`/`memory_impact` traverse the graph from a symbol node.

This design adds **HTTP-endpoint-anchored execution flows**: it recognizes the entry points of a web service (Echo routes), links them to handler call chains already in the graph, and surfaces the result as both a searchable artifact and a renderable flowchart.

### Decisions locked during brainstorming (2026-06-13)
- **Goal:** serve agent search *and* human diagrams equally → hybrid storage.
- **Storage:** edges always persisted; a lightweight flow-summary **document** is materialized per endpoint for search; full Mermaid is rendered **on demand** from live edges.
- **Entry points:** Go/Echo HTTP routes **only** in Phase 1. (Structured so other detectors are easy to add later, but none ship now.)
- **Diagram:** **flowchart** (`graph TD`) only. Sequence diagrams are Phase 2.
- **Surface:** new `memory_flow` MCP tool + `POST /api/v1/graph/flow`.
- **Handler resolution:** best-effort, **log on failure**, never silently drop.
- **Materialization:** **workspace-level** post-index pass (not per-file), because flows are cross-file.
- **Cross-service:** **out of scope** for Phase 1 (kept minimal). See "Why cross-service is excluded".

## Goals / Non-Goals

**Goals**
- Statically derive, for each Echo HTTP endpoint in a single repo, its in-process flow: route → middleware → handler → service → repo → external leaf.
- Make every endpoint flow searchable via `memory_query` and renderable as a Mermaid flowchart via `memory_flow`.
- Keep all three compute units (extract, build, render) pure and independently testable.

**Non-Goals (deferred)**
- Sequence diagrams; LLM/semantic flow summaries; conditional-branch (if/else) paths → **Phase 2**.
- Non-Echo entry points (Gin, net/http, Express, FastAPI, CLI, cron, queue consumers) → **Phase 2+**.
- Integration-point edges (publish/consume/emit/listen/external-HTTP) and cross-workspace stitching → **Phase 2**.
- Distributed/runtime tracing (OpenTelemetry), data-flow analysis → **Phase 3**.

## Architecture

```
INDEX TIME (per file, existing watcher hook):
  *.go ──▶ EchoRouteExtractor ──▶ [http, middleware] edges ──▶ graph_edges
  *.go ──▶ existing extractors  ──▶ [calls, contains, imports] edges (unchanged)

POST-INDEX (per workspace, debounced after scan settles):
  graph_edges ──▶ FlowMaterializer
                    for each http entry node:
                      FlowBuilder(edges, entry) ──▶ Flow tree
                      flow → text summary ──▶ documents (+chunks +embed) ──▶ searchable

QUERY TIME:
  memory_flow(entry) ──▶ load workspace edges ──▶ FlowBuilder ──▶ Flow
                    ──▶ MermaidFlowchart(Flow) ──▶ { chain, mermaid, externals }
```

### Units (each independently testable)

1. **EchoRouteExtractor** (`internal/graph/echo_extractor.go`) — `file → []Edge`. Tree-sitter (Go). Implements the existing `graph.Extractor` interface (`ExtractEdges`, `Supports`). Pure given file bytes.
   - **Registry change required:** `registry.go` `ExtractEdges` currently `return`s after the **first** supporting extractor. Since `go_extractor` already claims `.go`, a second `.go` extractor would never run. Change the registry to run **all** supporting extractors and concatenate their edges (de-duplicating identical edges). Existing single-extractor-per-ext behavior is preserved (one match → same result).
2. **FlowBuilder** (`internal/flow/builder.go`) — `(edges, entry, maxDepth) → Flow`. Pure graph traversal, no DB, no rendering.
3. **MermaidFlowchart** (`internal/flow/mermaid.go`) — `Flow → string`. Pure. Golden-file tested.
4. **FlowMaterializer** (`internal/flow/materializer.go`) — orchestration only: loads edges, calls FlowBuilder, writes/refreshes flow documents. The impure shell around the pure core.

## Data model

### Edge types (`internal/graph/edge.go`)
```go
const (
    EdgeContains   EdgeKind = "contains"
    EdgeImports    EdgeKind = "imports"
    EdgeCalls      EdgeKind = "calls"
    EdgeHTTP       EdgeKind = "http"        // NEW: "<METHOD> <path>" → handler symbol
    EdgeMiddleware EdgeKind = "middleware"  // NEW: middleware symbol → handler symbol
)

type Edge struct {
    SourceNode string
    TargetNode string
    Kind       EdgeKind
    SourceFile string
    Line       int
    Language   string
    Metadata   map[string]any // NEW: extractor-supplied; e.g. {method, path}
}
```

### Persistence
- **Migration number: `00024_flow_edge_types.sql`.** (Verified: latest migration is `00023_bm25_configurable_language.sql`; `00013` is already taken — the earlier draft's `00013` was wrong.) Extends the `graph_edges.edge_type` CHECK to `('contains','imports','calls','references','http','middleware')` — mirrors migration `00012`'s pattern, including a Down that documents the delete-first requirement (`DELETE FROM graph_edges WHERE edge_type IN ('http','middleware')`).
- **`GraphStats` must be extended.** The current query (`graph.sql`) hardcodes `FILTER (WHERE edge_type = 'contains'|'imports'|'calls')` and would silently omit the new edge types. Add `http_count`/`middleware_count` and surface them in `stats.go` and its response struct. PageRank/degree analytics (`00021_pagerank_importance.sql`): Phase-1 decision is to **exclude `http`/`middleware` edges from PageRank importance** (they are entry/guard structure, not call-importance signal) — filter them where PageRank reads edges, or confirm it already scopes to `calls`.
- `extractAndUpsertEdges` (watcher) currently writes `metadata = {line, language}`, **discarding** other fields. Change: start from `e.Metadata` (if any), then set/merge `line` and `language`, marshal the union. Existing extractors set no `Metadata`, so their behavior is unchanged.

### Node id convention — and the call-graph transitivity reality

**Verified against the live graph (2026-06-13):** the existing `calls` edges are keyed asymmetrically — `source_node = "<file>::<FuncName>"` but `target_node = <bare callee name>` (`go_extractor.go:211-213`). **Zero** source nodes are bare. Consequently `memory_trace`'s traversal (`trace.go` `traceCallChain`), which calls `GetOutgoingEdges(source_node = currentNode)` and then recurses on the *bare* `target_node`, **dead-ends after one hop** — a bare name like `runAuthHash` never matches a `<file>::<func>` source. The call graph is therefore *not transitively traversable by exact node id* as stored today.

- **Implication:** a FlowBuilder that mimics trace (exact `source_node` match) would yield only `route → handler → its direct callees`, then stop — shallow, low-value flows. This is the single most important correction from review.
- **Resolution (verified feasible):** 928 of 1643 distinct `calls` targets (~56%) match the *symbol part* of some `source_node` (`split_part(source_node,'::',2)`); the remaining ~44% are external/stdlib leaves (`Fprintf`, `len`, …). So the builder MUST traverse by **symbol-name reconciliation**: from a node, follow `calls` to a bare target name, then resolve that name to source nodes via `source_node LIKE '%::' || name` to continue. This needs a new query (see tasks) and must handle **ambiguity** (the same function name defined in N files → N candidate continuations). Phase-1 policy: when a bare name resolves to multiple source files, include all candidates but de-duplicate by node and cap fan-out; record ambiguity in flow metadata. Names that resolve to nothing are terminal `external` leaves.
- **Bonus:** this same reconciliation, if later applied to `traceCallChain`, would deepen `memory_trace` — but that is out of scope here; Phase 1 only adds the reconciling traversal inside `internal/flow`.

**Node ids for new edges:**
- HTTP entry node id: `"<METHOD> <fullpath>"`, e.g. `"POST /api/topup"` (the `http` edge `source_node`).
- HTTP edge `target_node`: the **bare handler name** (e.g. `WriteDocument`), matching the bare-name convention of `calls` targets so the builder's symbol reconciliation joins it uniformly. (See "Echo route extraction" for why the handler is usually a factory function, not an identifier.)

## Echo route extraction (the hard part)

**What to recognize** (Go tree-sitter call expressions). The receiver is **not** necessarily a local `e := echo.New()` — in this repo routes register on a struct field (`s.echo.Group(...)`, `routes.go:31`). So match by **method name on any receiver**, not by a known variable:
- `<recv>.GET/POST/PUT/DELETE/PATCH/HEAD/OPTIONS("/path", <handler> [, mw...])`
- Route groups: `g := <recv>.Group("/prefix" [, mw...])`, then `g.POST("/x", ...)` → full path `/prefix/x`. Track group variables and their accumulated prefixes, including **chained groups with empty prefixes** (`api := s.echo.Group("/api/v1")`, `data := api.Group("")`, `write := data.Group("")` → routes on `write` are still under `/api/v1`, `routes.go:31-67`). Group `Use(...)` args contribute group-scoped middleware.
- Middleware: `<recv>.Use(mw)`, group `Use(mw)`, and per-route trailing middleware args.

**Handlers are usually FACTORY CALLS, not identifiers (verified, `routes.go:68`).** Real registrations look like `write.POST("/write", handlers.WriteDocument(s.queries, s.db, …))` — the second arg is a *call expression* returning an `echo.HandlerFunc`, not a bare handler reference. The meaningful symbol is the factory function `WriteDocument`; calls inside the closure it returns are attributed by `go_extractor` to enclosing `…::WriteDocument`. So:
- HTTP edge target = the **bare callee name** of the handler argument: `handlers.WriteDocument(...)` → `WriteDocument`; method value `h.HandleGraph` → `HandleGraph`; bare `HandleTopup` → `HandleTopup`; inline `func(c echo.Context) error {…}` → no nameable target (rare; emit entry node only, log).
- This bare name joins the `calls` graph via the builder's symbol reconciliation (above) — no separate "canonical `path::Symbol`" resolution is needed or correct.

**Output edges** for `write.POST("/write", handlers.WriteDocument(...), csrfMW)` where the group chain resolves to `/api/v1`:
- `http`: `"POST /api/v1/write"` → `WriteDocument`, metadata `{method:"POST", path:"/api/v1/write"}`
- `middleware`: `csrfMW` → `WriteDocument`

**Handler-name extraction policy (never silently drop):**
- Factory call `pkg.Fn(...)` or `Fn(...)` → bare `Fn`. Method value `recv.Fn` → bare `Fn`. Bare identifier `Fn` → `Fn`. Inline closure → entry-only, logged.
- Always emit the `http` edge with the entry node even when no handler name is extractable, and **log at WARN/DEBUG with file+line**. Never panic.
- Bare names that the builder later cannot reconcile produce a flow that stops at the handler node — degraded, not broken.
- Explicit table-driven tests: factory call (qualified + unqualified), method value, bare identifier, inline closure, unresolved name.

**Cross-file constraint (accepted for Phase 1).** The `graph.Extractor` contract is per-file (`file → []Edge`), so a group variable defined in file A and used in file B cannot have its prefix resolved. **Verified acceptable here:** this repo registers all routes centrally in `internal/server/routes.go`, so group chains are single-file. Phase 1 documents this as a known limitation; repos that split route registration across files get partial paths (logged). Generalizing to cross-file group resolution is deferred.

## Flow builder

`FlowBuilder(edges []Edge, entry string, maxDepth int) Flow`:
- Index edges two ways: by exact `source_node`, AND by **symbol part** of `source_node` (`split_part(source_node,'::',2)`) to enable bare-name reconciliation (see "Node id convention").
- From `entry`, BFS following `http` → then `middleware` (annotated as guards, not depth-consuming) → then `calls`. **Traversal step:** at a node, take its `calls` edges to bare target names; for each bare target, (a) record it as a flow node, and (b) continue traversal from the source nodes whose symbol part equals that bare name (the reconciliation join). A bare target that reconciles to nothing is a terminal `external` leaf.
- **Ambiguity:** a bare name may reconcile to multiple files. Include each distinct candidate, de-duplicate by node, and cap fan-out per node (config-bounded) to avoid blow-up; mark ambiguous joins in flow metadata.
- `maxDepth` reuses the trace convention (default e.g. 10, max 10). Visited-set (keyed on resolved node) prevents cycles (recursion, mutual calls).
- **Node role classification** (heuristic, for styling + summary; never load-bearing): `entry` (the route), `middleware`, `handler` (direct `http` target), `service`/`repo` (by package/path/name heuristic, e.g. `*service*`, `*repo*`/`*repository*`, `*store*`), `external` (leaf with no outgoing edges in-workspace). Heuristics are isolated in one function so they're easy to tune.
- Output `Flow{ Entry, Method, Path, Nodes []FlowNode, Edges []FlowEdge }` — plain data.

## Mermaid rendering

`MermaidFlowchart(Flow) string` → `graph TD`:
- One node per `FlowNode` with a stable sanitized id and a label `name<br/>(role)`.
- One arrow per `FlowEdge`; middleware edges drawn with a dotted/guard style.
- Optional `classDef` per role for color. Deterministic ordering (sort nodes/edges) so golden files are stable.

## Flow materialization (hybrid storage)

- **Hook (verified):** the watcher already exposes a post-process notify callback — `summarizeNotify`, fired at `watcher.go:654` after a debounced processing pass via `WithSummarizeNotify`. Phase 1 adds a parallel `WithFlowNotify`/flow callback (or reuses the same post-process point) that triggers `Materialize` **per workspace**, not per file.
- For each `http` entry node: build the flow, render a **text** summary (entry line, ordered chain `A → B → C`, externals list, node roles) — *not* the Mermaid (that's on-demand). Write as a document via the normal `writeChunks` + embed-queue pipeline, title `"<METHOD> <path> flow"`, tags `["flow"]`.
- **Search isolation (verified mechanism).** Documents carry a `collection` column (`collections.sql`). Flow docs are written to a dedicated collection **`"flows"`** so they neither skew nor silently flood ordinary `memory_query`/`memory_search` results. Default search behavior is unchanged for existing collections; flow docs are discoverable but isolatable (and a future search option can include/exclude the `flows` collection explicitly). This addresses the "flow docs pollute search" risk directly rather than treating pollution as a feature.
- **Concurrency / single-flight.** `Materialize` for a given workspace MUST be single-flighted: overlapping settle ticks (rapid edits) must not race on flow-doc upsert/delete. Use a per-workspace mutex or in-progress guard; a newer trigger arriving mid-run schedules exactly one re-run after completion (coalesced).
- **Lifecycle / staleness:** flow docs are identified by a deterministic key (workspace + entry, within the `flows` collection). On each pass: upsert docs for current entries; **delete** flow docs whose entry no longer exists in the graph (route removed/renamed). Per-file edge re-extraction is already handled by `extractAndUpsertEdges` (delete-by-file then upsert); materialization reflects the latest edge set.

## Feature gating

The Echo extractor and flow materialization change indexing behavior for every workspace, so they are **gated behind config**, mirroring the existing `CodeSummarizationConfig{Enabled}` pattern (`config.go:469`). Add `FlowConfig{ Enabled bool; MaxDepth int; MaxFanout int }` under the main config (koanf/env, hot-reloadable like the rest). When `Enabled=false` (proposed Phase-1 default until validated): the Echo extractor is not registered, no `http`/`middleware` edges are produced, and materialization is skipped — the feature is fully inert. `memory_flow` returns a clear "flow indexing disabled" message when off.

## Surface

### MCP tool `memory_flow`
```
memory_flow({
  workspace,                 // workspace_hash
  entry:    "POST /api/topup",
  max_depth?: 10,
  format?:  "mermaid" | "json"   // default "mermaid"
})
→ { entry, method, path, chain: [...nodes], mermaid: "...", externals: [...] }
```
Discovery: the materialized flow docs already appear in `memory_query`, so an agent finds the entry string first, then calls `memory_flow`.

### REST `POST /api/v1/graph/flow`
Same request/response shape; backs the MCP tool (handlers delegate to the same `internal/flow` core).

## Testing strategy

- **EchoRouteExtractor:** table-driven over real Echo snippets — plain routes, nested groups, `e.Use`/group `Use`/per-route middleware, qualified vs bare vs method-value handlers, and the **unresolved-handler** case (asserts edge emitted to best-effort name + no panic).
- **FlowBuilder:** pure unit tests — linear chain, branching, cycle, depth cap, role classification, missing-downstream (unresolved handler) degrade path.
- **MermaidFlowchart:** golden-file tests; assert deterministic output.
- **Materializer + API (integration, `testutil.SetupTestDB`):** index a tiny Echo fixture repo → assert `http`/`middleware` rows in `graph_edges` → assert a flow document is searchable via query → `POST /api/v1/graph/flow` returns expected chain + Mermaid → remove a route, re-index, assert its flow doc is deleted.

## Why cross-service is excluded from Phase 1

A flow like *FE → BE → queue → old-backend → tradebot → socket → Steam* crosses **process boundaries** (HTTP between services, message queues, sockets, external APIs). Static call-graph analysis cannot link a `queue.Push("trade.created")` in one repo to a `queue.Consume("trade.created")` in another — they share only a **runtime string + broker config**, not a code edge. Honest cross-service tracing requires either (a) **Phase 2** heuristic stitching of typed integration edges by matching topic/event/URL strings across indexed workspaces, or (b) **Phase 3** runtime distributed tracing (OpenTelemetry). Phase 1 deliberately stops at the in-process boundary; an external/queue/socket call surfaces as a terminal `external` leaf node, which is accurate about what static analysis knows.

## Migration & rollback

- Forward: add migration `00024`; deploy; with `FlowConfig.Enabled=true`, existing graphs gain `http`/`middleware` edges on next index and flow docs appear after the next materialization pass. With the feature disabled, nothing changes.
- Rollback: the migration Down restores the prior CHECK and (per migration-12 precedent) requires operators to `DELETE FROM graph_edges WHERE edge_type IN ('http','middleware')` first. Flow docs can be deleted by tag.
- No change to existing edge types, tools, or search behavior; the feature is purely additive.
