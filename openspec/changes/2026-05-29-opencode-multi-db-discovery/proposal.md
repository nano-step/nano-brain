Tracking: #199

## Why

OpenCode CLI's session storage layout changed. Previously a single global SQLite at `~/.local/share/opencode/opencode.db` (or legacy filesystem JSON at `~/.local/share/opencode/storage/`), it now writes one SQLite per project at `<root>/<project-slug>-<8char-hex>/opencode.db` (user-configurable root, default in the user's sandbox at `~/.ai-sandbox/opencode-dbs/`). Each per-project SQLite has its own `project` row with `worktree` = absolute project path.

Current nano-brain harvester (`internal/harvest/opencode_sqlite.go`) only accepts a single `db_path`. With the new layout this either:
1. Misses all per-project DBs (if `db_path` points to the obsolete global file), or
2. Only harvests one project at a time (if user manually points `db_path` at one DB).

We need multi-DB discovery while staying backward-compatible with the old single-`db_path` and legacy filesystem-JSON modes (each still installed in the wild on users who haven't upgraded OpenCode).

The directory suffix (`<slug>-<hex>`) appears to be OpenCode-internal (hash derived from `project.id` priority chain: remote URL → cached `.git/opencode` → root commit) and we deliberately do NOT replicate it. Instead, we open each DB and read the canonical `project.worktree` field — this is the same source of truth OpenCode itself uses and is hash-implementation-independent.

## What Changes

- **New config field** `harvester.opencode.db_root` — directory containing per-project `<slug>-<hex>/opencode.db` files (default: `~/.ai-sandbox/opencode-dbs`, override via `NANO_BRAIN_HARVESTER_OPENCODE_DB_ROOT` or `OPENCODE_DB_ROOT` env var).
- **Discovery flow at startup**: scan `db_root/*/opencode.db`; for each found DB, open read-only and read `SELECT id, worktree FROM project`; if `worktree` matches a registered nano-brain workspace path (exact `filepath.Abs` string match), instantiate one `OpenCodeSQLiteHarvester` per matched DB.
- **Skip rules**: skip rows where `worktree IN ('', '/')` (OpenCode's "global" / non-project sessions) — these never correspond to a registered workspace. Skip whole DBs that produce zero matches.
- **Resolution priority** (first non-empty wins):
  1. `db_root` (new multi-DB discovery)
  2. `db_path` (single SQLite, current behavior — kept for backward compat)
  3. `session_dir` (legacy filesystem JSON, current behavior — kept for backward compat)
- **Auto-detection**: when none of the three is configured, probe in order: `OPENCODE_DB_ROOT` env → platform default (`~/.ai-sandbox/opencode-dbs` on macOS/Linux) → `OPENCODE_DB_PATH` env → existing single-DB defaults → `OPENCODE_STORAGE_DIR` env → legacy JSON defaults.
- **Status endpoint** (`GET /api/status`): replace single `opencode.session_dir` with structured `opencode.{mode, db_root|db_path|session_dir, db_count}` so operators can verify which mode is active and how many per-project DBs were discovered.
- **Rescan**: discovery runs once at daemon startup. Newly-created per-project DBs require a daemon restart to be picked up (live tick-level rescan deferred to a follow-up — tracked in `docs/HARNESS_BACKLOG.md`).

## Capabilities

### New Capabilities
- `opencode-multi-db-discovery`: Discover and harvest from multiple per-project OpenCode SQLite databases under a single root directory, filtering to registered nano-brain workspaces.

### Modified Capabilities
- `harvester-per-project-scoping`: The OpenCode SQLite harvester is now instantiated once per discovered DB (not once total). Each instance carries its own `dbPath` but shares the workspace cache lookup pattern.

## Impact

**Files changed**:
- `internal/config/config.go` — add `DBRoot string \`koanf:"db_root"\`` to `OpenCodeHarvesterConfig`; add env var mapping `NANO_BRAIN_HARVESTER_OPENCODE_DB_ROOT` and alias `OPENCODE_DB_ROOT`.
- `internal/config/defaults.go` — set `DBRoot: ""` (auto-detect at startup, not at config load).
- `cmd/nano-brain/detect.go` — add `detectOpenCodeDBRoot()` returning `~/.ai-sandbox/opencode-dbs` (and platform alternatives) when it exists.
- `cmd/nano-brain/main.go` — replace single-instance harvester construction (lines ~364-385) with: probe `DBRoot` first → scan + filter by registered workspaces → instantiate N harvesters via `hr.AddHarvester(...)`. Fall through to single `DBPath` then `SessionDir` if `DBRoot` produces zero matches or is empty.
- `internal/harvest/opencode_sqlite.go` — no signature changes; existing per-DB harvester is reused. Add a small helper `scanOpenCodeDBRoot(root string, registered map[string]string) []discoveredDB` that opens each candidate read-only, reads `project.worktree`, and returns the matching set. Helper handles globs, missing dirs, unreadable SQLites, and non-project (`/` or empty worktree) rows.
- `internal/server/handlers/health.go` — update `harvester_status.opencode` shape to include `mode` (`"db_root" | "db_path" | "session_dir" | "disabled"`) and `db_count` (active harvester count).

**No schema changes** — workspace lookup uses existing `workspaces` table.

**No API behavior change** to `POST /api/harvest` — `Runner.RunOnce()` already fans out to all registered harvesters.

**Backward compatibility**: users with only `db_path` or `session_dir` set continue to work unchanged. New `db_root` is opt-in (auto-detected only when the platform default directory exists).

**Risk**: low. New code paths only fire when `db_root` is non-empty or auto-detected; existing single-DB and JSON-file paths are untouched. Discovery is read-only against external SQLites with `mode=ro`.
