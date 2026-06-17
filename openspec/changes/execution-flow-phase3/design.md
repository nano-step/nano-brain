# Design: Execution Flow Visualization — Phase 3A (Trace Query Adapter)

## Context

Phase 1 delivered static in-process flow analysis: Echo route extraction → FlowBuilder → Mermaid flowcharts + searchable summaries. Phase 2 extended this with sequence diagrams, multi-language extractors, cross-workspace stitching, and LLM summaries. Both phases operate at index time from source code.

Phase 3 addresses the gap: **cross-service runtime tracing**. The user's example chain (`user trade → backend → old-backend → redis → tradebot → steam? → fe → db`) crosses 8 service boundaries. Static analysis sees terminal `external` leaves at each boundary. Runtime distributed tracing captures the actual call chain.

**Decision: Query Adapter pattern (3A) vs OTel Ingestion (3B).** Phase 3A queries existing tracing backends (Jaeger/Tempo) that already capture runtime spans. Phase 3B would require all services to add OTel SDK instrumentation and store spans locally. 3A is the right first step because:
- Zero service changes required (tracing backends already exist)
- Immediate value from existing instrumentation investment
- Validates the span→Flow normalization pipeline before investing in storage
- If 3A proves valuable, 3B becomes a natural next step (same normalization, different source)

## Goals / Non-Goals

**Goals**
- Query Jaeger/Tempo for traces matching an entry point or service+operation
- Normalize span arrays into FlowNode/FlowEdge for rendering with existing Phase 1/2 renderers
- Surface trace-based flows via `memory_flow` (with `source=trace` parameter)
- Provide raw trace query API for agent-side discovery

**Non-Goals**
- Storing trace spans in nano-brain's PostgreSQL (Phase 3B)
- OTel SDK instrumentation of services (Phase 3B)
- Real-time trace streaming / WebSocket updates (Phase 3C)
- Automatic static+trace hybrid overlay with confidence scoring (Phase 3C)
- Trace-based flow materialization / indexing (trace data is ephemeral, not indexed)
- Hot-reload of TracingConfig at runtime (constructed at server startup, immutable until restart)

## Architecture

```
QUERY TIME:
  memory_flow(entry, source=trace)
    ──▶ Parse entry as "METHOD /path"
    ──▶ SpanStore.QueryTraces(operation="METHOD /path", service=<DefaultService>)
      ──▶ Jaeger HTTP API / Tempo HTTP API
    ──▶ Select most recent matching trace (by StartTime descending)
    ──▶ []Span
    ──▶ TraceToFlow(spans, entry) ──▶ Flow (same shape as static)
    ──▶ MermaidFlowchart(Flow) / SequenceDiagram(Flow)
    ──▶ { chain, mermaid, externals, trace_id, duration_ms, source: "trace" }

TRACE DISCOVERY:
  memory_trace_discover(service, operation, time_range)
    ──▶ SpanStore.QueryTraces()
    ──▶ [{ trace_id, root_operation, duration_ms, span_count, services }]
```

### Entry-to-Query Mapping

When `memory_flow` is called with `source=trace`, the `entry` parameter (e.g., `"POST /api/v1/write"`) is parsed and mapped to a trace query:

1. **Parse entry**: split into `Method` and `Path` (e.g., `Method="POST"`, `Path="/api/v1/write"`)
2. **Query SpanStore**: `QueryTraces({Operation: "POST /api/v1/write", Service: <DefaultService>})`
   - If `DefaultService` is configured, query only that service
   - If `DefaultService` is empty, query all services (broader, slower)
3. **Select trace**: pick the **most recent** trace matching the query (by `StartTime` descending)
4. **If no traces found**: return error "no traces found for entry" (when `source=trace`) or fall back to static (when `source=auto`)

### Workspace-to-Service Mapping

The normalizer classifies nodes as `internal` or `external` based on whether the service name maps to an indexed workspace. Two mechanisms:

1. **Explicit mapping** (recommended): `TracingConfig.ServiceWorkspaceMap map[string]string` maps service names to workspace hashes. Example: `{backend: "ws-hash-1", old-backend: "ws-hash-2"}`. Nodes matching a mapped service get `service`/`handler` role; others get `external`.
2. **Default service fallback**: if `ServiceWorkspaceMap` is empty, only `DefaultService` is classified as internal; all other services are `external`.

This keeps the classification deterministic and explicit rather than relying on heuristics that could misclassify.

### SpanStore Interface

