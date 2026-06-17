## ADDED Requirements

### Requirement: Version endpoint
The server SHALL expose `GET /api/version` returning JSON with the current version and API compatibility range.

#### Scenario: Version returns valid JSON
- **WHEN** a client issues `GET /api/version`
- **THEN** the response is `200 OK` with `Content-Type: application/json`
- **AND** the body contains `{version: string, migration_version: int, api_min: string, api_max: string}`

#### Scenario: Version endpoint is public (no auth required)
- **WHEN** a client issues `GET /api/version` without authentication headers
- **THEN** the server returns `200 OK` with the version JSON
- **SO THAT** the dashboard can check compatibility before authenticating

#### Scenario: Version strings are semver
- **WHEN** the version endpoint returns
- **THEN** `version` is a valid semver string (e.g., `"2026.6.2.1"`)
- **AND** `api_min` and `api_max` are valid semver strings defining the supported API range

### Requirement: Flow response nodes and edges
The `POST /api/v1/graph/flow` response SHALL include `nodes[]` and `edges[]` arrays alongside the existing `mermaid`, `chain`, and `externals` fields. Each node SHALL have `{id: string, label: string, role: string, kind: string, line: int}`. Each edge SHALL have `{source: string, target: string, kind: string, conditional: bool}`.

#### Scenario: Flow response includes nodes and edges
- **WHEN** a client issues `POST /api/v1/graph/flow` with a valid entry point
- **THEN** the response includes `nodes` array with at least one node
- **AND** each node has `id`, `label`, `role`, `kind`, `line` fields
- **AND** the response includes `edges` array
- **AND** each edge has `source`, `target`, `kind`, `conditional` fields

#### Scenario: Flow response with empty graph
- **WHEN** the entry point has no reachable code
- **THEN** the response has `nodes: []` and `edges: []`
- **AND** `mermaid` is an empty string or "graph TD"

#### Scenario: Conditional edges are flagged
- **WHEN** a flow edge represents a conditional branch
- **THEN** the edge has `conditional: true`
- **AND** the dashboard renderer SHALL display this edge as a dashed line

#### Scenario: Backward compatibility preserved
- **WHEN** an existing client reads the flow response
- **THEN** the `mermaid`, `chain`, and `externals` fields are present and unchanged
- **AND** the new `nodes`/`edges` fields are additive (no existing fields removed or renamed)

### Requirement: MCP memory_flow field parity
The MCP `memory_flow` tool response SHALL include `nodes[]` and `edges[]` with the same schema as the HTTP `POST /api/v1/graph/flow` endpoint.

#### Scenario: MCP tool returns nodes and edges
- **WHEN** a client calls the `memory_flow` MCP tool with a valid entry point
- **THEN** the response includes `nodes` and `edges` arrays
- **AND** the field schema matches the HTTP endpoint exactly

### Requirement: API version check in dashboard
The dashboard SHALL call `GET /api/version` on startup and display a compatibility banner. If the API version is outside the supported range, a warning banner SHALL be shown.

#### Scenario: Version compatible
- **WHEN** the API version is within `SUPPORTED_API_RANGE`
- **THEN** the banner shows "Connected to API vX.Y.Z" in green

#### Scenario: Version incompatible
- **WHEN** the API version is outside `SUPPORTED_API_RANGE`
- **THEN** the banner shows a yellow warning with the detected version and supported range

#### Scenario: API unreachable
- **WHEN** the API is not running or unreachable
- **THEN** the banner shows a red error with "Cannot connect to nano-brain API"
