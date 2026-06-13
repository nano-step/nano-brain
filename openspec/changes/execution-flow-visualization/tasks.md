# Tasks: Execution Flow Visualization — Phase 1

Ordered for incremental, independently-verifiable delivery. Each numbered group should build (`CGO_ENABLED=0 go build ./...`) and pass `go test -race -short ./...` before moving on. Integration tasks use `-tags=integration`.

## 0. Config gate (do first — everything hangs off it)
- [x] 0.1 Add `FlowConfig{ Enabled bool; MaxDepth int; MaxFanout int }` to `internal/config/config.go` (koanf/env, hot-reloadable), mirroring `CodeSummarizationConfig`. Default `Enabled=false` for Phase 1 until validated.
- [x] 0.2 Validate config in the existing validation block (sane MaxDepth/MaxFanout bounds).

## 1. Data model & migration
- [x] 1.1 Add `EdgeHTTP = "http"` and `EdgeMiddleware = "middleware"` to `internal/graph/edge.go`.
- [x] 1.2 Add `Metadata map[string]any` field to `graph.Edge`.
- [x] 1.3 Write **`migrations/00024_flow_edge_types.sql`** (NOT 00013 — that exists; latest is 00023) extending the `graph_edges.edge_type` CHECK to include `'http'` and `'middleware'` (mirror migration 00012 Up/Down, including the `DELETE FROM graph_edges WHERE edge_type IN ('http','middleware')` note in Down).
- [x] 1.4 Apply migration locally (`go run ./cmd/nano-brain db:migrate`) and confirm the constraint accepts the new values.
- [x] 1.5 Stats/PageRank interaction — VERIFIED no change needed: `stats.go` uses the dynamic `CountGraphEdgesByType` (`stats.go:209`) which already counts any edge type, and the hardcoded `GraphStats :one` query has no non-test consumers; PageRank already scopes to `edge_type = 'calls'` (`pagerank.sql:4,22`), so `http`/`middleware` edges are auto-excluded.

## 2. Edge metadata persistence
- [x] 2.1 In `internal/watcher/watcher.go` `extractAndUpsertEdges`, build persisted metadata by starting from `e.Metadata` (if non-nil) and merging in `line` + `language`, instead of overwriting with `{line, language}`.
- [x] 2.2 Unit/integration test: an edge with extractor metadata round-trips through persistence with `method`/`path`/`line`/`language`; an edge without extractor metadata still stores exactly `{line, language}` (no regression for existing extractors).

## 3. Echo route extractor
- [x] 3.1 Create `internal/graph/echo_extractor.go` implementing `graph.Extractor` (`Supports(".go")`, `ExtractEdges`).
- [x] 3.2 Parse verb registrations `e.<VERB>("/path", handler [, mw...])` via Go tree-sitter; emit `http` edges with `{method, path}` metadata.
- [x] 3.3 Track route-group variables and accumulate prefixes (including nested groups) to compute full paths.
- [x] 3.4 Emit `middleware` edges for `e.Use`, group `Use`, and per-route trailing middleware args.
- [x] 3.5 Extract the **bare handler name** from: factory call `pkg.Fn(...)`/`Fn(...)` → `Fn`; method value `recv.Fn` → `Fn`; bare identifier `Fn` → `Fn`; inline closure → entry-only + log. Match routes/groups on **any receiver** (e.g. `s.echo`), not a known `echo.New()` var. Never drop, never panic.
- [x] 3.6a Change `registry.go` `ExtractEdges` to run **all** supporting extractors and concatenate edges (dedup identical edges), instead of returning after the first match. Preserve existing single-extractor behavior. Add a unit test for multi-extractor aggregation.
- [x] 3.6 Register the extractor at construction (`cmd/nano-brain/main.go:339` `graph.NewRegistry(graphExtractors...)`) — **only when `FlowConfig.Enabled`** (so the feature is inert when off).
- [x] 3.7 Table-driven tests: plain route, all verbs, single + nested + empty-prefix chained groups, three middleware forms, factory-call/method-value/bare/inline handlers, non-local receiver, unresolvable-name case.

