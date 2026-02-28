## ADDED Requirements

### Requirement: ProjectHash extraction from session file path
The system SHALL provide a utility function `extractProjectHashFromPath(filePath, sessionsDir)` that extracts the projectHash from a session file's path. The function SHALL match the pattern `{sessionsDir}/{projectHash}/*.md` where `projectHash` is a 12-character hexadecimal string. For paths that do not match this pattern, the function SHALL return `undefined`.

#### Scenario: Valid session file path
- **WHEN** `extractProjectHashFromPath` is called with path `~/.nano-brain/sessions/abc123def456/2026-02-16-session.md` and sessionsDir `~/.nano-brain/sessions`
- **THEN** the function returns `'abc123def456'`

#### Scenario: Non-session file path
- **WHEN** `extractProjectHashFromPath` is called with path `~/.nano-brain/memory/2026-02-16.md` and sessionsDir `~/.nano-brain/sessions`
- **THEN** the function returns `undefined`

#### Scenario: Nested path under session directory without valid hash
- **WHEN** `extractProjectHashFromPath` is called with a path under sessionsDir where the subdirectory name is not a 12-character hex string
- **THEN** the function returns `undefined`

### Requirement: Collection-aware projectHash during watcher reindex
The watcher's `triggerReindex()` SHALL use collection-aware projectHash assignment when indexing documents. For the `sessions` collection, the projectHash SHALL be extracted from each file's path using `extractProjectHashFromPath()`. For all other collections, the watcher's own `projectHash` (current workspace) SHALL be used. If extraction returns `undefined` for a session file, the watcher's `projectHash` SHALL be used as fallback.

#### Scenario: Watcher reindexes session file from another workspace
- **WHEN** the watcher runs in workspace A (projectHash `aaa111bbb222`) and reindexes a session file at `sessions/ccc333ddd444/2026-02-16-session.md`
- **THEN** the document is indexed with `project_hash = 'ccc333ddd444'` (extracted from path)
- **THEN** the document is NOT indexed with `project_hash = 'aaa111bbb222'`

#### Scenario: Watcher reindexes non-session collection file
- **WHEN** the watcher runs in workspace A (projectHash `aaa111bbb222`) and reindexes a memory file at `memory/2026-02-16.md`
- **THEN** the document is indexed with `project_hash = 'aaa111bbb222'` (watcher's own hash)

#### Scenario: Watcher reindexes session file with unrecognized path structure
- **WHEN** the watcher reindexes a session file whose path does not match the `sessions/{hash}/*.md` pattern
- **THEN** the document is indexed with the watcher's own `projectHash` as fallback

### Requirement: Correct projectHash during init indexing
The `init` command SHALL extract projectHash from session file paths when indexing the `sessions` collection, using the same `extractProjectHashFromPath()` utility. Non-session collections SHALL use the workspace's projectHash.

#### Scenario: Init indexes session files from multiple workspaces
- **WHEN** `nano-brain init` runs in workspace A and indexes session files from `sessions/aaa111bbb222/` and `sessions/ccc333ddd444/`
- **THEN** documents from `sessions/aaa111bbb222/` are tagged with `project_hash = 'aaa111bbb222'`
- **THEN** documents from `sessions/ccc333ddd444/` are tagged with `project_hash = 'ccc333ddd444'`

### Requirement: Correct projectHash during manual update
The `memory_update` MCP tool and the `update` CLI command SHALL extract projectHash from session file paths when reindexing the `sessions` collection, using the same `extractProjectHashFromPath()` utility.

#### Scenario: memory_update reindexes session files
- **WHEN** the `memory_update` tool triggers a reindex
- **THEN** session documents are tagged with projectHash extracted from their file paths
- **THEN** non-session documents retain their existing projectHash assignment

## MODIFIED Requirements

### Requirement: Document-level project tagging
The `documents` table SHALL have a `project_hash TEXT` column. Every document indexed from a session file SHALL be tagged with the projectHash extracted from its file path by the `extractProjectHashFromPath()` utility. Non-session documents (memory files, daily logs, codebase files) SHALL be tagged with the indexer's contextual projectHash (workspace hash for codebase, `'global'` for shared files). All indexing code paths (watcher reindex, init, update, memory_update tool) SHALL use this extraction consistently.

#### Scenario: New document indexed from session file
- **WHEN** a document is indexed from path `sessions/abc123def456/session-title.md`
- **THEN** the document's `project_hash` column is set to `abc123def456`

#### Scenario: New document indexed from non-session file
- **WHEN** a document is indexed from `MEMORY.md` or a daily log file
- **THEN** the document's `project_hash` column is set to `'global'`

#### Scenario: Document path does not match session pattern
- **WHEN** a document is indexed from a path that does not match `sessions/{hash}/*.md`
- **THEN** the document's `project_hash` column is set to the indexer's contextual projectHash
