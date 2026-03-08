## ADDED Requirements

### Requirement: Background embedding iterates all configured workspaces
The embed interval SHALL iterate all workspaces listed in `config.yml` → `workspaces` map where `codebase.enabled` is true. For each workspace, it SHALL open that workspace's SQLite DB, call `embedPendingCodebase()`, then close the DB.

#### Scenario: Multiple workspaces with pending embeddings
- **WHEN** the embed interval fires and config.yml has 3 workspaces with `codebase.enabled: true`
- **AND** workspace A has 100 pending docs, workspace B has 50 pending, workspace C has 0 pending
- **THEN** the interval processes up to 50 docs from workspace A
- **THEN** the interval processes up to 50 docs from workspace B
- **THEN** the interval skips workspace C (nothing pending)
- **THEN** the interval completes and schedules the next cycle

#### Scenario: Workspace with codebase disabled
- **WHEN** the embed interval fires and a workspace has `codebase.enabled: false` or no codebase config
- **THEN** that workspace is skipped entirely

#### Scenario: Workspace DB does not exist yet
- **WHEN** the embed interval encounters a workspace whose SQLite DB file does not exist
- **THEN** that workspace is skipped (no error, no DB creation)

#### Scenario: Startup workspace is also in config
- **WHEN** the server starts in workspace A and workspace A is also in the config.yml workspaces map
- **THEN** workspace A's embeddings are processed using the already-open primary store (no duplicate open)

### Requirement: Store factory for workspace DB access
The system SHALL provide a helper function to open a workspace's SQLite DB given a workspace path. The helper SHALL compute the DB filename using the same convention as `startServer()`: `{dirName}-{hash}.sqlite` in the data directory.

#### Scenario: Open store for a configured workspace
- **WHEN** `openWorkspaceStore("/Users/alice/projects/my-app")` is called
- **THEN** it opens `~/.nano-brain/data/my-app-{hash}.sqlite`
- **THEN** it returns a usable Store instance

#### Scenario: Caller closes store after use
- **WHEN** the embed interval finishes processing a workspace
- **THEN** it calls `store.close()` on the temporary store
- **THEN** the SQLite file handle is released

### Requirement: Session harvesting across all workspaces
The harvest interval SHALL write harvested session documents to the correct per-workspace DB based on the session's project hash, not just the startup workspace's DB.

#### Scenario: Sessions from multiple workspaces
- **WHEN** the harvest interval runs and finds sessions for workspace A and workspace B
- **THEN** workspace A's sessions are written to workspace A's DB
- **THEN** workspace B's sessions are written to workspace B's DB

### Requirement: Embed cycle timing
The embed interval timer SHALL start AFTER the previous cycle completes (including all workspaces), not on a fixed interval. This prevents overlapping cycles when processing is slow.

#### Scenario: Slow embed cycle
- **WHEN** an embed cycle takes 5 minutes to process all workspaces
- **AND** the embed interval is configured to 60 seconds
- **THEN** the next cycle starts 60 seconds after the previous cycle finishes (at t=6min)
- **THEN** cycles never overlap
