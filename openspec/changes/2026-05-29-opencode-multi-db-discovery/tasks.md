## 1. Config schema — add `db_root` field

- [ ] 1.1 Add `DBRoot string \`koanf:"db_root"\`` field to `OpenCodeHarvesterConfig` in `internal/config/config.go` (alongside existing `SessionDir`, `DBPath`)
- [ ] 1.2 Add env var alias mapping `OPENCODE_DB_ROOT` → `harvester.opencode.db_root` next to existing `OPENCODE_STORAGE_DIR` / `OPENCODE_DB_PATH` aliases (~config.go:191)
- [ ] 1.3 Document in code comment: `db_root` takes priority over `db_path` which takes priority over `session_dir`
- [ ] 1.4 Verify: `go build ./...` clean; `go test -race -short ./internal/config/...` passes

## 2. Auto-detection — `detectOpenCodeDBRoot`

- [ ] 2.1 Add `detectOpenCodeDBRoot()` in `cmd/nano-brain/detect.go` mirroring `detectOpenCodeDBPath` pattern: check `OPENCODE_DB_ROOT` env, then `platformOpenCodeDBRootPaths()`
- [ ] 2.2 Add `platformOpenCodeDBRootPaths()` returning: `~/.ai-sandbox/opencode-dbs` on linux+darwin (only path for now — windows variant deferred until a user requests it)
- [ ] 2.3 Return path only if it exists AND is a directory (`os.Stat` + `IsDir()`)
- [ ] 2.4 Unit tests in `detect_test.go`: env override priority, platform default exists, platform default missing → empty, file (not dir) at path → empty
- [ ] 2.5 Verify: `go test -race -short ./cmd/nano-brain/...` passes

## 3. Discovery helper — `scanOpenCodeDBRoot`

- [ ] 3.1 Add `scanOpenCodeDBRoot(ctx context.Context, root string, registered map[string]string, logger zerolog.Logger) []DiscoveredDB` in `internal/harvest/opencode_sqlite.go`
- [ ] 3.2 Define `type DiscoveredDB struct { DBPath, Worktree, WorkspaceHash string }`
- [ ] 3.3 Use `filepath.Glob(filepath.Join(root, "*", "opencode.db"))` for candidates; log Debug when zero candidates
- [ ] 3.4 For each candidate: open with `sql.Open("sqlite", path+"?mode=ro")`, `PingContext` with 2s timeout, query `SELECT id, worktree FROM project LIMIT 1`. Close immediately after.
- [ ] 3.5 Skip when query fails (corrupt/schema-drift), `worktree` is empty, or `worktree == "/"` — log Debug with `reason` field, never Error (external state).
- [ ] 3.6 Match: `hash, ok := registered[worktree]` — only include in output when match. Always normalize candidate path via `filepath.Clean` before map lookup.
- [ ] 3.6b **Bonus fix (Oracle M3)**: In existing `OpenCodeSQLiteHarvester.HarvestAll` (`internal/harvest/opencode_sqlite.go` ~line 144), normalize `worktree := filepath.Clean(sess.Worktree)` before the `wsCache[worktree]` lookup. Same one-line fix benefits all three modes (db_root, db_path, session_dir). Add a unit-test case: `project.worktree="/u/foo/"` (trailing slash) matches workspace registered as `/u/foo`.
- [ ] 3.7 Unit tests with `t.TempDir()` fixtures: registered match, unregistered worktree skipped, `/` worktree skipped, empty worktree skipped, unreadable file skipped, missing `project` table skipped, zero candidates returns nil.
- [ ] 3.8 Verify: `go test -race -short ./internal/harvest/...` passes

## 4. Startup wiring — multi-instance registration

- [ ] 4.1 In `cmd/nano-brain/main.go`, after the existing auto-detect blocks (~lines 340-352) add a `DBRoot` auto-detect block (only runs when `cfg.Harvester.OpenCode.DBRoot == ""`).
- [ ] 4.2 Refactor the harvester-instantiation block (~lines 364-385) into a function `buildOpenCodeHarvesters(cfg, db, logger) []harvest.Harvester` returning a slice (possibly empty).
- [ ] 4.3 Inside the new function, branch in priority order:
  - If `DBRoot != ""`: call `storage.ListWorkspaces`, build `path→hash` map, call `scanOpenCodeDBRoot`. For each discovered DB, instantiate `NewOpenCodeSQLiteHarvester(db, logger, discovered.DBPath)`. If zero matches: log Info "no per-project DBs matched registered workspaces" and fall through to next branch.
  - Else if `DBPath != ""`: single-instance current behavior unchanged.
  - Else if `SessionDir != ""`: legacy JSON harvester current behavior unchanged.
  - Else: return empty slice (log "opencode harvester disabled").
- [ ] 4.4 Create `Runner` from the first harvester (if any), then `AddHarvester` for the rest. Match existing wiring (summarizer propagation, runner.Run in errgroup).
- [ ] 4.5 Log Info per matched DB at startup: `db_path`, `worktree`, `workspace_hash`.
- [ ] 4.5b **Log level discrimination** (Oracle minor): when `db_root` is set EXPLICITLY by config/env and produces zero matches, log `Warn` ("db_root configured but no per-project DBs matched registered workspaces — falling through"). When `db_root` was AUTO-DETECTED and produces zero matches, log `Info` (less noisy for users without the wrapper).
- [ ] 4.6 Verify: `CGO_ENABLED=0 go build ./...` clean. Manual smoke: start daemon pointing `DBRoot` at `/Users/tamlh/.ai-sandbox/opencode-dbs`, observe N harvesters registered.

