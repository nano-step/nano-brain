## ADDED Requirements

### Requirement: Condition labels in flow response (Phase 1a)
`POST /api/v1/graph/flow` with `format:"json"` SHALL include `condition_label` on edges that have a conditional metadata.

#### Scenario: Edge with condition carries label
- **WHEN** a client requests `format:"json"` for an entry with conditional edges
- **THEN** each conditional edge includes `"condition_label": "<predicate text>"`
- **AND** the `conditional` boolean remains `true`

#### Scenario: Existing formats are unaffected
- **WHEN** a client requests `format:"mermaid"` or `"sequence"`
- **THEN** the response is identical to current behavior (mermaid renderer labels conditional edges)

### Requirement: Flowchart endpoint (Phase 1b)
`POST /api/v1/graph/flowchart` SHALL return the control-flow graph of the entry's handler as `{ found, entry, method, path, cfg, status }`.

#### Scenario: Flowchart requested for a JS/TS handler with branches
- **WHEN** a client POSTs `{ entry: "POST /express-app/api/game" }`
- **AND** the handler has a stored CFG
- **THEN** the response has `found: true`
- **AND** `cfg.nodes` includes `decision` and `terminal` nodes
- **AND** `cfg.edges` carry `branch` labels
- **AND** `cfg.status` is `"complete"`

#### Scenario: Flowchart requested for a handler without a CFG
- **WHEN** the entry's handler is non-JS/TS or has no stored CFG
- **THEN** the response has `found: true` and `cfg: null`
- **AND** `status` is `"no_cfg"`

#### Scenario: CFG is too complex
- **WHEN** the stored CFG has `status: "truncated"` (exceeded 500 nodes)
- **THEN** the response has `cfg: null` and `status: "too_complex"`
- **AND** the client falls back to the graph view

#### Scenario: Entry not found
- **WHEN** the entry does not match any HTTP edge
- **THEN** the response has `found: false`

#### Scenario: Existing flow endpoint is unaffected
- **WHEN** a client calls `POST /api/v1/graph/flow` with any format
- **THEN** the response is identical to current behavior (no `cfg` field)

### Requirement: MCP flowchart tool (Phase 1b)
A new MCP tool `memory_flowchart` SHALL return the CFG spec for a given node location.

#### Scenario: memory_flowchart returns a flowchart
- **WHEN** an agent calls `memory_flowchart` with `node: "routes/game.ts::15-42"`
- **AND** a CFG exists for that location
- **THEN** the tool result includes the `cfg` object with `nodes` and `edges`

#### Scenario: memory_flowchart with no CFG
- **WHEN** the node location has no stored CFG
- **THEN** the tool result has `cfg: null`

### Requirement: Entry resolution via HTTP edges
The flowchart endpoint SHALL resolve the entry to a handler's source location using the existing HTTP edges, then look up the CFG by location.

#### Scenario: Entry resolves to handler location
- **WHEN** `POST /express-app/api/game` maps to `routes/game.ts::15-42` via HTTP edges
- **THEN** the endpoint queries `function_flowcharts` by `(workspace_hash, source_file, start_line, end_line)`
