## ADDED Requirements

### Requirement: Cache clear command scoped to current workspace
The CLI SHALL provide a `cache clear` command that deletes all cache entries for the current workspace's `projectHash`. The workspace is determined by `resolveDbPath(defaultDbPath, process.cwd())` and `sha256(cwd).substring(0, 12)`.

#### Scenario: Clear cache for current workspace
- **WHEN** user runs `nano-brain cache clear` from `/Users/alice/projects/my-app`
- **THEN** all `llm_cache` entries with `project_hash` matching the workspace hash are deleted
- **THEN** entries with `project_hash = 'global'` (embedding cache) are NOT deleted
- **THEN** entries from other workspaces are NOT deleted
- **THEN** a summary is printed: `Cleared N cache entries for workspace {hash}`

#### Scenario: No cache entries for workspace
- **WHEN** user runs `nano-brain cache clear` and no entries exist for the current workspace
- **THEN** output shows `Cleared 0 cache entries for workspace {hash}`

### Requirement: Cache clear --all flag
The `cache clear` command SHALL accept a `--all` flag that deletes ALL cache entries across all workspaces and types.

#### Scenario: Clear all cache entries
- **WHEN** user runs `nano-brain cache clear --all`
- **THEN** all rows in `llm_cache` are deleted regardless of `project_hash` or `type`
- **THEN** a summary is printed: `Cleared all cache entries (N total)`

### Requirement: Cache clear --type filter
The `cache clear` command SHALL accept a `--type=<type>` flag to delete only entries of a specific type (`embed`, `expand`, `rerank`).

#### Scenario: Clear only embedding cache
- **WHEN** user runs `nano-brain cache clear --type=embed`
- **THEN** only entries with `type = 'qembed'` for the current workspace are deleted
- **THEN** expansion and reranking cache entries are preserved

#### Scenario: Clear all embedding cache globally
- **WHEN** user runs `nano-brain cache clear --type=embed --all`
- **THEN** all entries with `type = 'qembed'` across all workspaces are deleted

#### Scenario: Invalid type value
- **WHEN** user runs `nano-brain cache clear --type=banana`
- **THEN** an error is printed: `Invalid cache type "banana". Valid types: embed, expand, rerank`
- **THEN** no entries are deleted

### Requirement: Cache stats command
The CLI SHALL provide a `cache stats` command that displays cache entry counts grouped by type and workspace.

#### Scenario: Cache stats with entries
- **WHEN** user runs `nano-brain cache stats`
- **THEN** output shows a table with columns: type, project_hash, count
- **THEN** entries are grouped by (type, project_hash)

#### Scenario: Cache stats with no entries
- **WHEN** user runs `nano-brain cache stats` and `llm_cache` is empty
- **THEN** output shows `No cache entries`

### Requirement: Cache command registered in CLI router
The `cache` command SHALL be registered in the main CLI command router alongside existing commands (mcp, init, collection, status, etc.).

#### Scenario: Help text includes cache command
- **WHEN** user runs `nano-brain --help`
- **THEN** the help output includes the `cache` command with subcommands `clear` and `stats`

#### Scenario: Unknown cache subcommand
- **WHEN** user runs `nano-brain cache foo`
- **THEN** an error is printed: `Unknown cache subcommand: foo`
- **THEN** exit code is 1
