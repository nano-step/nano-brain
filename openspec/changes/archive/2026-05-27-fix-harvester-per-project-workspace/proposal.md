## Why

The `OpenCodeSQLiteHarvester` computes workspace hash from the OpenCode DB file path (`~/.local/share/opencode/opencode.db`) instead of each session's `project.worktree` path. This collapses all sessions from all projects into a single workspace, violating the workspace isolation invariant (NFR-2) and the PRD definition of workspace as a project directory (§5.3, A-6, FR-18).

## What Changes

- **BREAKING**: Harvested OpenCode sessions will be stored under per-project workspace hashes (`SHA256(project.worktree)`) instead of a single DB-file workspace hash. Existing harvested data must be dropped and re-harvested.
- `OpenCodeSQLiteHarvester.HarvestAll()` joins `session → project` in OpenCode SQLite to get `project.worktree` for each session.
- Workspace hash is computed once per project (not once per DB file) and cached within the harvest cycle.
- Each project workspace is auto-registered in nano-brain PostgreSQL if it does not already exist (reusing existing `EnsureWorkspace` logic).
- Session documents are stored under their correct per-project workspace hash.
- The `workspace` parameter passed to `NewOpenCodeSQLiteHarvester()` is removed — the harvester now derives workspace dynamically from OpenCode DB content.

## Capabilities

### New Capabilities

- `harvester-per-project-scoping`: OpenCode SQLite harvester reads `project.worktree` from OpenCode DB and maps each session to its correct nano-brain workspace. One OpenCode project = one nano-brain workspace.

### Modified Capabilities

- `workspace-scoping`: Harvested sessions now satisfy the PRD workspace definition (workspace = project directory) instead of bucketing all sessions under a single installation-level hash.

## Impact

- **`internal/harvest/opencode_sqlite.go`**: `HarvestAll()`, `NewOpenCodeSQLiteHarvester()`, `listSessions()` SQL query (add project join)
- **`cmd/nano-brain/main.go`**: Remove `wsHash` pre-computation for SQLite harvester (lines 321–330); harvester derives workspace internally
- **`internal/storage/`**: No schema changes — existing `EnsureWorkspace` / `WorkspaceHash` functions are reused as-is
- **Existing harvested data**: Must be dropped (`init --force` or manual DB reset) — hashes will change
- **No API changes**, no migration needed (data is a derivable cache per NFR-4)
