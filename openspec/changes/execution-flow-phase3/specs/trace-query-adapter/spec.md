# Spec: Trace Query Adapter

## Purpose
Enable nano-brain to query existing distributed tracing backends (Jaeger, Grafana Tempo) and normalize span data into the FlowNode/FlowEdge model for rendering with existing Mermaid and sequence diagram renderers.

## Capability: `trace-span-store`

### Scenario: Query trace by ID
**Given** a Jaeger instance with trace `abc123` containing 5 spans across 3 services
**When** `SpanStore.GetTrace(ctx, "abc123")` is called
**Then** the store returns all 5 spans with correct parent-child relationships

### Scenario: Query traces by service
**Given** a Jaeger instance with 10 traces for service "backend"
**When** `SpanStore.QueryTraces(ctx, {Service: "backend", Limit: 5})` is called
**Then** the store returns up to 5 trace summaries with correct trace IDs, durations, and service lists

### Scenario: Query with time range filter
**Given** a Tempo instance with traces from the last hour
**When** `SpanStore.QueryTraces(ctx, {TimeRange: {Start: 1h ago, End: now}})` is called
**Then** only traces within the specified time range are returned

### Scenario: Query with duration filter
**Given** a Jaeger instance with traces of varying durations
**When** `SpanStore.QueryTraces(ctx, {MinDuration: 100ms, MaxDuration: 1s})` is called
**Then** only traces with duration between 100ms and 1s are returned

### Scenario: Tracing backend unreachable
**Given** the Jaeger endpoint is down
**When** `SpanStore.GetTrace(ctx, "abc123")` is called
**Then** the store returns an error with context (not a panic)

### Scenario: Empty results
**Given** a Tempo instance with no traces matching the query
**When** `SpanStore.QueryTraces(ctx, {Service: "nonexistent"})` is called
**Then** the store returns an empty slice (not an error)

## Capability: `trace-to-flow-normalization`

### Scenario: Linear chain normalization
**Given** 3 spans: A→B→C (A is root, B is child of A, C is child of B)
**When** `TraceToFlow(spans, "entry", serviceWorkspaceMap)` is called
**Then** the returned Flow contains 3 FlowNodes and 2 FlowEdges in the correct order

### Scenario: Fan-out normalization
**Given** 3 spans: A→B and A→C (A is root, B and C are both children of A)
**When** `TraceToFlow(spans, "entry", serviceWorkspaceMap)` is called
**Then** the returned Flow contains 3 FlowNodes and 2 FlowEdges with A as the common parent

### Scenario: Merge normalization
**Given** 3 spans: A→C and B→C (A and B are roots, C is child of both)
**When** `TraceToFlow(spans, "entry", serviceWorkspaceMap)` is called
**Then** the returned Flow contains 3 FlowNodes and 2 FlowEdges with C as the common child

### Scenario: External node detection
**Given** spans from services "backend" (mapped in ServiceWorkspaceMap) and "steam" (not mapped)
**When** `TraceToFlow(spans, "entry", {backend: "ws-hash-1"})` is called
**Then** nodes for "steam" have role "external" while nodes for "backend" have role "service"

### Scenario: Duration metadata preservation
**Given** spans with durations: A=50ms, B=30ms, C=20ms
**When** `TraceToFlow(spans, "entry", serviceWorkspaceMap)` is called
**Then** each FlowNode has the correct DurationMS value

### Scenario: Root span as entry
**Given** a trace with root span "POST /api/v1/write"
**When** `TraceToFlow(spans, "POST /api/v1/write", serviceWorkspaceMap)` is called
**Then** the root FlowNode has role "entry"

### Scenario: Edge deduplication
**Given** 3 consecutive A→B calls in the trace (3 spans with same parent-child)
**When** `TraceToFlow(spans, "entry", serviceWorkspaceMap)` is called
**Then** the flow has 1 A→B edge (not 3), with duration = max(3 calls)

### Scenario: Orphan span handling
**Given** spans with an orphan span D (no parent, not root)
**When** `TraceToFlow(spans, "entry", serviceWorkspaceMap)` is called
**Then** D is attached to a synthetic "orphan" node with role "external"

## Capability: `trace-flow-api`

### Scenario: memory_flow with source=trace
**Given** tracing is enabled and Jaeger has traces for "POST /api/v1/write"
**When** `memory_flow({workspace, entry: "POST /api/v1/write", source: "trace"})` is called
**Then** the response includes a valid Mermaid diagram with cross-service nodes

### Scenario: memory_flow with source=auto (traces available)
**Given** tracing is enabled and Jaeger has traces for "POST /api/v1/write"
**When** `memory_flow({workspace, entry: "POST /api/v1/write", source: "auto"})` is called
**Then** the response uses trace data (source="trace") and includes trace_id

### Scenario: memory_flow with source=auto (no traces)
**Given** tracing is enabled but Jaeger has no traces for "POST /api/v1/write"
**When** `memory_flow({workspace, entry: "POST /api/v1/write", source: "auto"})` is called
**Then** the response falls back to static analysis (source="static")

### Scenario: memory_flow with source=trace when disabled
**Given** tracing is disabled (Provider="")
**When** `memory_flow({workspace, entry: "POST /api/v1/write", source: "trace"})` is called
**Then** the response returns an error: "tracing not configured"

### Scenario: memory_flow with source=trace, no traces found
**Given** tracing is enabled but no traces match the entry
**When** `memory_flow({workspace, entry: "POST /api/v1/write", source: "trace"})` is called
**Then** the response returns an error: "no traces found for entry"

### Scenario: Trace selection (most recent)
**Given** 3 traces matching the entry, with StartTime: T1 < T2 < T3
**When** `memory_flow({workspace, entry: "POST /api/v1/write", source: "trace"})` is called
**Then** the response uses the trace with StartTime=T3 (most recent)

### Scenario: POST /api/v1/traces/query
**Given** tracing is enabled and Jaeger has traces for service "backend"
**When** `POST /api/v1/traces/query` is called with `{service: "backend", limit: 10}`
**Then** the response contains up to 10 trace summaries

### Scenario: memory_trace_discover MCP tool
**Given** tracing is enabled
**When** `memory_trace_discover({workspace, service: "backend"})` is called
**Then** the response contains trace summaries discoverable by the agent

## Non-Functional Requirements

### Performance
- Trace queries complete within 5 seconds (bounded by tracing backend latency)
- No local caching in v1 (tracing backends have their own optimization)
- Max 3 concurrent trace queries (configurable via MaxConcurrent)

### Reliability
- Tracing backend unreachable → graceful fallback to static (when source=auto)
- Malformed response → clear error message (no panic)
- Timeout → clear error message (no hang)
- Orphan spans → attached to synthetic node, not dropped

### Security
- Optional Bearer token for authenticated tracing backends
- No sensitive data logged (trace IDs only, no span payloads)