```go
// internal/trace/store.go
type SpanStore interface {
    // GetTrace retrieves all spans for a given trace ID.
    GetTrace(ctx context.Context, traceID string) ([]Span, error)
    
    // QueryTraces finds traces matching the query criteria.
    // Returns summary information (not full spans) for discovery.
    QueryTraces(ctx context.Context, query TraceQuery) ([]TraceSummary, error)
}

type TraceQuery struct {
    Service     string            // filter by service name
    Operation   string            // filter by operation/span name
    Tags        map[string]string // filter by tag key-value pairs
    MinDuration time.Duration     // minimum trace duration
    MaxDuration time.Duration     // maximum trace duration
    TimeRange   TimeRange         // start/end time window
    Limit       int               // max results (default 20)
}

type TraceSummary struct {
    TraceID      string
    RootOperation string
    DurationMS   int64
    SpanCount    int
    Services     []string
    StartTime    time.Time
}

type TimeRange struct {
    Start time.Time
    End   time.Time
}
```

### Span Model

```go
// internal/trace/span.go
type Span struct {
    TraceID       string
    SpanID        string
    ParentSpanID  string            // "" for root span
    OperationName string            // e.g. "POST /api/v1/write"
    ServiceName   string            // e.g. "backend", "old-backend"
    StartTime     time.Time
    Duration      time.Duration
    Tags          map[string]string // e.g. {"http.method": "POST", "http.url": "/api/v1/write"}
    Logs          []Log
}

type Log struct {
    Timestamp time.Time
    Fields    map[string]string
}
```

### Jaeger Adapter

```go
// internal/trace/jaeger.go
type JaegerStore struct {
    endpoint   string        // e.g. "http://localhost:16686"
    httpClient *http.Client
    timeout    time.Duration
    apiVersion string        // "v3" (default) or "v2" for older deployments
}
```

**API version support:**
- **v3** (default, Jaeger 1.62+): `GET /api/v3/traces/{traceID}` and `GET /api/v3/traces?service=X&operation=Y`
- **v2** (Jaeger 1.35-1.61): `GET /api/v2/traces/{traceID}` and `GET/api/traces?service=X&operation=Y` (note: different path structure)
- **v1** (legacy): `GET /api/traces/{traceID}` and `GET /api/traces?service=X&operation=Y`

The adapter auto-detects v3 availability on first call (try v3 endpoint, fall back to v2 if 404). `TracingConfig.JaegerAPIVersion` can override auto-detection if needed.

### Tempo Adapter

```go
// internal/trace/tempo.go
type TempoStore struct {
    endpoint   string        // e.g. "http://localhost:3200"
    httpClient *http.Client
    timeout    time.Duration
}
```

**TraceQL translation:** The `TraceQuery` struct is translated to Tempo's TraceQL query language:
```go
func translateToTraceQL(q TraceQuery) string {
    // Service: .service.name = "backend"
    // Operation: .name = "POST /api/v1/write"
    // Tags: .http.method = "POST"
    // Duration: duration > 100ms
    // Combine with && operators
}
```

For power users, `TraceQuery` includes an optional `RawTraceQL string` field that bypasses translation.

### Trace-to-Flow Normalization

```go
// internal/flow/trace_normalizer.go
func TraceToFlow(spans []span.Span, entry string, serviceWorkspaceMap map[string]string) Flow {
    // 1. Build parent-child map
    // 2. Create FlowNode for each UNIQUE service::operation (deduplicate by ID)
    // 3. Create FlowEdge for each UNIQUE parent→child pair (deduplicate, keep max duration)
    // 4. Classify node roles using serviceWorkspaceMap:
    //    - root span → "entry"
    //    - service in ServiceWorkspaceMap → "service"/"handler" (by naming heuristics)
    //    - service NOT in map → "external"
    // 5. Handle orphan spans (no parent, not root): attach to synthetic "orphan" node
    // 6. Attach duration metadata to nodes and edges
    // 7. Return Flow (same structure as static FlowBuilder output)
}
```

