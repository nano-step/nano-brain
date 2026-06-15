# Design: Execution Flow Visualization — Phase 2 (Semantic Flow)

## Current State (inherited from Phase 1 + early Phase 2 code)

```
graph_edges table
  ├── EdgeHTTP      "POST /api/v1/write" → "WriteDocument"
  ├── EdgeMiddleware "csrfMW" → "WriteDocument"
  ├── EdgeCalls     "file::WriteDocument" → "upsertChunks"
  └── EdgeIntegration "file::WriteDocument" → "POST api.example.com/v1/store"  ← already extracted for Go
```

`BuildFlow` traverses: entry → http → middleware (guards) → calls (with symbol reconciliation) → integration (terminal leaves).
`RenderFlowchart` → Mermaid `graph TD` (done).
`RenderSequenceDiagram` → Mermaid `sequenceDiagram` (done, needs conditional blocks).

## 2A — Integration Edge Extraction: JS/TS and Python

### JS/TS extractor (`internal/graph/js_integration_extractor.go`)

Reuses the existing `javascript_extractor.go` tree-sitter infrastructure (grammars.JavascriptLanguage, grammars.TypescriptLanguage).

**Patterns to detect:**

| Pattern | Target node id | Metadata |
|---------|---------------|----------|
| `fetch(url, opts)` | `HTTP <url>` | `{kind: "http_call", url}` |
| `axios.get/post/put/delete/patch(url)` | `<METHOD> <url>` | `{kind: "http_call", method, url}` |
| `axios({method, url})` | `<METHOD> <url>` | `{kind: "http_call"}` |
| `client.get/post/...(url)` (generic) | `<METHOD> <url>` | `{kind: "http_call"}` |
| `emitter.emit(topic, ...)` | `emit:<topic>` | `{kind: "queue_publish", topic}` |
| `channel.publish(exchange, routingKey, ...)` | `publish:<routingKey>` | `{kind: "queue_publish", topic: routingKey}` |
| `redis.publish(channel, ...)` | `publish:<channel>` | `{kind: "queue_publish", topic: channel}` |

**Enclosing function detection**: same pattern as Go extractor — walk `function_declaration` / `arrow_function` / `method_definition` nodes to build byte-range → function-name map, then map each call site to its enclosing function for `SourceNode`.

**Supports**: `.js`, `.ts`, `.jsx`, `.tsx`

### Python extractor (`internal/graph/python_integration_extractor.go`)

Uses grammars.PythonLanguage().

**Patterns:**

| Pattern | Target node id | Metadata |
|---------|---------------|----------|
| `requests.get/post/put/delete/patch(url)` | `<METHOD> <url>` | `{kind: "http_call"}` |
| `httpx.get/post/...(url)` (sync + async) | `<METHOD> <url>` | `{kind: "http_call"}` |
| `session.get/post/...(url)` | `<METHOD> <url>` | `{kind: "http_call"}` |
| `channel.basic_publish(routing_key=..., ...)` | `publish:<routing_key>` | `{kind: "queue_publish"}` |
| `redis.publish(channel, ...)` | `publish:<channel>` | `{kind: "queue_publish"}` |

**Enclosing function detection**: walk `function_definition` and `decorated_definition` nodes.

**Supports**: `.py`

### Registration

Both extractors registered in `cmd/nano-brain/main.go` alongside `IntegrationExtractor`, under the same `FlowConfig.Enabled` gate.

### Testing

Table-driven tests for each extractor:
- `fetch(url)` with literal URL → correct target node
- `fetch(variable)` → `HTTP <var:variable>` (placeholder, not dropped)
- `axios.post("https://api.example.com/pay", data)` → `POST api.example.com/pay`
- `emitter.emit("trade.created", payload)` → `emit:trade.created`
- Each pattern verified to not panic on malformed ASTs (nil child guards)

---

## 2B — Non-Echo Entry Points (Go)

### `net/http` extractor (`internal/graph/nethttp_extractor.go`)

**Patterns:**

| Pattern | Entry node id | Handler |
|---------|--------------|---------|
| `http.HandleFunc("/path", handler)` | `HTTP /path` | bare handler name |
| `http.Handle("/path", handlerObj)` | `HTTP /path` | type name |
| `mux.HandleFunc("/path", handler)` | `HTTP /path` | bare handler name |
| `mux.Handle("/path", handler)` | `HTTP /path` | bare handler name |
| `r.HandleFunc("/path", handler).Methods("GET","POST")` (gorilla) | `GET /path`, `POST /path` | bare handler name |

**Method detection**: when `.Methods(...)` is chained, emit one `EdgeHTTP` per method. Without Methods, emit `HTTP /path` (method unknown).

**Receiver matching**: same approach as `EchoRouteExtractor` — match on method name (`HandleFunc`, `Handle`), any receiver. Do not require a known `http.DefaultServeMux` var.

**Supports**: `.go`

### Gin extractor (`internal/graph/gin_extractor.go`)

**Patterns:**

