# Tasks: Execution Flow Visualization — Phase 3A (Trace Query Adapter)

## Task 1: Span Model + SpanStore Interface
**Status:** `[ ]`
**Files:** `internal/trace/span.go`, `internal/trace/store.go`

- [ ] Create `internal/trace/span.go` with `Span`, `Log`, `TraceQuery`, `TraceSummary`, `TimeRange` structs
- [ ] Create `internal/trace/store.go` with `SpanStore` interface (`GetTrace`, `QueryTraces`)
- [ ] Add unit tests for span model (JSON serialization, zero values)

## Task 2: Jaeger Adapter
**Status:** `[ ]`
**Files:** `internal/trace/jaeger.go`

- [ ] Implement `JaegerStore` struct with `NewJaegerStore(endpoint, timeout)` constructor
- [ ] Implement `GetTrace(ctx, traceID)` — calls `GET /api/v3/traces/{traceID}`
- [ ] Implement `QueryTraces(ctx, query)` — calls `GET /api/v3/traces` with query params
- [ ] Handle Jaeger v3 response format (parse spans, extract service names)
- [ ] Add unit tests with mock HTTP server (successful, timeout, malformed, empty)
- [ ] Add error wrapping: `fmt.Errorf("jaeger get trace: %w", err)`

## Task 3: Tempo Adapter
**Status:** `[ ]`
**Files:** `internal/trace/tempo.go`

- [ ] Implement `TempoStore` struct with `NewTempoStore(endpoint, timeout)` constructor
- [ ] Implement `GetTrace(ctx, traceID)` — calls `GET /api/traces/{traceID}`
- [ ] Implement `QueryTraces(ctx, query)` — calls `POST /api/search` with JSON body
- [ ] Handle Tempo response format (parse trace results, extract service names)
- [ ] Add unit tests with mock HTTP server (successful, timeout, malformed, empty)
- [ ] Add error wrapping: `fmt.Errorf("tempo get trace: %w", err)`

## Task 4: Trace-to-Flow Normalizer
**Status:** `[ ]`
**Files:** `internal/flow/trace_normalizer.go`

- [ ] Implement `TraceToFlow(spans []Span, entry string, serviceWorkspaceMap map[string]string) Flow`
- [ ] Build parent-child map from span array
- [ ] Create FlowNode for each UNIQUE service::operation (deduplicate by ID)
- [ ] Create FlowEdge for each UNIQUE parent→child pair (deduplicate, keep max duration)
- [ ] Classify node roles using serviceWorkspaceMap: root→entry, mapped services→service/handler, others→external
- [ ] Handle orphan spans (no parent, not root): attach to synthetic "orphan" node
- [ ] Add `FlowNode.DurationMS` and `FlowEdge.DurationMS` fields to existing structs (0 = not applicable)
- [ ] Add `Flow.Source` and `Flow.TraceID` fields
- [ ] Unit tests: linear chain, fan-out, merge, external nodes, depth cap, orphan spans, edge deduplication

## Task 5: Config Extension
**Status:** `[ ]`
**Files:** `internal/config/config.go`

- [ ] Add `TracingConfig` struct with `Provider`, `Endpoint`, `Timeout`, `DefaultService`, `BearerToken`, `JaegerAPIVersion`, `ServiceWorkspaceMap`, `MaxConcurrent` fields
- [ ] Add `Tracing TracingConfig` to main config struct
- [ ] Set defaults: `Provider: ""` (disabled), `Timeout: 5s`, `JaegerAPIVersion: "v3"`, `MaxConcurrent: 3`
- [ ] Add validation: if `Provider` is set, `Endpoint` must be non-empty
- [ ] Document: TracingConfig is NOT hot-reloadable (constructed at startup, immutable)

## Task 6: memory_flow Extension
**Status:** `[ ]`
**Files:** `internal/mcp/tools.go`

- [ ] Add `source` parameter to `memory_flow` MCP tool schema
- [ ] Add `FlowSource` type: `"static" | "trace" | "auto"`
- [ ] Implement source routing:
  - `"static"` → current behavior (unchanged)
  - `"trace"` → query SpanStore → select most recent trace → TraceToFlow → render. Error if no traces found.
  - `"auto"` → try trace first, fallback to static if no results or tracing disabled
- [ ] Return `source`, `trace_id`, `duration_ms` in response when applicable
- [ ] Add validation: `source=trace` requires tracing to be enabled

## Task 7: Trace Query Endpoint
**Status:** `[ ]`
**Files:** `internal/server/handlers/trace_query.go`

- [ ] Create `POST /api/v1/traces/query` handler (note: plural `/traces/` to avoid collision with existing `/api/v1/graph/trace`)
- [ ] New file `trace_query.go` (not `trace.go` which already exists for static graph trace)
- [ ] Request body: `{ service, operation, tags, time_range, min_duration, max_duration, limit }`
- [ ] Response: `{ traces: [{ trace_id, root_operation, duration_ms, span_count, services }] }`
- [ ] Validate request (service required, limit capped at 50)
- [ ] Add integration test with mock SpanStore

## Task 8: memory_trace_discover MCP Tool
**Status:** `[ ]`
**Files:** `internal/mcp/tools.go`

- [ ] Register `memory_trace_discover` MCP tool (named to avoid confusion with existing `memory_trace`)
- [ ] Parameters: `workspace`, `service`, `operation?`, `tags?`, `time_range?`, `min_duration?`, `max_duration?`, `limit?`
- [ ] Response: same shape as `POST /api/v1/traces/query`
- [ ] Add agent-facing documentation

## Task 9: Integration Tests (Optional)
**Status:** `[ ]`
**Files:** `internal/trace/integration_test.go` (build tag `integration`)

- [ ] Skip if Jaeger/Tempo not available (environment detection)
- [ ] Ingest known spans via Jaeger/Tempo API
- [ ] Query via SpanStore and verify results
- [ ] End-to-end: `memory_flow(source=trace)` returns correct chain + Mermaid

## Task 10: Documentation
**Status:** `[ ]`
**Files:** `.opencode/skills/nano-brain/SKILL.md`, `docs/trace-query.md` (optional)

- [ ] Update SKILL.md with `memory_trace_query` documentation
- [ ] Update `memory_flow` documentation with `source` parameter
- [ ] Add examples: trace query, trace-based flow visualization
- [ ] Add configuration reference for `TracingConfig`

## Acceptance Criteria

- [ ] `memory_flow(source=trace)` returns a valid Flow with chain + Mermaid when tracing is enabled
- [ ] `memory_flow(source=auto)` falls back to static when tracing is disabled or no traces found
- [ ] `POST /api/v1/trace/query` returns trace summaries for a given service
- [ ] `memory_trace_query` MCP tool works and returns discoverable traces
- [ ] Unit tests pass for all new code
- [ ] No existing functionality broken (static flows continue working)