**Key decisions:**
- **Node ID**: `"serviceName::operationName"` (not span ID, because span IDs are opaque and don't convey meaning)
- **Node deduplication**: if 3 spans have `backend::POST /api/v1/write`, they produce 1 FlowNode with that ID
- **Edge deduplication**: if A calls B 3 times (3 spans with A→B parent-child), they produce 1 A→B edge (with max duration of the 3 calls)
- **Orphan spans**: spans with no parent (other than root) are attached to a synthetic "orphan" node with role "external" — prevents disconnected subgraphs
- **Duration metadata**: both `FlowNode.DurationMS` and `FlowEdge.DurationMS` (edge duration = max child span duration) — only populated for trace flows
- **Trace ID**: stored in `Flow.Metadata["trace_id"]` for linking back to the source trace

### Config Extension

```go
// internal/config/config.go addition
type TracingConfig struct {
    Provider          string            `koanf:"provider"`           // "jaeger" | "tempo" | "" (disabled)
    Endpoint          string            `koanf:"endpoint"`           // base URL, e.g. "http://localhost:16686"
    Timeout           time.Duration     `koanf:"timeout"`            // HTTP timeout (default 5s)
    DefaultService    string            `koanf:"default_service"`    // default service name for queries
    BearerToken       string            `koanf:"bearer_token"`       // optional auth token
    JaegerAPIVersion  string            `koanf:"jaeger_api_version"` // "v3" (default), "v2", "v1"
    ServiceWorkspaceMap map[string]string `koanf:"service_workspace_map"` // service name → workspace hash
    MaxConcurrent     int               `koanf:"max_concurrent"`     // max concurrent trace queries (default 3)
}
```

**Config decisions:**
- **`JaegerAPIVersion`**: auto-detected by default (try v3, fall back to v2). Override only if auto-detection fails.
- **`ServiceWorkspaceMap`**: explicit mapping from service names to workspace hashes. If empty, only `DefaultService` is classified as internal.
- **`MaxConcurrent`**: semaphore guard to prevent overwhelming the tracing backend. Default 3 is conservative — increase if backend can handle more.
- **Not hot-reloadable**: `TracingConfig` is constructed at server startup and immutable until restart. Changing provider/endpoint requires a server restart.

### memory_flow Extension

Existing signature:
```
memory_flow({workspace, entry, max_depth?, format?})
```

Extended signature:
```
memory_flow({workspace, entry, max_depth?, format?, source?})
```

Where `source` is:
- `"static"` (default when tracing disabled) — current behavior
- `"trace"` (requires tracing enabled) — queries SpanStore, normalizes, renders
- `"auto"` (default when tracing enabled) — tries trace first, falls back to static if no results

**Trace selection strategy:** When multiple traces match the entry query, select the **most recent** trace (by `StartTime` descending). This is deterministic and predictable. The `trace_id` of the selected trace is returned in the response for debugging.

**Error handling:**
- `source=trace` + no traces found → return error "no traces found for entry"
- `source=auto` + no traces found → fall back to static analysis
- `source=trace` + tracing disabled → return error "tracing not configured"
- `source=auto` + tracing disabled → fall back to static analysis (no error)

## Data Model

No new database tables. Trace data lives in the external tracing backend (Jaeger/Tempo) with its own storage and retention policies.

The `Flow` struct gains optional trace metadata:
```go
type Flow struct {
    Entry    string
    Method   string
    Path     string
    Nodes    []FlowNode
    Edges    []FlowEdge
    Externals []string
    Source   string // "static" or "trace"
    TraceID  string // only for trace flows
    Duration time.Duration // only for trace flows
}
```

`FlowNode` gains optional duration:
```go
type FlowNode struct {
    ID         string
    Name       string
    Role       string
    Package    string
    DurationMS int64 // 0 = not applicable (static flows)
}
```

`FlowEdge` gains optional duration:
```go
type FlowEdge struct {
    From       string
    To         string
    Kind       string
    Label      string
    DurationMS int64 // 0 = not applicable (static flows)
}
```

## Testing Strategy

### Unit Tests
- **TraceToFlow**: table-driven tests with synthetic span arrays. Cover: linear chain (A→B→C), fan-out (A→B, A→C), merge (A→C, B→C), external nodes (A→X where X is not in workspace), depth cap, cycle detection (shouldn't exist in traces but handle gracefully).
- **JaegerStore**: mock HTTP server returning Jaeger v3 JSON responses. Test: successful query, timeout, malformed response, empty results.
- **TempoStore**: mock HTTP server returning Tempo JSON responses. Same coverage.

### Integration Tests (requires Jaeger/Tempo instance)
- Start a test Jaeger/Tempo instance (Docker or testcontainers)
- Ingest known spans via Jaeger/Tempo API
- Query via SpanStore and verify results match
- End-to-end: `memory_flow(source=trace)` returns correct chain + Mermaid

### Fallback Tests
- Tracing backend unreachable → `source=auto` falls back to static
- `source=trace` with no matching traces → returns empty chain with clear error message
- Tracing disabled in config → `source=trace` returns "tracing not configured"

## Migration & Rollback

No database migration needed. This is purely additive — new package, new config fields, new endpoints. Rollback is removing the new config and endpoints.

- Forward: add `TracingConfig` to config, set `provider: "jaeger"` and `endpoint`, deploy. `memory_flow` gains `source` parameter.
- Rollback: remove `TracingConfig` from config. All trace-related features become inert. Static flows continue working.

## Implementation Order

1. **Span model + SpanStore interface** (`internal/trace/span.go`, `internal/trace/store.go`)
2. **Jaeger adapter** (`internal/trace/jaeger.go`) + unit tests with mock server
3. **Tempo adapter** (`internal/trace/tempo.go`) + unit tests with mock server
4. **Trace-to-Flow normalizer** (`internal/flow/trace_normalizer.go`) + unit tests
5. **Config extension** (`internal/config/config.go`)
6. **memory_flow extension** (`internal/mcp/tools.go`) — add `source` parameter
7. **POST /api/v1/trace/query** handler (`internal/server/handlers/trace.go`)
8. **memory_trace_query** MCP tool (`internal/mcp/tools.go`)
9. **Integration tests** (optional, requires Jaeger/Tempo Docker)
10. **Documentation** — update SKILL.md, add trace query examples
