# Tasks: Execution Flow Visualization — Phase 2 (Semantic Flow)

Items marked [x] are already implemented and in master (PR #423).
Each numbered group must build (`CGO_ENABLED=0 go build ./...`) and pass `go test -race -short ./...` before moving on.

---

## 0. Sequence Diagrams (already done — verify only)

- [x] 0.1 `RenderSequenceDiagram(Flow) string` implemented in `internal/flow/sequence.go`
- [x] 0.2 Golden-file tests in `internal/flow/sequence_test.go`
- [x] 0.3 `format=sequence` wired in `memory_flow` MCP tool (`internal/mcp/tools.go`)
- [x] 0.4 `format=sequence` wired in `POST /api/v1/graph/flow` handler (`internal/server/handlers/flow.go`)
- [ ] 0.5 Extend `RenderSequenceDiagram` with `alt`/`opt` blocks for conditional edges (depends on task 4.1)

---

## 1. Go Integration Edge Extraction (already done — verify gaps)

- [x] 1.1 `IntegrationExtractor` implemented in `internal/graph/integration_extractor.go`
- [x] 1.2 Detects: `http.NewRequest/WithContext`, `http.Get/Post/Put/Delete/Do`, `<recv>.Publish/Send/Emit/Enqueue/Dispatch/Broadcast/Notify/Produce/Push`
- [x] 1.3 Emits `EdgeIntegration` with `{kind, method, url/topic, receiver}` metadata
- [x] 1.4 Registered in `cmd/nano-brain/main.go` under `FlowConfig.Enabled`
- [x] 1.5 `RoleIntegration` in `FlowBuilder`; integration edges treated as terminal leaves
- [ ] 1.6 Add consumer detection to `integration_extractor.go`: `<recv>.Subscribe(topic, handler)`, `<recv>.Consume(queue, ...)`, `<recv>.Listen(topic, handler)`, `<recv>.On(event, handler)` → emit `EdgeHTTP` with `source_node = "CONSUME <topic>"` and `kind = queue_consumer` metadata
- [ ] 1.7 Unit tests for consumer detection patterns (Subscribe, Consume, Listen, On)

---

## 2. JS/TS Integration Edge Extraction

- [ ] 2.1 Create `internal/graph/js_integration_extractor.go` implementing `graph.Extractor` for `.js`, `.ts`, `.jsx`, `.tsx`
- [ ] 2.2 Detect `fetch(url)` → `HTTP <url>`; handle string literal and variable placeholder
- [ ] 2.3 Detect `axios.get/post/put/delete/patch(url)` → `<METHOD> <url>`
- [ ] 2.4 Detect `axios({method, url})` object-form calls
- [ ] 2.5 Detect `emitter.emit("topic", ...)` → `emit:<topic>`
- [ ] 2.6 Detect `channel.publish(exchange, routingKey, ...)` → `publish:<routingKey>` (amqplib)
- [ ] 2.7 Detect `redis.publish("channel", ...)` → `publish:<channel>`
- [ ] 2.8 Enclosing function detection: walk `function_declaration`, `arrow_function`, `method_definition` to map call-site to containing function for `SourceNode`
- [ ] 2.9 Detect JS/TS consumer patterns: `emitter.on("event", handler)` → `ON event`, `channel.consume("queue", handler)` → `CONSUME queue`, `redis.subscribe("channel", handler)` → `CONSUME channel`
- [ ] 2.10 Register extractor in `cmd/nano-brain/main.go` under `FlowConfig.Enabled`
- [ ] 2.11 Table-driven tests: each pattern with literal URL/topic; variable placeholder (`<var:name>`); nil-child guards (no panic on malformed AST)

---

## 3. Python Integration Edge Extraction

- [ ] 3.1 Create `internal/graph/python_integration_extractor.go` implementing `graph.Extractor` for `.py`
- [ ] 3.2 Detect `requests.get/post/put/delete/patch(url)` → `<METHOD> <url>`
- [ ] 3.3 Detect `httpx.get/post/put/delete/patch(url)` (sync and async) → `<METHOD> <url>`
- [ ] 3.4 Detect `session.get/post/...(url)` (generic session-like receiver)
- [ ] 3.5 Detect `channel.basic_publish(routing_key=..., ...)` → `publish:<routing_key>` (pika/kombu)
- [ ] 3.6 Detect `redis.publish("channel", ...)` → `publish:<channel>`
- [ ] 3.7 Enclosing function detection: walk `function_definition` and `decorated_definition` nodes
- [ ] 3.8 Register extractor in `cmd/nano-brain/main.go` under `FlowConfig.Enabled`
- [ ] 3.9 Table-driven tests: each pattern with literal and variable args; nil-child guards

---

## 4. Conditional Branch Metadata

- [ ] 4.1 Extend `go_extractor.go` `ExtractEdges` to detect when a `call_expression` is inside an `if_statement`, `switch_statement`, or `select_statement` ancestor; set `e.Metadata["conditional"] = true`
- [ ] 4.2 Extend `extractAndUpsertEdges` in `internal/watcher/watcher.go` to persist `conditional` field from `e.Metadata` (already persists full metadata since Phase 1 task 2.1)
- [ ] 4.3 Add `Conditional bool` field to `FlowEdge` in `internal/flow/builder.go`
- [ ] 4.4 `BuildFlow`: copy `conditional` from graph edge `Metadata` when constructing `FlowEdge`
- [ ] 4.5 `RenderFlowchart` (`mermaid.go`): render conditional edges as `-.->` (dotted) instead of `-->`
- [ ] 4.6 `RenderSequenceDiagram` (`sequence.go`): wrap consecutive conditional messages from the same sender in `alt` block; isolated conditional messages in `opt` block
- [ ] 4.7 Unit tests: `go_extractor` sets `conditional=true` for call inside `if`, not for call outside; `FlowEdge.Conditional` propagated; mermaid golden file with dotted edge; sequence golden file with `alt`/`opt`

---

## 5. Non-Echo Go Entry Points

- [ ] 5.1 Create `internal/graph/nethttp_extractor.go` implementing `graph.Extractor` for `.go`
- [ ] 5.2 Detect `http.HandleFunc("/path", handler)` and `http.Handle("/path", handler)` on any receiver (DefaultServeMux or named mux)
- [ ] 5.3 Detect gorilla/mux pattern: `r.HandleFunc("/path", handler).Methods("GET","POST")` → one `EdgeHTTP` per method; without `.Methods(...)` emit `HTTP /path`
- [ ] 5.4 Extract handler name with same policy as `EchoRouteExtractor` (factory call → bare name, method value → field name, bare identifier → itself, closure → entry-only + log)
- [ ] 5.5 Extract shared route-registration helpers into `internal/graph/http_router_helpers.go` (handler name extraction, path cleaning) — used by Echo, net/http, and Gin extractors
- [ ] 5.6 Create `internal/graph/gin_extractor.go` implementing `graph.Extractor` for `.go`
- [ ] 5.7 Detect Gin verb registrations `r.GET/POST/PUT/DELETE/PATCH/HEAD/OPTIONS("/path", handler)` on any receiver
- [ ] 5.8 Detect Gin route groups `g := r.Group("/prefix")` with prefix accumulation and nested groups (same algorithm as `EchoRouteExtractor`)
- [ ] 5.9 Detect `r.Use(mw)` / `g.Use(mw)` → `EdgeMiddleware`
- [ ] 5.10 Register both extractors in `cmd/nano-brain/main.go` under `FlowConfig.Enabled`
- [ ] 5.11 Table-driven tests for net/http: plain HandleFunc, Handle, mux variable, gorilla Methods chaining, unresolvable handler (no panic)
- [ ] 5.12 Table-driven tests for Gin: all verbs, single group, nested group, Use middleware, factory handler, inline closure

---

## 6. Queue Consumer Entry Points — Materialization

- [ ] 6.1 `FlowMaterializer.Materialize` currently queries only `http` entry nodes; extend to also query for entry nodes matching `CONSUME %` and `ON %` prefix (from consumer detection in tasks 1.6 and 2.9)
- [ ] 6.2 Flow documents for consumer flows tagged `["flow", "consumer"]` (in addition to `["flow"]`) for filter-ability
- [ ] 6.3 Integration test: index a fixture with a `Subscribe("trade.created", handler)` call → verify `CONSUME trade.created` flow document is searchable and contains the consumer handler chain

---

## 7. LLM Flow Summaries

- [ ] 7.1 Add `SummaryEnabled bool` to `FlowConfig` in `internal/config/config.go` (koanf/env, default false); validate in existing validation block
- [ ] 7.2 Define `FlowSummarizer` interface in `internal/flow/` (analogous to `Summarizer` in `internal/summarize/`): `Summarize(ctx, entry, chain []string, integrations []string) (string, error)`
- [ ] 7.3 Implement `LLMFlowSummarizer` wrapping the existing `internal/summarize` package with the flow-specific prompt (see design.md §2D)
- [ ] 7.4 Wire `LLMFlowSummarizer` into `FlowMaterializer` (constructor injection); only called when `SummaryEnabled`; 5s timeout with plain-text chain fallback; WARN log on failure
- [ ] 7.5 Update flow document content: when summary available, use as `content`; move plain-text chain to `metadata.chain`; when no summary, keep plain-text chain as `content` (no regression)
- [ ] 7.6 Unit test: mock summarizer called with correct args; fallback on error; document content matches summary when available, chain when not

---

## 8. Cross-Workspace Integration Stitching

- [ ] 8.1 Add `ListIntegrationEdgesByWorkspace` query to `internal/storage/queries/graph.sql` (select all `EdgeIntegration` edges for a workspace); run `sqlc generate`
- [ ] 8.2 Add `ListConsumerEntryNodesByWorkspace` query (select all entry nodes matching `CONSUME %` or `ON %` for a workspace)
- [ ] 8.3 Implement `Stitch(ctx, publishEdges []graph.Edge, targetWorkspaces []string, querier StitchQuerier) []FlowEdge` in `internal/flow/stitch.go`: for each publish edge, match `topic` in metadata against consumer entry nodes in target workspaces by string equality; return virtual cross-service `FlowEdge` with `Kind="cross_service"` (not persisted)
- [ ] 8.4 Add `StitchWorkspaces []string` to `POST /api/v1/graph/flow` request body
- [ ] 8.5 Add `stitch_workspaces` to `memory_flow` MCP tool input schema
- [ ] 8.6 Wire stitching in `handlers/flow.go` and `tools.go`: when `stitch_workspaces` non-empty, load integration publish edges from current flow, call `Stitch`, append virtual edges to the flow before rendering
- [ ] 8.7 Rendering: cross-service nodes in Mermaid get `classDef crossService fill:#f9f,stroke:#a0a` and a label showing the target workspace (first 8 chars of hash); in sequence diagram, a `Note over <participant>: cross-service (<workspace>)` note
- [ ] 8.8 Unit test: `Stitch` matches string-literal topics, skips `<var:...>` placeholders, returns empty on no match
- [ ] 8.9 Integration test: two test schemas (simulating two workspaces), one publishes `"trade.created"`, other has `CONSUME trade.created` entry; `Stitch` returns the link

---

## 9. Verification

- [ ] 9.1 Full build `CGO_ENABLED=0 go build ./...` green
- [ ] 9.2 `go test -race -short ./...` green
- [ ] 9.3 Integration suite `go test -race -tags=integration ./internal/flow/... ./internal/graph/... ./internal/server/handlers/...` green
- [ ] 9.4 Dogfood: enable on nano-brain itself; call `memory_flow` on `POST /api/v1/query` with `format=sequence`; verify sequence diagram renders in Mermaid live editor
- [ ] 9.5 Dogfood: call `memory_flow` on a known integration call (e.g. embed queue publish); verify `CONSUME` entry node appears in a matched consumer workspace if available

---

## Out of scope (Phase 3)
- OpenTelemetry / distributed runtime tracing
- Data-flow variable tracking across calls
- Dynamic topic string resolution (runtime value interpolation)
- Conditional branch detection for JS/TS and Python
