## ADDED Requirements

### Requirement: MCP tool listings guide agent tool selection

MCP tool registrations SHALL include agent-facing descriptions that teach when to choose each nano-brain memory and code-intelligence tool. The guidance SHALL be visible from MCP `tools/list` output without requiring the agent to read README files, skills, or external documentation.

#### Scenario: Search tool descriptions teach search selection

- **WHEN** an MCP client lists tools
- **THEN** `memory_query` description SHALL identify it as the default hybrid search for broad/codebase questions
- **AND** `memory_search` description SHALL identify it as the exact-keyword/BM25 tool for errors, identifiers, and literal text
- **AND** `memory_vsearch` description SHALL identify it as the fuzzy semantic/vector tool for concepts where exact words may differ

#### Scenario: Code-intelligence tool descriptions teach graph selection

- **WHEN** an MCP client lists tools
- **THEN** `memory_symbols` description SHALL say to use it for known functions, classes, interfaces, types, constants, or variables
- **AND** `memory_graph` description SHALL say to use it for one-hop callers, callees, imports, or containment
- **AND** `memory_impact` description SHALL say to use it before changing a file or symbol to find affected callers/dependents
- **AND** `memory_trace` description SHALL say to use it to follow downstream call chains from an entry symbol

#### Scenario: Flow tool descriptions teach execution-flow selection

- **WHEN** an MCP client lists tools
- **THEN** `memory_flow` description SHALL say to use it for HTTP route/request execution flows
- **AND** `memory_flowchart` description SHALL say to use it for a function-level control-flow graph

#### Scenario: Support tool descriptions teach common workflow

- **WHEN** an MCP client lists tools
- **THEN** `memory_wake_up` description SHALL say to use it at session start for workspace orientation
- **AND** `memory_get` description SHALL say to use it after search when one result needs full content
- **AND** `memory_status` description SHALL say to use it when results look stale or indexing/embedding may be incomplete