| Pattern | Entry node id |
|---------|--------------|
| `r.GET("/path", handler)` | `GET /path` |
| `r.POST("/path", handler)` | `POST /path` |
| `g := r.Group("/prefix")` then `g.GET("/x", h)` | `GET /prefix/x` |
| `r.Use(mw)` / `g.Use(mw)` | middleware edges |

Nearly identical to `EchoRouteExtractor` — same tree-sitter pattern matching, same group-prefix accumulation, same handler-name extraction policy. Consider extracting shared logic into `internal/graph/http_router_helpers.go`.

**Supports**: `.go`

### Registration

Both registered under `FlowConfig.Enabled` in `main.go`. The registry already supports multiple extractors per extension (changed in Phase 1, task 3.6a).

---

## 2C — Queue Consumer Entry Points

### Approach

Consumer registrations create **flow entry nodes** analogous to HTTP routes. A `CONSUME <topic>` entry node is the root of its own flow.

**Detection patterns (Go, in `integration_extractor.go` extension):**

| Pattern | Entry node id | Handler |
|---------|--------------|---------|
| `<recv>.Subscribe("topic", handler)` | `CONSUME topic` | bare handler name |
| `<recv>.Consume("queue", ...)` | `CONSUME queue` | (handler in callback arg) |
| `<recv>.Listen("topic", handler)` | `CONSUME topic` | bare handler name |
| `<recv>.On("event", handler)` | `ON event` | bare handler name |

**Edge emitted**: `EdgeHTTP` with `source_node = "CONSUME <topic>"`, `target_node = <handler>`, metadata `{kind: "queue_consumer", topic}`. Using `EdgeHTTP` means `FlowBuilder` picks it up automatically as an entry — no builder changes needed. (Alternative: introduce `EdgeConsumer` and update builder; prefer reuse for Phase 2.)

**Node id convention**: `CONSUME <topic>` (uppercase, matches `CONSUME` prefix) so `memory_flow` can query by entry.

### JS/TS consumer patterns (in `js_integration_extractor.go`):

| Pattern | Entry node id |
|---------|--------------|
| `emitter.on("event", handler)` | `ON event` |
| `channel.consume("queue", handler)` | `CONSUME queue` |
| `redis.subscribe("channel", handler)` | `CONSUME channel` |

### Materialization

`FlowMaterializer.Materialize` currently queries for `http` entry nodes. Extend to also query for `CONSUME` and `ON` entry nodes and materialize flows for them. Flow documents get tag `["flow", "consumer"]`.

---

## 2D — LLM Flow Summaries

### When

Generated during `FlowMaterializer.Materialize`, after the flow is built but before the document is written. Async — does not block the HTTP response.

### Gating

New config field `FlowConfig.SummaryEnabled bool` (default: false). Mirrors `CodeSummarizationConfig.Enabled`. Only runs when both `FlowConfig.Enabled` and `FlowConfig.SummaryEnabled` are true.

### Prompt

```
You are summarizing an API endpoint's execution flow for a search index.
Given the flow chain below, write 2-4 sentences describing:
1. What the endpoint does (business purpose)
2. Which downstream services or systems it calls
3. Any notable patterns (async, external HTTP, queue publish)

Flow: {entry}
Chain: {A → B → C → D}
Integration points: {list of integration targets}
External leaves: {list}

Summary:
```

Reuse `internal/summarize/` package — the existing `Summarizer` interface and `NewLLMSummarizer`.

### Storage

Summary stored as the `content` field of the flow document (replacing the current plain-text chain). The plain-text chain moves to a `metadata` field on the document for programmatic access. This makes `memory_query` return human-readable summaries instead of raw chain strings.

### Fallback

If LLM call fails or times out (5s), store the plain-text chain as before. Log at WARN. Never block materialization.

---

## 2E — Conditional Branch Metadata

### Detection

During flow builder traversal, an edge is marked `conditional=true` when its call site byte offset falls inside an `if_statement`, `switch_statement`, or `select_statement` node in the parsed AST.

**Implementation**: `BuildFlow` receives the raw `[]graph.Edge` which already carry `Line` (call site line number). The extractor must additionally carry `Conditional bool` in edge `Metadata`. The Go extractor (`go_extractor.go`) is extended to detect this during its tree-sitter walk: for each `call_expression`, check if any ancestor is an `if_statement` or `switch_statement`.

**Edge metadata field**: `e.Metadata["conditional"] = true`.

**FlowEdge extension**:
```go
type FlowEdge struct {
    From        string
    To          string
    Kind        string
    Line        int
    Conditional bool  // NEW: call is inside an if/switch/select
}
```

`BuildFlow` copies `Conditional` from the graph edge metadata when constructing `FlowEdge`.

### Rendering

**Mermaid flowchart** (`mermaid.go`): conditional edges rendered with `-.->` (dotted arrow) instead of `-->`.

