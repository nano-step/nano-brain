## ADDED Requirements

### Requirement: memory_expand tool registration
The MCP server SHALL register a `memory_expand` tool with description "Expand a compact search result to see full content" and input schema: `cacheKey` (string, required), `index` (number, optional, 1-based result index), `indices` (number[], optional, array of 1-based indices), `docid` (string, optional, document ID fallback).

#### Scenario: memory_expand appears in tool listing
- **WHEN** an MCP client lists available tools
- **THEN** `memory_expand` is listed with its description and input schema
- **THEN** the schema shows `cacheKey` as required and `index`/`indices`/`docid` as optional

## MODIFIED Requirements

### Requirement: Search tool parameter schema includes workspace
The MCP tool registration for `memory_search`, `memory_vsearch`, and `memory_query` SHALL include `workspace` in their input schema as an optional string parameter with description explaining the scoping behavior. Additionally, all three tools SHALL include a `compact` boolean parameter (optional, defaults to false) with description: "Set to true for compact single-line results with caching. Defaults to verbose."

#### Scenario: Tool schema advertises workspace parameter
- **WHEN** an MCP client lists available tools
- **THEN** `memory_search`, `memory_vsearch`, and `memory_query` each show a `workspace` parameter in their input schema
- **THEN** the parameter description explains: omit for current workspace, `"all"` for cross-workspace search

#### Scenario: Tool schema advertises compact parameter
- **WHEN** an MCP client lists available tools
- **THEN** `memory_search`, `memory_vsearch`, and `memory_query` each show a `compact` boolean parameter in their input schema
- **THEN** the parameter description explains: defaults to false (verbose), set to true for compact single-line results
