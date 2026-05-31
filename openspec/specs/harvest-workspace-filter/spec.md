# harvest-workspace-filter Specification

## Purpose
TBD - created by archiving change harvest-workspace-filter. Update Purpose after archive.
## Requirements
### Requirement: Pre-filter sessions by registered workspace paths

The OpenCode SQLite harvester SHALL query PostgreSQL for registered workspace paths at the start of `HarvestAll()` and pass them to `listSessions()`. The `listSessions()` SQL query SHALL include a `WHERE p.worktree IN (...)` clause to return only sessions belonging to registered workspaces. Sessions with `p.worktree = ''` (no project) SHALL be excluded.

#### Scenario: Only registered workspace sessions returned

- **WHEN** `listSessions()` is called with registered paths `["/home/user/project-a", "/home/user/project-b"]`
- **THEN** only sessions whose `p.worktree` matches one of those paths are returned
- **AND** sessions with `p.worktree = '/home/user/unregistered'` are NOT returned
- **AND** sessions with `p.worktree = ''` are NOT returned

#### Scenario: No registered workspaces

- **WHEN** `listSessions()` is called with an empty registered paths list
- **THEN** zero sessions are returned
- **AND** a WARN log is emitted: "no registered workspaces, skipping harvest"

### Requirement: Workspace path cache built once per harvest cycle

`HarvestAll()` SHALL query `ListWorkspaces()` once and build a `map[string]string` (path → hash) from the result. This map SHALL be used both for the `listSessions()` filter and for workspace hash lookup per session, eliminating redundant `WorkspaceHash()` calls.

#### Scenario: Cache populated from PostgreSQL

- **WHEN** `HarvestAll()` starts and PostgreSQL has 3 registered workspaces
- **THEN** the workspace cache contains exactly 3 entries (path → hash)
- **AND** `ListWorkspaces()` is called exactly once during the harvest cycle

