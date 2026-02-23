## ADDED Requirements

### Requirement: Search tools support workspace filtering
The `memory_search`, `memory_vsearch`, and `memory_query` MCP tools SHALL accept an optional `workspace` parameter. When omitted, results are scoped to the current workspace and global documents. When set to `"all"`, results include all workspaces.

#### Scenario: memory_search with default workspace scoping
- **WHEN** `memory_search` is called with `{"query": "test"}` and no `workspace` parameter
- **THEN** results are filtered to `currentProjectHash` and `'global'` documents only

#### Scenario: memory_vsearch with workspace="all"
- **WHEN** `memory_vsearch` is called with `{"query": "test", "workspace": "all"}`
- **THEN** results include documents from all workspaces

#### Scenario: memory_query with specific workspace
- **WHEN** `memory_query` is called with `{"query": "test", "workspace": "abc123def456"}`
- **THEN** results are filtered to `project_hash = 'abc123def456'` and `project_hash = 'global'`

### Requirement: memory_status reports storage usage
The `memory_status` tool SHALL report per-workspace document counts and total storage size, in addition to existing health information.

#### Scenario: memory_status with workspace data
- **WHEN** `memory_status` is called after documents from multiple workspaces are indexed
- **THEN** the response includes a breakdown of document counts per workspace (projectHash)
- **THEN** the response includes total storage size (DB + sessions directory)
- **THEN** the response includes storage limit configuration (maxSize, retention, minFreeDisk)

### Requirement: Search tool parameter schema includes workspace
The MCP tool registration for `memory_search`, `memory_vsearch`, and `memory_query` SHALL include `workspace` in their input schema as an optional string parameter with description explaining the scoping behavior.

#### Scenario: Tool schema advertises workspace parameter
- **WHEN** an MCP client lists available tools
- **THEN** `memory_search`, `memory_vsearch`, and `memory_query` each show a `workspace` parameter in their input schema
- **THEN** the parameter description explains: omit for current workspace, `"all"` for cross-workspace search
