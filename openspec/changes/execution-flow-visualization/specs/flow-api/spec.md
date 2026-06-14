## ADDED Requirements

### Requirement: `memory_flow` MCP tool
The system SHALL provide a `memory_flow` MCP tool that returns the in-process flow and its Mermaid flowchart for a given HTTP entry point.

#### Scenario: Flow returned for an entry
- **WHEN** an agent calls `memory_flow({ workspace, entry: "POST /api/topup" })`
- **THEN** the response contains the entry, method, path, the ordered chain of nodes, the externals, and a Mermaid `graph TD` string

#### Scenario: Format selection
- **WHEN** the tool is called with `format: "json"`
- **THEN** the structured chain is returned without requiring the Mermaid string, and with `format: "mermaid"` (the default) the Mermaid string is included

#### Scenario: Unknown entry
- **WHEN** the requested entry point does not exist in the workspace graph
- **THEN** the tool returns an empty/!found result with a clear message rather than an error or fabricated flow

#### Scenario: Flow indexing disabled
- **WHEN** flow indexing is disabled in config and `memory_flow` is called
- **THEN** the tool returns a clear "flow indexing disabled" message rather than an empty or misleading result

### Requirement: REST flow endpoint
The system SHALL expose `POST /api/v1/graph/flow` that backs the `memory_flow` tool using the same flow-building core.

#### Scenario: Endpoint returns flow
- **WHEN** a client POSTs `{ workspace, entry, max_depth? }` to `/api/v1/graph/flow`
- **THEN** the response is the flow chain plus Mermaid for that entry, identical in shape to the `memory_flow` tool result

#### Scenario: Depth parameter
- **WHEN** the request includes `max_depth`
- **THEN** the returned flow honors that depth cap, defaulting when omitted
