## Context

`OpenCodeSQLiteHarvester.HarvestAll()` queries ALL sessions from OpenCode's SQLite DB, then per-session: derives workspace hash, loads messages, renders markdown, checks PostgreSQL for existing document. Sessions belonging to unregistered workspaces waste all that I/O before being effectively discarded (auto-registered into a workspace nobody queries).

The `workspaces` table in PostgreSQL already tracks registered workspaces (path + hash). This data can be passed to the SQLite query to pre-filter.

## Goals / Non-Goals

**Goals:**
- Pre-filter sessions in `listSessions()` so only registered-workspace sessions are returned
- Eliminate per-session I/O for unregistered workspaces
- Preserve fallback behavior for orphaned sessions (no project row)

**Non-Goals:**
- Changing Claude Code harvester (no per-session workspace data available)
- Adding config for workspace whitelist/blacklist (use existing registered workspaces)
- Changing the `init` / workspace registration flow

## Decisions

### 1. Pass registered workspace paths into `listSessions()`

**Choice:** Query PostgreSQL for registered workspace paths, pass as `[]string` arg to `listSessions()`, add `WHERE p.worktree IN (?, ...)` clause.

**Alternative:** Filter in Go after query returns. Rejected — defeats the purpose of reducing SQLite I/O.

**Alternative:** JOIN against PostgreSQL from SQLite. Not possible — separate databases.

### 2. Orphaned sessions (no project) — skip, don't fallback

**Choice:** Sessions with `p.worktree = ''` are skipped. Previously they fell back to `WorkspaceHash(dbPath)` creating a catch-all workspace.

**Rationale:** The catch-all workspace pollutes search results. If a workspace isn't registered, its sessions shouldn't be in memory. Users who want those sessions can register the workspace via `init`.

**Trade-off:** Existing fallback sessions already in PostgreSQL remain. This change only affects future harvests.

### 3. Remove auto-register of unknown workspaces

**Choice:** Remove the `UpsertWorkspace` call for newly-discovered worktrees in `HarvestAll()`.

**Rationale:** Auto-register contradicts the filter intent. If we filter by registered workspaces but then auto-register every new one, the filter is pointless after first run.

### 4. Fetch registered paths once per harvest cycle

**Choice:** Query `SELECT path FROM workspaces` once at the start of `HarvestAll()`, cache as `map[string]string` (path → hash).

**Rationale:** Workspace list changes infrequently. One query per cycle is sufficient.

## Risks / Trade-offs

- **Existing auto-registered workspaces** → Sessions previously harvested via auto-register remain in PostgreSQL. No cleanup needed — they're valid data, just won't get new sessions added. → Acceptable, no migration needed.

- **User expectation shift** → Users must `init` a workspace before its sessions are harvested. Previously it was automatic. → Document in changelog. The `init` command already exists and is the standard onboarding path.

- **Empty workspace list** → If no workspaces registered, harvest returns 0 sessions. → Log a WARN message so users know why nothing was harvested.
