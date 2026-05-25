## ADDED Requirements

### Requirement: CLI rm command accepts workspace identifier

The `nano-brain rm <workspace>` command SHALL accept a workspace identifier in one of three forms: an absolute path, a hex hash prefix (4-12 characters), or a workspace name (basename of the workspace root). The command SHALL resolve the identifier to a unique project_hash before proceeding.

#### Scenario: Remove workspace by absolute path
- **WHEN** user runs `nano-brain rm /Users/me/projects/my-app`
- **THEN** the system computes the project_hash from the path and removes all data for that workspace

#### Scenario: Remove workspace by hash prefix
- **WHEN** user runs `nano-brain rm 0ac58c`
- **THEN** the system matches the prefix against known workspace hashes and removes all data for the matched workspace

#### Scenario: Remove workspace by name
- **WHEN** user runs `nano-brain rm my-app`
- **THEN** the system searches config.yml workspace entries for one whose basename matches `my-app` and removes all data for that workspace

#### Scenario: Ambiguous name matches multiple workspaces
- **WHEN** user runs `nano-brain rm api` and multiple workspace paths have basename `api`
- **THEN** the system prints all matching workspaces with their full paths and hash prefixes and exits with a non-zero exit code without deleting anything

#### Scenario: No matching workspace found
- **WHEN** user runs `nano-brain rm nonexistent`
- **THEN** the system prints an error message indicating no workspace was found and exits with a non-zero exit code

### Requirement: Complete data removal from all workspace-scoped tables

The `rm` command SHALL delete all rows scoped to the target workspace's project_hash from every workspace-scoped table: documents, content (orphaned only), content_vectors, vectors_vec, llm_cache, file_edges, symbols, code_symbols, symbol_edges, execution_flows, and flow_steps. All deletions SHALL occur within a single database transaction.

#### Scenario: All workspace-scoped tables are cleaned
- **WHEN** a workspace with project_hash `abc123def456` has data in documents, file_edges, code_symbols, symbol_edges, execution_flows, flow_steps, symbols, content_vectors, and llm_cache
- **THEN** after `nano-brain rm abc123def456`, zero rows with project_hash `abc123def456` remain in any of those tables

#### Scenario: Shared content hashes are preserved
- **WHEN** workspace A and workspace B both reference the same content hash
- **THEN** removing workspace A does NOT delete the shared content row; it remains for workspace B

#### Scenario: Orphaned content is deleted
- **WHEN** a content hash is only referenced by the workspace being removed
- **THEN** the content row is deleted along with the workspace data

### Requirement: Config cleanup after removal

The `rm` command SHALL remove the workspace entry from `config.yml` under the `workspaces:` key after successfully deleting database records.

#### Scenario: Workspace entry removed from config
- **WHEN** `nano-brain rm /Users/me/projects/my-app` succeeds
- **THEN** the `workspaces` section in `config.yml` no longer contains the key `/Users/me/projects/my-app`

#### Scenario: Config unchanged if workspace not in config
- **WHEN** the workspace has database records but no entry in config.yml
- **THEN** the database records are still deleted and no config error occurs

### Requirement: Dry-run mode

The `rm` command SHALL support a `--dry-run` flag that prints a summary of what would be deleted without performing any deletions.

#### Scenario: Dry-run shows deletion summary
- **WHEN** user runs `nano-brain rm my-app --dry-run`
- **THEN** the system prints row counts per table (e.g., "documents: 42, code_symbols: 156, file_edges: 89") and the config entry that would be removed, without modifying any data

### Requirement: List known workspaces

The `rm` command SHALL support a `--list` flag that displays all known workspaces with their name, path, hash prefix, and document count.

#### Scenario: List all workspaces
- **WHEN** user runs `nano-brain rm --list`
- **THEN** the system prints a table of all workspaces found in config.yml and/or the database, showing name, full path, hash prefix, and document count
