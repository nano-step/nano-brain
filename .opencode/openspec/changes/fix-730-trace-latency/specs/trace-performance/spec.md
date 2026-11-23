## ADDED Requirements

### Requirement: Trace avoids repeated symbol-resolution queries

The MCP `memory_trace` handler SHALL reuse results for repeated bare
target symbols within a request, including symbols without a definition,
without reusing them across separate requests or workspaces.

#### Scenario: Repeated missing target
- **WHEN** multiple call edges target the same missing symbol
- **THEN** the handler SHALL resolve it at most once per request and
  preserve the current default omission or `include_external` behavior.

#### Scenario: Same symbol from different callers
- **WHEN** multiple callers target the same ambiguous bare symbol
- **THEN** the handler SHALL still select nearest definition relative
  to each caller independently and preserve deterministic BFS output.

### Requirement: Trace SQL lookup does not scan unrelated workspace data

Symbol resolution SHALL use an index-friendly workspace-scoped lookup;
any new migration SHALL be additive and verified on isolated test DB.

#### Scenario: Existing indexed workspace
- **WHEN** tracing a symbol on a workspace with unrelated documents
- **THEN** the lookup SHALL preserve match semantics without rescanning
  all unrelated workspace documents for every call edge.
