## ADDED Requirements

### Requirement: Workspace detection from PWD
The MCP server SHALL compute a `projectHash` from `process.cwd()` at startup using `sha256(cwd).substring(0, 12)`. This hash SHALL be stored as `currentProjectHash` on the server context and used for all default search filtering.

#### Scenario: Server starts in a workspace directory
- **WHEN** the MCP server starts with `PWD=/Users/alice/projects/my-app`
- **THEN** `currentProjectHash` is set to the first 12 characters of `sha256("/Users/alice/projects/my-app")`
- **THEN** the hash is consistent across restarts in the same directory

#### Scenario: Hash matches harvester convention
- **WHEN** the MCP server computes `currentProjectHash` for a workspace
- **THEN** the hash matches the directory name used by the harvester for that workspace's sessions (`sessions/{projectHash}/*.md`)

### Requirement: Document-level project tagging
The `documents` table SHALL have a `project_hash TEXT` column. Every document indexed from a session file SHALL be tagged with the projectHash extracted from its file path. Non-session documents (MEMORY.md, daily logs) SHALL be tagged with `'global'`.

#### Scenario: New document indexed from session file
- **WHEN** a document is indexed from path `sessions/abc123def456/session-title.md`
- **THEN** the document's `project_hash` column is set to `abc123def456`

#### Scenario: New document indexed from non-session file
- **WHEN** a document is indexed from `MEMORY.md` or a daily log file
- **THEN** the document's `project_hash` column is set to `'global'`

#### Scenario: Document path does not match session pattern
- **WHEN** a document is indexed from a path that does not match `sessions/{hash}/*.md`
- **THEN** the document's `project_hash` column is set to `'global'`

### Requirement: Database migration for existing documents
On startup, the store SHALL add the `project_hash` column if it does not exist, then backfill existing documents by extracting the projectHash from their file paths.

#### Scenario: First startup after upgrade
- **WHEN** the store opens a database that lacks the `project_hash` column
- **THEN** the column is added via `ALTER TABLE documents ADD COLUMN project_hash TEXT DEFAULT 'global'`
- **THEN** existing documents with paths matching `sessions/{hash}/*.md` are updated with the correct projectHash
- **THEN** existing documents not matching the pattern retain `project_hash = 'global'`

#### Scenario: Subsequent startup
- **WHEN** the store opens a database that already has the `project_hash` column
- **THEN** no migration runs
- **THEN** no data is modified

### Requirement: Default search scoping to current workspace
All search operations SHALL filter results to documents matching `currentProjectHash` or `'global'` by default. This ensures searches return only results relevant to the current workspace plus cross-project notes.

#### Scenario: Search without workspace parameter
- **WHEN** `memory_search` is called with `{"query": "authentication"}` and no `workspace` parameter
- **THEN** only documents with `project_hash = currentProjectHash` or `project_hash = 'global'` are returned
- **THEN** documents from other workspaces are excluded

#### Scenario: Global documents always included
- **WHEN** a search is performed with default workspace scoping
- **THEN** MEMORY.md entries and daily logs (tagged `'global'`) are included in results
- **THEN** session documents from other workspaces are excluded

### Requirement: Cross-workspace search opt-in
All search tools SHALL accept an optional `workspace` parameter. When set to `"all"`, search results SHALL include documents from all workspaces. When set to a specific hash, results SHALL be filtered to that workspace plus `'global'`.

#### Scenario: Search with workspace="all"
- **WHEN** `memory_search` is called with `{"query": "auth", "workspace": "all"}`
- **THEN** documents from all workspaces are included in results

#### Scenario: Search with specific workspace hash
- **WHEN** `memory_search` is called with `{"query": "auth", "workspace": "abc123def456"}`
- **THEN** only documents with `project_hash = 'abc123def456'` or `project_hash = 'global'` are returned
