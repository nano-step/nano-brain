# harvester-per-project-scoping Specification

## Purpose
TBD - created by archiving change fix-harvester-per-project-workspace. Update Purpose after archive.
## Requirements
### Requirement: Session-to-workspace mapping via project.worktree JOIN

The OpenCode SQLite harvester SHALL derive workspace hash per-session from `project.worktree` via LEFT JOIN on `session.project_id`. It SHALL use the pre-built workspace cache (path → hash) from registered workspaces instead of calling `storage.WorkspaceHash()` per session. It SHALL NOT auto-register unknown workspaces via `UpsertWorkspace`. Orphaned sessions (no project row or empty worktree) SHALL be excluded by the pre-filter query.

#### Scenario: Session mapped to per-project workspace

- **WHEN** `HarvestAll()` processes a session with a valid `project.worktree` that exists in the workspace cache
- **THEN** the session is stored under the cached hash for that worktree path
- **AND** `UpsertWorkspace` is NOT called
- **AND** `storage.WorkspaceHash()` is NOT called

#### Scenario: Session with unregistered worktree is never seen

- **WHEN** a session has `p.worktree = '/unregistered/path'`
- **THEN** the session is excluded by `listSessions()` SQL filter
- **AND** no messages are loaded for it
- **AND** no `UpsertWorkspace` call occurs

