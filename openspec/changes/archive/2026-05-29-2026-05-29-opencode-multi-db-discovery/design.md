## Context

OpenCode CLI moved from one global SQLite (or filesystem JSON) to **one SQLite per project** under a configurable root. nano-brain's current harvester accepts a single `db_path` — it cannot follow this new layout without per-project enumeration.

## Goals

- Harvest sessions from every per-project OpenCode SQLite that corresponds to a nano-brain-registered workspace.
- Stay deterministic — no guessing of OpenCode's internal hash. We resolve project→worktree at runtime by opening each DB.
- Backward-compatible: single `db_path` and legacy `session_dir` modes continue working unchanged.
- Discovery is read-only and cheap (one `SELECT` per DB at startup, then per-DB harvesters use the same `?mode=ro` pattern already in production).

## Non-goals

- Do **not** replicate OpenCode's `Hash.fast` directory-naming algorithm. The directory suffix is opaque metadata; the canonical join key is `project.worktree` inside the DB.
- Do **not** support live filesystem watching for new per-project DBs in v1. Startup-only discovery is enough — daemon restart picks up new projects. Live rescan deferred to backlog.
- Do **not** support multi-root scan in v1 (no `db_roots []string`). One `db_root` suffices for current users.

## Decisions

### Decision 1: Match by `project.worktree`, not directory suffix

**What**: Open every `<db_root>/*/opencode.db`, read `SELECT worktree FROM project LIMIT 1`, compare against registered workspace paths.

**Why**: Empirical investigation showed the 8-char suffix in directory names (e.g. `nano-brain-ab295520`) does NOT correspond to MD5/SHA1/SHA256/xxhash32/xxhash64/xxh3 of any input we tested (worktree, project_id, name, project_id prefix). OpenCode's `Hash.fast` is internal and could change. The `project.worktree` field is the **stable, source-of-truth** join key — it's the same value OpenCode itself uses to scope sessions.

**Alternatives rejected**:
- Replicating Hash.fast → brittle (upstream change breaks us silently).
- Trusting directory basename prefix (`nano-brain-*`) → ambiguous when two projects share a basename (`foo` in `/a/foo` and `/b/foo`).

**Cost**: one SQLite open + 1-row query per candidate DB at startup. With ~10 DBs (current user state) this is <50ms. Even 1000 DBs would stay under 5s.

### Decision 2: Three-mode priority chain, not auto-merge

**What**: At startup, evaluate in order: `db_root` (multi) → `db_path` (single) → `session_dir` (legacy). First non-empty mode that produces ≥1 harvester wins. Subsequent modes are skipped entirely.

**Why**:
- Users with existing single-DB or legacy-JSON configs see no behavior change.
- Avoids the "is this harvester running twice?" debugging nightmare from running multiple modes simultaneously.
- Explicit: `harvester_status.mode` tells operators which path is active.

**Alternative rejected**: union of all configured modes — too easy to double-ingest the same session via two paths after a partial migration.

**Fallback rule**: if `db_root` is set but produces zero matched DBs (empty dir, all unregistered worktrees), log Info and **fall through** to `db_path`. This avoids silent total disable when a user migrates `db_root` config but hasn't registered any workspaces yet.

### Decision 3: Skip rules — `worktree IN ('', '/')`

**What**: Treat sessions whose project has empty or root-path worktree as out-of-scope.

**Why**: Empirical inspection of the user's `~/.ai-sandbox/opencode-dbs/` shows three DBs (`lgc-*`, `tools-*`, `express-app-*`) have `project.id='global', worktree='/', name='global'` containing 9, 174, and 1544 sessions respectively. These are OpenCode's "no project" sessions (started outside any git repo). They cannot match any registered nano-brain workspace by definition (no nano-brain user registers `/`) and harvesting them would either fail the workspace-hash lookup or produce a synthetic workspace with no useful path.

**Alternative considered**: index them under a synthetic `global` workspace. Rejected because:
1. The user's clarification answer leans on OpenCode's existing per-project scoping ("opencode chỉ thấy session của project đó thôi"), implying global sessions are intentionally not user-facing per-project.
2. It opens a separate UX question (how do users search across global sessions) better deferred until requested.

### Decision 4: Per-tick discovery deferred

**What**: Discovery runs once at startup; new per-project DBs created during the daemon's lifetime are NOT picked up until restart.

**Why**: v1 simplicity. The harvest loop currently shares `Runner.harvesters` slice mutated only at startup — making it tick-mutable requires lock review and per-tick `ListWorkspaces + Glob` overhead. The win (auto-discover new project after `opencode` opens it once) is real but small; we can ship v1 fast and iterate.

**Mitigation**: clear log Info at startup showing N discovered DBs; clear status endpoint field `db_count` so operators can detect mismatch and restart.

## Risks / Tradeoffs

| Risk | Mitigation |
|------|------------|
| OpenCode upstream schema drift (renames `worktree`) | `scanOpenCodeDBRoot` catches query error per DB, logs Debug with reason, skips that DB; daemon stays up. Add unit test fixture with malformed schema. |
| Slow startup with many DBs | 2s per-DB ping timeout. Worst case: 1000 corrupt DBs = 2000s. Real case: tens of DBs, <1s total. Add a global 30s discovery timeout if needed in v2. |
| Stale per-project DB references in `Runner` after the underlying SQLite file is deleted | Harvester already handles `sql.Open` failure per tick — logs Error and continues. No crash. |
| Path normalization mismatch (trailing slash, symlinks) | Use `filepath.Clean` and `filepath.Abs` on both the workspace path (already done at registration) and the SQLite `worktree` value at compare time. Test fixture covers trailing-slash case. |
| User runs nano-brain in container, `db_root` on host | Out of scope. Document in AGENTS.md that `db_root` must be readable from the daemon's process namespace. |

## Migration

No data migration. Pure additive config field + new code paths.

Existing users:
- Single-DB config (`db_path` set): no action required, works as before.
- Legacy JSON config (`session_dir` set): no action required, works as before. (Recommend migration to OpenCode v2's SQLite output but that's an OpenCode CLI decision, not ours.)
- New users on the user's wrapper layout: `OPENCODE_DB_ROOT=~/.ai-sandbox/opencode-dbs` env var (or set in `config.yml`), or rely on auto-detection if that exact path exists.

## Open Questions

1. Should `db_root` support multiple roots (`[]string`)? **Defer** — no user has asked. YAML-friendly schema is `db_root` (string); upgrading to `db_roots` later is a non-breaking additive change.
2. Should we deprecate `session_dir` (legacy JSON) with a warning log? **Defer** — orthogonal cleanup, low value, separate proposal if ever.
3. Should `POST /api/harvest` rescan `db_root` before running? **Lean no** — operators wanting fresh discovery should restart daemon; mixing rescan into the manual-trigger path muddies semantics. Revisit if requested.
