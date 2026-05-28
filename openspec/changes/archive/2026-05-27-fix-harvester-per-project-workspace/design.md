## Context

The `OpenCodeSQLiteHarvester` reads sessions from OpenCode's SQLite DB (`opencode.db`). Currently it receives a pre-computed `workspace` string (hash of the DB file path) in its constructor and applies it uniformly to every session it harvests. This is wrong: OpenCode groups sessions by `project` (each project has a `worktree` field = the directory path), and the PRD requires workspace hash = `SHA256(project directory path)`.

OpenCode SQLite schema (confirmed from live DB):
```sql
CREATE TABLE project (id TEXT PRIMARY KEY, worktree TEXT NOT NULL, ...);
CREATE TABLE session (id TEXT PRIMARY KEY, project_id TEXT NOT NULL REFERENCES project(id), ...);
```

nano-brain already has `UpsertWorkspace` in sqlc-generated code. `WorkspaceHash(path)` is a pure function: `SHA256(abs(path))`.

## Goals / Non-Goals

**Goals:**
- Each OpenCode project (`project.worktree`) maps to exactly one nano-brain workspace
- Sessions are stored under `WorkspaceHash(project.worktree)`, matching what `nano-brain init --root <worktree>` would produce
- Workspace is auto-registered in PostgreSQL on first harvest (no user action needed)
- `NewOpenCodeSQLiteHarvester` no longer accepts a pre-computed workspace string

**Non-Goals:**
- Migrating existing harvested data (drop and re-harvest)
- Changing the session dir harvester (`OpenCodeHarvester`) — it already uses `session_dir` as workspace root which is correct for that mode
- Changing the Claude Code harvester

## Decisions

### Decision 1: Harvester derives workspace per-project, not pre-computed in main.go

**Choice:** Move workspace hash computation inside `HarvestAll()`, joining `session → project` in the OpenCode SQLite query.

**Rationale:** The DB file path is a deployment detail, not a semantic identity. The project worktree path is the correct identity per PRD. The harvester is the only component that knows the OpenCode DB content.

**Alternative considered:** Keep workspace pre-computed in `main.go`, pass a map of `project_id → workspace_hash`. Rejected: requires two-pass query (first list projects, then harvest), and tightly couples main.go to OpenCode internals.

### Decision 2: Extend `listSessions` SQL to join `project`

**Choice:** Modify `listSessions` query to `JOIN project ON session.project_id = project.id` and return `project.worktree` alongside each session.

```sql
SELECT s.id, COALESCE(s.title, ''), COALESCE(s.time_created, 0), p.worktree
FROM session s
JOIN project p ON s.project_id = p.id
ORDER BY s.time_created DESC
```

**Rationale:** Single query, minimal change to existing code structure.

### Decision 3: Cache workspace hash per worktree within one harvest cycle

**Choice:** Use a `map[string]string` (`worktree → wsHash`) local to `HarvestAll()`, populated lazily as new worktrees are encountered.

**Rationale:** Avoids redundant `UpsertWorkspace` calls for projects with many sessions. Workspace registration is idempotent anyway (upsert), but caching reduces noise.

### Decision 4: Auto-register workspace via `UpsertWorkspace`

**Choice:** Call `q.UpsertWorkspace(ctx, UpsertWorkspaceParams{Hash: wsHash, Root: worktree})` when a new worktree is first seen.

**Rationale:** Reuses existing sqlc-generated code. User does not need to run `nano-brain init --root` for sessions to be harvestable — harvesting is automatic.

### Decision 5: Remove `workspace` parameter from constructor

**Choice:** `NewOpenCodeSQLiteHarvester(pgDB, logger, dbPath)` — no workspace string.

**Rationale:** The constructor parameter was only used to pass the wrong value. Removing it prevents future misuse.

## Risks / Trade-offs

- **[Risk] Sessions with missing project row** → If `project` row is deleted but session still exists, the JOIN returns no row. Mitigation: use `LEFT JOIN`, fallback workspace = `WorkspaceHash(dbPath)` with a warning log. This is an edge case (cascade delete should prevent it, but OpenCode DB is partially corrupt in dev).
- **[Risk] `project.worktree` is a host path** → On the host, `WorkspaceHash("/Users/tamlh/...")` is stable. If the DB is ever accessed from a different machine with a different path, hashes won't match. Acceptable: same constraint applies to `nano-brain init --root`.
- **[Trade-off] Existing data must be dropped** → Data is a derivable cache (NFR-4). User confirmed this is acceptable.

## Migration Plan

1. Run `nano-brain init --force` (or `POST /api/v1/reset-workspace`) to drop existing documents
2. Deploy new binary
3. Harvester re-harvests all sessions under correct per-project workspace hashes
4. Users re-run `nano-brain init --root <project>` for any workspace they want to query (or workspaces are auto-registered by harvester)

No rollback needed — the old behavior was broken. Forward-only.