## 4. Flow builder (`internal/flow`)
- [x] 4.1 Define `Flow`, `FlowNode`, `FlowEdge`, and node `Role` types (plain data).
- [x] 4.2 Implement `BuildFlow(edges, entry, maxDepth, maxFanout) Flow`: index edges by exact source AND by **symbol part** of source. BFS `http` → `middleware` (guards, non-depth-consuming) → `calls` with **bare-name reconciliation** (bare target → source nodes whose symbol part matches → continue). Visited-set on resolved node; depth cap; per-node fan-out cap.
- [x] 4.3 Implement isolated role-classification heuristics (entry/middleware/handler/service/repo/external).
- [x] 4.4 Unit tests: multi-hop via reconciliation, exact-match-would-dead-end case, ambiguous reconciliation (name in N files), external leaf (no reconciliation), branching, cycle, depth cap, fan-out cap, middleware guard.

## 5. Mermaid renderer (`internal/flow`)
- [x] 5.1 Implement `RenderFlowchart(Flow) string` → `graph TD` with sanitized stable ids, role labels, deterministic ordering, distinct middleware-edge style, optional `classDef` per role.
- [x] 5.2 Golden-file tests; assert byte-identical output across two runs and id sanitization.

## 6. Flow materializer (`internal/flow`)
- [x] 6.1 Implement `Materialize(ctx, workspace)`: load workspace edges, find `http` entries, build each flow, render a text summary (entry, ordered chain, externals, roles — no Mermaid), and upsert as a `flow`-tagged document **in the dedicated `flows` collection** via the existing chunk + embed pipeline.
- [x] 6.2 Implement deterministic doc keying per (workspace, entry) within the `flows` collection; delete flow docs whose entry no longer exists.
- [x] 6.3 Add `ListAllEdgesByWorkspace` (or equivalent) to `internal/storage/queries/graph.sql` (none exists today); consider pagination/bounds for large graphs; run `sqlc generate`.
- [x] 6.4 Add `WithFlowNotify`/flow callback mirroring `summarizeNotify` (`watcher.go:654`); wire it to trigger `Materialize` per workspace **only when `FlowConfig.Enabled`**. Implement **single-flight per workspace** (in-progress guard + coalesced re-run).
- [x] 6.5 Integration test: index Echo fixture → flow doc searchable + isolated in `flows` collection; remove a route → flow doc deleted; concurrent triggers do not duplicate/race.

## 7. API & MCP surface
- [x] 7.1 Create `internal/server/handlers/flow.go` with `POST /api/v1/graph/flow` (request `{workspace, entry, max_depth?, format?}`), delegating to `internal/flow` core; register the route.
- [x] 7.2 Add `memory_flow` to `internal/mcp/tools.go` with the same contract; handle unknown entry as empty result + clear message; when `FlowConfig.Enabled=false`, return a clear "flow indexing disabled" message (no error/fabrication).
- [x] 7.3 Integration test: `POST /api/v1/graph/flow` returns expected chain + Mermaid for the fixture; `memory_flow` returns the same shape; unknown entry returns not-found message.

## 8. Verification & docs
- [x] 8.1 Full build `CGO_ENABLED=0 go build ./...` and `go test -race -short ./...` green; integration suite `go test -race -tags=integration ./internal/flow/... ./internal/graph/... ./internal/server/handlers/...` green.
- [x] 8.2 Dogfood: run against the nano-brain repo itself (an Echo service); spot-check `memory_flow` on a real endpoint (e.g. `POST /api/v1/query`).
- [x] 8.3 Update the nano-brain MCP skill / tool docs to list `memory_flow`.
- [x] 8.4 `openspec validate execution-flow-visualization --strict` passes.

## Out of scope (do NOT implement here)
- Sequence diagrams; LLM/semantic summaries; conditional-branch metadata (Phase 2).
- Non-Echo / non-Go entry points (Phase 2+).
- Integration-point edges (publish/consume/emit/listen/external-HTTP) and cross-workspace stitching (Phase 2).
- Distributed/runtime tracing, data-flow analysis (Phase 3).
