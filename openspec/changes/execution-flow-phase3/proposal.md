# Execution Flow Visualization — Phase 3: Runtime Distributed Tracing

## Why

Phases 1 and 2 deliver static, in-process flow analysis: the Echo extractor, flow builder, Mermaid renderer, and sequence diagrams — all resolved from source code at index time. These work well for single-service, single-repo flows, but they cannot answer the user's core question: *"when a user makes a trade, what chain of calls actually executes across backend → old-backend → redis → tradebot → steam → frontend → db?"*

Cross-service calls are **runtime phenomena**. A `queue.Publish("trade.created")` in one repo and a `queue.Consume("trade.created")` in another share only a runtime string and broker config — not a code edge. Static analysis produces a terminal `external` leaf node at every cross-service boundary, which is honest about what it knows but useless for tracing end-to-end.

Phase 3 solves this by querying **existing distributed tracing systems** (Jaeger, Grafana Tempo) that already capture real call chains at runtime. Rather than requiring services to adopt OpenTelemetry from scratch (Phase 3B — deferred), Phase 3A queries the tracing backends already in production and normalizes their span data into the FlowNode/FlowEdge model, enabling the existing Mermaid and sequence diagram renderers to produce accurate cross-service visualizations.

**What this delivers:**
- Query real distributed traces (Jaeger/Tempo) for a given entry point
- Normalize span data to FlowNode/FlowEdge (reusing Phase 1/2 renderers)
- Render cross-service flows as Mermaid flowcharts and sequence diagrams
- Surface trace-based flows in `memory_flow` alongside static flows

**What this does NOT deliver (deferred):**
- Storing trace spans in nano-brain's own database (Phase 3B)
- Requiring services to add OTel instrumentation (Phase 3B)
- Real-time streaming of traces (Phase 3C)
- Automatic trace-to-static hybrid overlay (Phase 3C)

## What Changes

- **New package `internal/trace/`** — `SpanStore` interface with `GetTrace(ctx, traceID) ([]Span, error)` and `QueryTraces(ctx, query TraceQuery) ([]TraceSummary, error)`. Two implementations: `JaegerStore` (Jaeger HTTP API) and `TempoStore` (Grafana Tempo HTTP API).

- **Span model** — `internal/trace/span.go` defines `Span{TraceID, SpanID, ParentSpanID, OperationName, ServiceName, StartTime, Duration, Tags map[string]string, Logs []Log}` and `TraceQuery{Service, Operation, Tags, MinDuration, MaxDuration, TimeRange}`.

- **Trace-to-Flow normalization** — `internal/flow/trace_normalizer.go` converts `[]Span` into `Flow` (the same structure used by Phase 1 flow builder). Parent-child relationships become `FlowEdge`s. Service name + operation name become `FlowNode`s. External services (not in the indexed workspace) get `external` role. Duration metadata is attached to nodes and edges for optional annotation.

- **Config extension** — `Config.Tracing` adds `Provider string` ("jaeger" | "tempo" | ""), `Endpoint string` (base URL), `Timeout`, and `DefaultService string`. Feature is inert when Provider is empty.

- **`memory_flow` extension** — new optional parameter `source: "static" | "trace" | "auto"` (default "auto"). When `source=trace` or `source=auto` with trace data available, queries the SpanStore instead of (or in addition to) the static graph. Returns the same `{entry, method, path, chain, mermaid, externals}` shape.

- **REST endpoint extension** — `POST /api/v1/graph/flow` accepts the same `source` parameter.

- **New endpoint `POST /api/v1/traces/query`** — raw trace query: accepts `{service, operation, tags, time_range, min_duration}` and returns `{traces: [{trace_id, root_operation, duration_ms, span_count, services}]}` for discovery. Note: uses plural `/traces/` to avoid collision with existing `/api/v1/graph/trace` (static symbol trace).

- **New MCP tool `memory_trace_discover`** — wraps `POST /api/v1/traces/query` for agent-side trace discovery. Named `memory_trace_discover` (not `memory_trace_query`) to avoid confusion with the existing `memory_trace` MCP tool (static graph trace).

## Capabilities

### New Capabilities
- `trace-span-store`: Pluggable SpanStore interface for querying distributed traces from Jaeger or Tempo backends.
- `trace-to-flow-normalization`: Convert span arrays into FlowNode/FlowEdge for rendering with existing Mermaid/sequence renderers.
- `trace-flow-api`: Expose trace-based flows via `memory_flow` (with `source=trace`) and `POST /api/v1/trace/query`.

### Modified Capabilities
- `flow-api`: `memory_flow` gains `source` parameter to select static vs trace vs auto resolution.

## Impact

- **Code affected**:
  - `internal/trace/` — new package: `store.go` (interface), `jaeger.go` (Jaeger adapter), `tempo.go` (Tempo adapter), `span.go` (models)
  - `internal/flow/trace_normalizer.go` — new: span→Flow conversion
  - `internal/flow/` — extend `Flow` struct with optional trace metadata (trace_id, duration)
  - `internal/config/config.go` — add `TracingConfig{Provider, Endpoint, Timeout, DefaultService}`
  - `internal/mcp/tools.go` — register `memory_trace_query`; extend `memory_flow` params
  - `internal/server/handlers/trace.go` — new `POST /api/v1/trace/query` handler
  - `internal/server/handlers/flow.go` — extend to accept `source` param

- **Dependencies**: Go `net/http` for Jaeger/Tempo REST calls. No new external Go packages. Requires a running Jaeger or Tempo instance at config time (dev/test only).

- **Performance**: All trace queries are read-only HTTP calls to external backends. Latency is bounded by the tracing backend (typically <100ms for recent traces). No local caching needed for v1 — tracing backends have their own query optimization.

- **Risk**: The main risk is **trace availability** — not all services may be instrumented, and trace data has retention limits (typically 14-30 days). Phase 3A handles this gracefully: if no traces match, `memory_flow` falls back to static analysis (when `source=auto`). The secondary risk is **schema divergence** between Jaeger and Tempo response formats — addressed by the SpanStore abstraction layer.

## Open Questions

1. **Jaeger vs Tempo API compatibility** — do both backends support equivalent query patterns? Need to verify Tempo's search API against Jaeger's during implementation.
2. **Trace retention** — how far back can we query? This affects whether trace-based flows are useful for historical debugging vs only recent activity.
3. **Authentication** — do Jaeger/Tempo endpoints require auth tokens? Config should support optional Bearer token.
