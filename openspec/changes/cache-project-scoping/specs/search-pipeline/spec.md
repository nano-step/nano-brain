## MODIFIED Requirements

### Requirement: Expansion cache is project-scoped
The `hybridSearch` function SHALL pass `projectHash` when storing and retrieving query expansion cache entries. Expansion results for the same query text in different workspaces SHALL be stored as separate cache entries.

#### Scenario: Expansion cache hit in same workspace
- **WHEN** `hybridSearch` is called with query `"auth"` and `projectHash = "abc123"`
- **AND** a cached expansion result exists for `("auth", "abc123")`
- **THEN** the cached expansion variants are used without calling the expander

#### Scenario: Expansion cache miss across workspaces
- **WHEN** `hybridSearch` is called with query `"auth"` and `projectHash = "def456"`
- **AND** a cached expansion result exists only for `("auth", "abc123")`
- **THEN** the expander is called to generate new variants
- **THEN** the result is cached with `project_hash = "def456"`

#### Scenario: Expansion cache write includes project hash
- **WHEN** the expander generates variants for query `"auth"` in workspace `"abc123"`
- **THEN** `setCachedResult` is called with `projectHash = "abc123"` and `type = "expand"`

### Requirement: Reranking cache is project-scoped
The `hybridSearch` function SHALL pass `projectHash` when storing and retrieving reranking cache entries.

#### Scenario: Reranking cache includes project hash
- **WHEN** reranking results are cached for query `"auth"` with candidates in workspace `"abc123"`
- **THEN** `setCachedResult` is called with `projectHash = "abc123"` and `type = "rerank"`

#### Scenario: Reranking cache lookup includes project hash
- **WHEN** `hybridSearch` checks for cached reranking results
- **THEN** `getCachedResult` is called with the workspace's `projectHash`

### Requirement: Query embedding cache remains global
The `hybridSearch` function SHALL NOT pass a workspace-specific `projectHash` for query embedding cache operations. Embedding cache entries SHALL use `project_hash = 'global'`.

#### Scenario: Same query produces cache hit across workspaces
- **WHEN** query `"auth"` is embedded in workspace `"abc123"` and cached
- **AND** query `"auth"` is searched in workspace `"def456"`
- **THEN** the cached embedding vector is reused (cache hit)
- **THEN** no Ollama round-trip occurs