## 5. Per-tick rescan for `db_root` mode

- [ ] 5.1 Decision: keep startup-only discovery for v1 (simpler, low risk). New per-project DBs require daemon restart.
- [ ] 5.2 Add follow-up issue in `docs/HARNESS_BACKLOG.md` for "live rescan on tick" — defer until a user reports needing it.
- [ ] 5.3 (No code change in this task; just document the deferral inline in `buildOpenCodeHarvesters` with a `// TODO: live rescan — see HARNESS_BACKLOG.md` comment.)

## 6. Status endpoint update

- [ ] 6.1 Add fields to the `harvester_status.opencode` struct in `internal/server/handlers/health.go`: `Mode string` (`"db_root" | "db_path" | "session_dir" | "disabled"`), `DBCount int`.
- [ ] 6.2 Keep existing `Enabled bool` and `SessionDir string` fields for backward compat; add `DBRoot string` and `DBPath string`.
- [ ] 6.3 `Enabled = Mode != "disabled"`. `Mode` derived from same priority chain used at startup.
- [ ] 6.4 **Inject the Runner reference into the Health handler at server construction time** (Oracle M2). Mirror how `queue` is already injected. Add a `harvester` slot on Health struct holding either the `*harvest.Runner` (preferred) or a snapshot struct `{Mode string, OpenCodeDBCount int, DBRoot, DBPath, SessionDir string}` captured at startup. In `cmd/nano-brain/main.go`, after building the harvester runner, compute the mode + count once and pass to `srv.SetHealth(...)` (or via the existing `SetHarvestRunner` if it can flow through).
- [ ] 6.5 Add `Runner.HarvesterCount() int` returning `len(r.harvesters)` — used by the handler to expose live count without exposing internals.
- [ ] 6.6 Update `internal/server/handlers/health_test.go` (or equivalent) to cover all four modes: `db_root` (with N>0), `db_path`, `session_dir`, `disabled`.
- [ ] 6.7 Verify: `go test -race -short ./internal/server/...` passes.

## 7. Integration test

- [ ] 7.1 Add `internal/harvest/opencode_multi_db_integration_test.go` with `//go:build integration` build tag.
- [ ] 7.2 Setup: real PG via `testutil.SetupTestDB`; two `t.TempDir()` SQLite DBs (DB-A with worktree `/tmp/proj-a`, DB-B with worktree `/tmp/proj-b`, DB-C with worktree `/` to verify skip).
- [ ] 7.3 Register `/tmp/proj-a` only in PG. Call `scanOpenCodeDBRoot` + run discovered harvesters.
- [ ] 7.4 Assert: only DB-A sessions land in PG; DB-B sessions absent; DB-C sessions absent; logs include skip reason for B and C.
- [ ] 7.5 Verify: `go test -race -tags=integration ./internal/harvest/...` passes (skip if PG unavailable — match existing pattern).

## 8. Documentation

- [ ] 8.1 Update `internal/harvest/AGENTS.md` — add a "Multi-DB discovery" subsection explaining the three-mode priority and `db_root` semantics.
- [ ] 8.2 Update `README.md` (or `docs/` if present) — config example showing `harvester.opencode.db_root`.
- [ ] 8.3 No new env vars in `OPENSPEC` config section beyond what's added — single line in `AGENTS.md` env var table.

## 9. Validation ladder

- [ ] 9.1 `validate:quick` — `CGO_ENABLED=0 go build ./... && go test -race -short ./...`
- [ ] 9.2 `self-review:response-shape` — verify health endpoint JSON shape matches struct, no missing field assignments.
- [ ] 9.3 `self-review:staged-files` — `rtk git status` shows only changed Go files + this proposal; no `.opencode/`, no lockfiles.
- [ ] 9.4 `test:integration` — `go test -race -tags=integration ./internal/harvest/...`
- [ ] 9.5 `smoke:e2e` — Build binary, start daemon with `OPENCODE_DB_ROOT=/Users/tamlh/.ai-sandbox/opencode-dbs`, curl `/api/status`, verify `harvester_status.opencode.mode == "db_root"` and `db_count > 0`. Trigger `POST /api/harvest`, verify response shows harvested > 0 for at least one registered workspace.
- [ ] 9.6 LSP diagnostics clean on all changed files.

## 10. Harness classification

- [ ] 10.1 Risk flags: workspace-data-flow (1), external-provider (OpenCode SQLite — 1). Total: 2 → **normal lane**.
- [ ] 10.2 Change type: `user-feature` (operators see new config + new status fields). Review gate required.
- [ ] 10.3 Create tracking GitHub issue before implementation: `gh issue create --repo nano-step/nano-brain --title "feat(harvest): multi-DB OpenCode discovery via db_root" --label "lane:normal,change-type:user-feature,area:harvest"`.
