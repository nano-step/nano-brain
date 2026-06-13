## Why

nano-brain today is a **symbol search engine**: it finds functions, types, and 1-hop call/import edges, but it cannot answer "how does *topup* work end-to-end?" A developer (or an AI agent) must manually stitch together router → handler → service → repo → external call by repeated `memory_trace` and grep. Research stored in nano-brain ("Research: Execution Flow Visualization — Impact Analysis", 2026-06-13) estimates that materializing **execution flows** as a first-class, searchable, renderable artifact would lift search recall@10 from ~0.62 to ~0.85–0.90 and materially cut debugging and onboarding time.

This change implements **Phase 1 (Structural Flow)** of the roadmap: derive in-process execution flows for HTTP endpoints **statically** from Go/Echo code and expose them both as **searchable summaries** (for agents) and **on-demand Mermaid flowcharts** (for humans). It is deliberately minimal — single repo, in-process, synchronous, flowchart only.

**Explicitly out of scope (later phases, see design.md):** sequence diagrams, LLM/semantic summaries, conditional-branch metadata (Phase 2); cross-service / queue / socket / external stitching and distributed tracing (Phase 2 stitching + Phase 3 runtime). Cross-process boundaries (HTTP-to-service, message queues, sockets, external APIs like Steam) are invisible to static analysis and are **not** traced in Phase 1.

## What Changes

- **Detect HTTP entry points** — a new Go/Echo extractor parses `e.GET/POST/PUT/DELETE/PATCH(...)`, route groups (`e.Group("/prefix")`), and middleware (`e.Use(...)`, per-route middleware args) into typed graph edges.
- **Add two graph edge types** — `http` (route → handler) and `middleware` (middleware → handler), stored in the existing `graph_edges` table alongside `contains`/`imports`/`calls`/`references`, with method/path carried in `metadata`.
- **Carry per-edge metadata** — the `graph.Edge` struct and the watcher persistence path currently flatten metadata to `{line, language}`; this change preserves extractor-supplied metadata so HTTP edges can carry `{method, path}`.
- **Build flows** — a pure flow builder walks the graph from an HTTP entry node over `http → middleware → calls` edges (depth-capped, cycle-safe) into a `Flow` tree with role-classified nodes. Because `calls` edges target **bare callee names** while sources are `<file>::<func>` (so the graph is not transitively traversable by exact id, and `memory_trace` itself dead-ends after one hop), the builder reconciles bare target names to defining source nodes by symbol-part match to reach service/repo depth.
- **Render flowcharts** — a pure renderer turns a `Flow` into a Mermaid `graph TD` string.
- **Materialize searchable flow summaries** — a workspace-level, single-flighted pass (hooked to the existing post-process notify at `watcher.go:654`) writes one searchable document per detected endpoint into a dedicated **`flows` collection** (so it does not flood ordinary search) through the normal chunk + embed pipeline. Mermaid is rendered on demand, not stored.
- **Gate behind config** — a new `FlowConfig{Enabled,…}` (mirroring `CodeSummarizationConfig`) makes the whole feature inert when disabled; proposed Phase-1 default is off until validated.
- **Expose `memory_flow`** — a new MCP tool plus backing `POST /api/v1/graph/flow` endpoint returning the flow chain + Mermaid for a given entry point.

## Capabilities

### New Capabilities
- `flow-entry-extraction`: Statically extract HTTP entry points and middleware from Go/Echo source into `http`/`middleware` graph edges.
- `flow-builder`: Build a depth-capped, cycle-safe in-process flow tree from an entry node by traversing graph edges.
- `flow-rendering`: Render a flow tree as a Mermaid `graph TD` flowchart.
- `flow-search`: Materialize one searchable flow-summary document per detected endpoint (hybrid storage), kept fresh on code change.
- `flow-api`: Expose flows via the `memory_flow` MCP tool and `POST /api/v1/graph/flow`.

### Modified Capabilities
- `graph-stats`: `GraphStats` is extended to count the new `http`/`middleware` edge types (otherwise they are silently omitted). Existing `contains`/`imports`/`calls` extraction, `memory_trace`, `memory_graph`, and `memory_impact` behavior is unchanged.

## Impact

- **Code affected**:
  - `internal/graph/edge.go` — add `EdgeHTTP`, `EdgeMiddleware`; add `Metadata map[string]any` to `Edge`.
  - `internal/graph/echo_extractor.go` — new Echo route/middleware extractor (implements `graph.Extractor`).
  - `internal/graph/registry.go` — register the Echo extractor.
  - `internal/watcher/watcher.go` — merge `Edge.Metadata` into persisted JSONB in `extractAndUpsertEdges`; trigger flow materialization in the post-index pass.
  - `internal/flow/` — new package: `builder.go` (traversal), `mermaid.go` (renderer), `materializer.go` (workspace pass + flow-doc lifecycle).
  - `internal/server/handlers/flow.go` — new `POST /api/v1/graph/flow` handler.
  - `internal/mcp/tools.go` — register `memory_flow`.
  - `migrations/00024_flow_edge_types.sql` — extend the `edge_type` CHECK constraint to include `'http'`, `'middleware'`. (Numbered 00024: latest existing migration is 00023.)
  - `internal/storage/queries/graph.sql` — add a "load all edges for a workspace" query (none exists today) for the builder/materializer; extend `GraphStats` to count `http`/`middleware` edges.
  - `internal/config/config.go` — add `FlowConfig{Enabled, MaxDepth, MaxFanout}` (mirrors `CodeSummarizationConfig`); feature is inert when disabled.
  - `internal/server/handlers/stats.go` — surface the new edge-type counts.

- **Dependencies**: None new. Reuses existing tree-sitter (Go), `graph_edges` table, document/chunk/embed pipeline, and `testutil.SetupTestDB`.

- **Performance**: Edge extraction adds one more extractor per Go file (negligible). Flow materialization is a workspace-level pass bounded by endpoint count; runs debounced after indexing, off the request path. Mermaid is generated on demand.

- **API changes**: New `POST /api/v1/graph/flow` endpoint and `memory_flow` MCP tool (additive). Flow-summary documents become discoverable through existing `memory_query`/`memory_search`.

- **Risk**: The main risk is **call-graph transitivity** — `calls` edges target bare callee names while sources are `<file>::<func>`, so multi-hop flows require symbol-part reconciliation (~56% of targets reconcile; the rest are external leaves). The builder must implement this join (the existing `memory_trace` does not, and is shallow as a result). Secondary risk is **handler-name extraction** from factory calls / method values / closures; it is best-effort, **logs rather than silently dropping**, and carries explicit test coverage.
