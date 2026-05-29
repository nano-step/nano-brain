Tracking: #194

## Why

OpenCode SQLite harvester scans ALL sessions then skips unregistered workspaces at persist time. This wastes I/O — each unneeded session triggers a SQLite message query, markdown render, and PostgreSQL lookup before being discarded. As session count grows, this becomes increasingly wasteful.

## What Changes

- Filter `listSessions()` SQL query to only return sessions whose `project.worktree` matches a registered workspace path in nano-brain PostgreSQL
- Sessions without a project (`worktree = ''`) retain existing fallback behavior
- Auto-register of unknown workspaces via `UpsertWorkspace` is removed — only pre-registered workspaces are harvested

## Capabilities

### New Capabilities
- `harvest-workspace-filter`: Pre-filter OpenCode sessions by registered workspace paths at scan time, before message loading and embedding

### Modified Capabilities
- `harvester-per-project-scoping`: Session-to-workspace mapping now requires workspace to be pre-registered; unknown workspaces are skipped instead of auto-registered

## Impact

- `internal/harvest/opencode_sqlite.go` — `listSessions()` query gains WHERE clause; `HarvestAll()` removes auto-register logic
- No API changes — harvest endpoint behavior unchanged externally
- No schema changes — uses existing `workspaces` table for lookup
- Sessions from unregistered workspaces will no longer appear in search results (intentional)
