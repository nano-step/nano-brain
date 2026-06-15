# Proposal: Execution Flow Visualization — Phase 2 (Semantic Flow)

## Context

Phase 1 shipped a structural flow visualizer: Echo HTTP routes → call chain → Mermaid flowchart + searchable flow documents. It is complete and in production (`FlowConfig.Enabled`).

During Phase 1 development, two Phase 2 components were implemented ahead of schedule and merged to master in PR #423:
- **Sequence diagram renderer** (`internal/flow/sequence.go`) — `RenderSequenceDiagram(Flow) string`
- **Go integration edge extractor** (`internal/graph/integration_extractor.go`) — detects outbound HTTP calls and queue publish patterns in Go files; emits `EdgeIntegration` edges

Both are wired and functional. Phase 2 acknowledges these as done, identifies their gaps, and completes the remaining semantic-flow capabilities.

## Why

Phase 1 answered: *"what functions does this endpoint call?"*
Phase 2 answers: *"what does this endpoint actually DO — what systems does it touch, what does it talk to, what order does it do things in?"*

Real-world agent queries that Phase 1 cannot answer today:
- "What does the withdrawal flow do after sell?" — needs integration edges showing queue publish or HTTP to payment service
- "Which endpoints talk to the payment service?" — needs cross-cutting integration search
- "Show me the sequence of calls for POST /api/v1/query" — needs sequence diagram in a useful shape
- "What entry points consume from the trade queue?" — needs consumer detection as flow entry nodes

## What Changes

### Already implemented (carried from Phase 1 development)
- **Sequence diagrams** — `RenderSequenceDiagram(Flow)` renders Mermaid `sequenceDiagram` with participants, middleware guards as notes, integration edges as `-->>` async arrows. Exposed via `format=sequence` in `memory_flow` and `POST /api/v1/graph/flow`.
- **Go integration edge extraction** — `IntegrationExtractor` detects in Go files: `http.NewRequest/WithContext`, `http.Get/Post/Put/Delete/Do`, and `<recv>.Publish/Send/Emit/Enqueue/Dispatch/Broadcast/Notify/Produce/Push`. Emits `EdgeIntegration` with `{kind, method, url/topic}` metadata. Registered in main.go when `FlowConfig.Enabled`.
- **Integration nodes in FlowBuilder** — `RoleIntegration` role; integration edges are terminal leaf nodes never traversed further.

### Remaining Phase 2 work

**2A. Integration edge extraction — non-Go languages**
- JS/TS: `fetch(url)`, `axios.get/post/put/delete`, `EventEmitter.emit(topic)`, `amqplib.channel.publish(...)`, `redis.publish(...)`
- Python: `requests.get/post/put/delete`, `httpx.get/post`, `pika.channel.basic_publish(...)`, `redis.publish(...)`

**2B. Non-Echo entry points (Go)**
- `net/http` handlers: `http.HandleFunc("/path", handler)`, `http.Handle("/path", handler)`, `mux.HandleFunc(...)`
- Gin: `r.GET/POST/PUT/DELETE("/path", handler)`, route groups `g := r.Group("/prefix")`

**2C. Queue consumer entry points**
- Go: `<recv>.Subscribe(topic, handler)`, `<recv>.Consume(queue, ...)`, `<recv>.Listen(topic, handler)` — emit as flow entry nodes with `RoleEntry` and synthetic entry id `CONSUME <topic>`
- Enables answering "what processes trade.completed events?"

**2D. LLM flow summaries**
- After building a flow, generate a 2–4 sentence natural language summary: what the endpoint does, which systems it calls, what it returns
- Stored as part of the flow document (searchable); not on-demand (avoids latency at query time)
- Gated behind existing `CodeSummarizationConfig.Enabled` or a new `FlowConfig.SummaryEnabled`

**2E. Conditional branch metadata**
- Detect if/else and switch branches in call chains (advisory, not structural)
- Mark flow edges with `conditional: true` when the call is inside a conditional block
- Mermaid flowchart renders conditional edges with a dashed style
- Sequence diagram adds an `alt`/`opt` block around conditional messages

**2F. Cross-workspace integration stitching**
- When workspace A publishes to topic `"trade.created"` and workspace B subscribes to `"trade.created"`, link them in `memory_flow` as cross-service edges
- Requires: both workspaces indexed, topic string matching across `EdgeIntegration` publish nodes and consumer entry nodes
- Out of scope: dynamic topic strings (variable placeholders)

## Capabilities

### New Capabilities
- `js-ts-integration-edges`: detect outbound HTTP and event publish in JS/TS files
- `python-integration-edges`: detect outbound HTTP and event publish in Python files
- `go-nethttp-entry-points`: detect net/http and Gin route registrations as flow entries
- `queue-consumer-entry-points`: detect queue/event consumer registrations as flow entry nodes
- `flow-llm-summaries`: natural language summary of what a flow does, stored in flow doc
- `conditional-branch-metadata`: mark conditional call edges; render alt/opt in sequence diagrams
- `cross-workspace-stitching`: link publish→subscribe pairs across indexed workspaces

### Modified Capabilities
- `sequence-diagrams`: already implemented; extended with conditional `alt`/`opt` blocks (2E)
- `integration-edge-extraction`: Go done; extended to JS/TS and Python (2A)
- `flow-entry-detection`: Echo done; extended to net/http, Gin, queue consumers (2B, 2C)

## Impact

**Code affected:**
- `internal/graph/` — new extractors: `nethttp_extractor.go`, `gin_extractor.go`, `js_integration_extractor.go`, `python_integration_extractor.go`; add consumer detection to `integration_extractor.go`
- `internal/flow/builder.go` — conditional branch metadata, cross-workspace stitching hook
- `internal/flow/mermaid.go` — dashed conditional edge style
- `internal/flow/sequence.go` — `alt`/`opt` blocks for conditional messages
- `internal/flow/materializer.go` — LLM summary generation and storage
- `internal/mcp/tools.go` — cross-workspace stitching in `memory_flow`
- `internal/server/handlers/flow.go` — stitching in REST handler

**Dependencies:** No new external dependencies. LLM summaries reuse existing `summarize` package.

**Performance:** Integration extraction adds O(nodes) per file. Conditional branch detection adds one tree-sitter pass. LLM summaries are async (materializer). Cross-workspace stitching is O(publish_edges × subscribe_edges) at query time — bounded by workspace size.

**API changes:** None breaking. Flow JSON response gains optional `summary` (string, additive — does NOT replace or modify existing `content` field) and `conditional_edges` ([]string of edge ids) fields.

## Non-Goals (deferred to Phase 3)
- OpenTelemetry runtime tracing
- Data-flow analysis (variable tracking across calls)
- Cross-service distributed tracing
- Dynamic topic string resolution
