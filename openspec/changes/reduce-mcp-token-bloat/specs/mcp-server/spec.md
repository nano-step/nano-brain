## MODIFIED Requirements

### Requirement: All MCP tool handlers return valid responses
Every registered MCP tool SHALL return a valid JSON-RPC response for valid inputs, never an unhandled exception. Tool responses SHALL enforce size limits to prevent unbounded token consumption in AI agent context windows.

#### Scenario: memory_search with valid query
- **WHEN** `memory_search` is called with `{"query": "test"}` via JSON-RPC
- **THEN** a valid response with `content` array is returned

#### Scenario: memory_update with configured collections
- **WHEN** `memory_update` is called via JSON-RPC with collections configured
- **THEN** a valid response with reindex summary is returned, not a runtime error

#### Scenario: memory_status returns health info
- **WHEN** `memory_status` is called via JSON-RPC
- **THEN** a valid response with document count, chunk count, and collection info is returned

#### Scenario: Tool responses include truncation indicators
- **WHEN** any tool response is truncated due to size limits
- **THEN** the response SHALL include a clear indicator showing how many items were omitted (e.g., `... and N more`)
