## ADDED Requirements

### Requirement: Cache entries participate in workspace isolation
Cache entries in `llm_cache` SHALL be tagged with `project_hash` matching the workspace that created them. Workspace-scoped cache types (expansion, reranking) SHALL only return hits for the requesting workspace. Global cache types (embedding) SHALL use `project_hash = 'global'`.

#### Scenario: Expansion cache isolated between workspaces
- **WHEN** workspace `"abc123"` caches expansion for query `"auth"`
- **AND** workspace `"def456"` searches for `"auth"`
- **THEN** workspace `"def456"` does NOT get workspace `"abc123"`'s expansion cache

#### Scenario: Embedding cache shared across workspaces
- **WHEN** workspace `"abc123"` caches embedding for query `"auth"`
- **AND** workspace `"def456"` searches for `"auth"`
- **THEN** workspace `"def456"` gets the cached embedding (global scope)

### Requirement: Workspace cache cleanup on clearWorkspace
The `clearWorkspace(projectHash)` function SHALL also delete `llm_cache` entries with the matching `project_hash`, in addition to existing document and embedding cleanup.

#### Scenario: clearWorkspace removes cache entries
- **WHEN** `clearWorkspace("abc123")` is called
- **THEN** all `llm_cache` entries with `project_hash = 'abc123'` are deleted
- **THEN** entries with `project_hash = 'global'` are NOT deleted
- **THEN** entries from other workspaces are NOT deleted

### Requirement: Store interface supports scoped cache operations
The `Store` interface methods `getCachedResult` and `setCachedResult` SHALL accept an optional `projectHash` parameter. When omitted, `'global'` is used. The `clearQueryEmbeddingCache` method SHALL accept an optional `projectHash` parameter to scope deletion.

#### Scenario: getCachedResult with projectHash
- **WHEN** `getCachedResult(hash, "abc123")` is called
- **THEN** only entries matching both `hash` and `project_hash = 'abc123'` are returned

#### Scenario: getCachedResult without projectHash
- **WHEN** `getCachedResult(hash)` is called without projectHash
- **THEN** entries matching `hash` and `project_hash = 'global'` are returned

#### Scenario: clearQueryEmbeddingCache scoped to type
- **WHEN** `clearQueryEmbeddingCache()` is called without arguments
- **THEN** only entries with `type = 'qembed'` are deleted
- **THEN** expansion and reranking cache entries are preserved
