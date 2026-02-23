## MODIFIED Requirements

### Requirement: ESM module compliance
All source files in `src/` SHALL use ESM `import` syntax exclusively. No `require()` calls SHALL exist in any TypeScript source file.

#### Scenario: Server starts under Node.js ESM runtime
- **WHEN** the MCP server is started via `node bin/cli.js mcp`
- **THEN** the server starts without `require is not defined` errors
- **THEN** all tool handlers execute without CJS/ESM compatibility errors

#### Scenario: No require() in source files
- **WHEN** running `grep -r "require(" src/` on the source directory
- **THEN** zero matches are returned (excluding comments and string literals)

### Requirement: Dynamic collection config reload
The `memory_update` tool handler SHALL reload the collection configuration file on every invocation, not use the cached startup value.

#### Scenario: Collection added after server start
- **WHEN** a user adds a collection via CLI (`collection add`) while the MCP server is running
- **THEN** calling `memory_update` through MCP indexes documents from the newly added collection
- **THEN** no server restart is required

#### Scenario: Collection removed after server start
- **WHEN** a user removes a collection via CLI while the MCP server is running
- **THEN** calling `memory_update` through MCP no longer indexes documents from the removed collection

### Requirement: All MCP tool handlers return valid responses
Every registered MCP tool SHALL return a valid JSON-RPC response for valid inputs, never an unhandled exception.

#### Scenario: memory_search with valid query
- **WHEN** `memory_search` is called with `{"query": "test"}` via JSON-RPC
- **THEN** a valid response with `content` array is returned

#### Scenario: memory_update with configured collections
- **WHEN** `memory_update` is called via JSON-RPC with collections configured
- **THEN** a valid response with reindex summary is returned, not a runtime error

#### Scenario: memory_status returns health info
- **WHEN** `memory_status` is called via JSON-RPC
- **THEN** a valid response with document count, chunk count, and collection info is returned
