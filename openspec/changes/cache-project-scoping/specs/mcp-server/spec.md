## MODIFIED Requirements

### Requirement: Vector search cache passes workspace context
The `memory_vsearch` MCP tool handler SHALL pass `currentProjectHash` when interacting with the query embedding cache. Since embedding cache is global, this ensures consistency with the Store interface while the actual storage uses `project_hash = 'global'`.

#### Scenario: memory_vsearch caches embedding with global scope
- **WHEN** `memory_vsearch` is called and the embedding is not cached
- **THEN** the embedding is computed and stored with `project_hash = 'global'`
- **THEN** subsequent calls with the same query from any workspace get a cache hit

### Requirement: Hybrid search passes workspace context to cache
The `memory_query` MCP tool handler SHALL pass `currentProjectHash` (or the explicit `workspace` parameter) through to `hybridSearch` so that expansion and reranking caches are project-scoped.

#### Scenario: memory_query caches expansion per workspace
- **WHEN** `memory_query` is called with `{"query": "auth"}` from workspace `"abc123"`
- **THEN** expansion cache is stored with `project_hash = "abc123"`
- **THEN** the same query from workspace `"def456"` does NOT get a cache hit for expansion