**Sequence diagram** (`sequence.go`): conditional message groups wrapped in `alt` / `opt` blocks:
```
alt conditional path
    A->>B: callName
end
```
Group consecutive conditional messages from the same sender into one `alt` block. Non-consecutive conditional messages each get their own `opt` block. "Consecutive" means adjacent in the topologically-sorted node list (not path-dependent) — two conditional nodes are grouped if no unconditional node separates them in the sort order.

### Scope

Only Go in Phase 2 (Go extractor already has AST). JS/TS and Python conditional detection deferred to Phase 3.

---

## 2F — Cross-Workspace Integration Stitching

### Concept

When workspace A has `EdgeIntegration` with `kind=queue_publish, topic="trade.created"` and workspace B has an entry node `CONSUME trade.created`, nano-brain can link them: calling `memory_flow` on workspace A's publish node shows workspace B's consumer as a downstream participant.

### Data model

No new tables. Stitching is **query-time** only — it reads integration edges from both workspaces at query time and joins by topic string.

```
stitching input:
  workspaceA publish edges: [{target: "publish:trade.created", metadata: {topic: "trade.created"}}]
  workspaceB entry edges:   [{source: "CONSUME trade.created", target: "TradeConsumerHandler"}]

output edge (virtual, not persisted):
  {from: "publish:trade.created", to: "CONSUME trade.created", kind: "cross_service", workspace: workspaceB}
```

### Surface

`memory_flow` request gains optional `stitch_workspaces: [hash1, hash2, ...]` field. When present, the handler loads integration publish edges from the current flow and queries each listed workspace for matching consumer entry nodes.

`POST /api/v1/graph/flow` request gains the same field.

Cross-service nodes rendered in Mermaid with a distinct box style (`classDef crossService`). In sequence diagrams, cross-service participants are in a separate section with a divider note.

### Limitations (documented, not worked around)

- Dynamic topics (`<var:topic>`) cannot be stitched — matched only when the topic is a string literal
- Stitching is opt-in (caller must pass `stitch_workspaces`) — avoids unexpected latency
- No persistence of cross-service edges — always recomputed at query time

---

## Architecture Summary

```
INDEX TIME (per file):
  *.go  → EchoRouteExtractor      → http, middleware edges  (Phase 1 ✅)
  *.go  → IntegrationExtractor    → integration edges       (Phase 2A partial ✅)
           + consumer detection   → http/consumer edges     (Phase 2C)
  *.go  → NetHttpExtractor        → http edges              (Phase 2B)
  *.go  → GinExtractor            → http edges              (Phase 2B)
  *.go  → go_extractor            + conditional metadata    (Phase 2E)
  *.js/ts → JsIntegrationExtractor → integration edges     (Phase 2A)
  *.py  → PythonIntegrationExtractor → integration edges   (Phase 2A)

POST-INDEX (per workspace):
  FlowMaterializer
    ├── build flows for http + CONSUME entries
    ├── (optional) generate LLM summary          (Phase 2D)
    └── upsert flow documents in "flows" collection

QUERY TIME:
  memory_flow / POST /api/v1/graph/flow
    ├── BuildFlow (with conditional edges)        (Phase 2E)
    ├── optional: stitch cross-workspace         (Phase 2F)
    ├── format=mermaid → RenderFlowchart         (Phase 1 ✅)
    └── format=sequence → RenderSequenceDiagram  (Phase 2 partial ✅)
```

## Testing Strategy

| Component | Test type | Notes |
|-----------|-----------|-------|
| JS/TS integration extractor | Table-driven unit | fetch, axios, EventEmitter, amqplib, redis patterns |
| Python integration extractor | Table-driven unit | requests, httpx, pika, redis patterns |
| net/http extractor | Table-driven unit | HandleFunc, Handle, gorilla Methods chaining |
| Gin extractor | Table-driven unit | verbs, groups, middleware |
| Consumer detection (Go) | Table-driven unit | Subscribe, Consume, Listen, On patterns |
| Consumer detection (JS/TS) | Table-driven unit | on, consume, subscribe patterns |
| Conditional metadata | Unit | go_extractor: call inside if vs. not; FlowEdge.Conditional copied correctly |
| Conditional rendering | Golden-file | mermaid: -.-> for conditional; sequence: alt/opt blocks |
| LLM summary (gated) | Integration | mock summarizer; fallback on timeout |
| Cross-workspace stitching | Integration | two test workspaces, string-literal topic match, variable topic not matched |
| End-to-end | Integration (testutil.SetupTestDB) | index fixture with consumers + integration calls → flow doc searchable → stitching returns cross-service node |

## Migration

No new DB migrations required. `EdgeIntegration` was added in Phase 1 migration `00024`. Consumer detection reuses `EdgeHTTP` node convention. LLM summaries use existing document/chunk schema.

Config additions (hot-reloadable via existing koanf mechanism):
```yaml
flow:
  enabled: true         # Phase 1 gate (existing)
  max_depth: 10         # Phase 1 (existing)
  max_fanout: 5         # Phase 1 (existing)
  summary_enabled: false  # Phase 2D: LLM summaries (new)
```
