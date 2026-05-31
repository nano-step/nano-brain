# Tasks: Restore Summary Disk Persistence

Tracking: #258
Branch: `feat/258-summary-disk-persistence`

## Phase A — Foundations

- [x] **A1** Verify baseline tests green:
  ```bash
  go test -race -short ./internal/summarize/... ./internal/config/...
  ```

- [x] **A2** Read existing code surface:
  - `internal/summarize/persist.go` — current `Persister.Save()` (DB-only, ~180 lines)
  - `internal/config/config.go` — `SummarizationConfig` struct (current fields)
  - `cmd/nano-brain/main.go` — `buildHarvestSummarizer()` wiring
  - `cmd/nano-brain/cmd_cleanup_orphan_workspaces.go` — CLI command reference pattern

## Phase B — Slugify helper (pure function, no I/O)

- [x] **B1** Create `internal/summarize/slug.go`:
  - `slugify(title string) string` — lowercase + non-alphanum → dashes + collapse + trim + max 80 chars + empty → "untitled-session"
  - Constants `maxSlugLen = 80`, `defaultSlug = "untitled-session"`

- [x] **B2** Create `internal/summarize/slug_test.go`:
  - Table-driven test with 15+ cases:
    - "Oracle Verify Epic 9" → "oracle-verify-epic-9"
    - "Foo!!! @bar ###" → "foo-bar"
    - "" → "untitled-session"
    - "Chào thế giới" → "chao-the-gioi" (vietnamese diacritics stripped or preserved? — verify with test)
    - "a" repeated 200 times → truncated to 80 chars
    - All special chars → "untitled-session" (no alphanum)
    - Multi-dash collapse: "a---b" → "a-b"
    - Leading/trailing dash: "-foo-" → "foo"

- [x] **B3** Run: `go test -race -short ./internal/summarize/...`
  Commit: `feat(summary): add slugify helper for disk-friendly filenames (#258)`

## Phase C — Path helper (depends on slug)

- [x] **C1** Create `internal/summarize/diskpath.go`:
  - `buildDiskPath(outputDir, workspaceName, source, title, sessionID string, date time.Time) string` — produces full path
  - `expandTilde(path string) (string, error)` — handles `~/...`
  - `workspaceNameOrFallback(name, hash string) string` — name if non-empty, else `ws-<hash[:12]>`

- [x] **C2** Create `internal/summarize/diskpath_test.go`:
  - Tilde expansion (with $HOME)
  - Path structure: `<dir>/<workspace>/<source>_<slug>_<date>.md`
  - Workspace fallback when name empty
  - Path with workspace having special chars (defensive — slug workspace too)

- [x] **C3** Run: `go test -race -short ./internal/summarize/...`
  Commit: `feat(summary): add disk path generation helper (#258)`

## Phase D — Atomic write helper

- [x] **D1** Create `internal/summarize/diskwrite.go`:
  - `writeFileAtomic(path string, content []byte) error` — write to `path + ".tmp"` then `os.Rename`
  - `ensureDir(path string) error` — `os.MkdirAll` for parent directory
  - `resolveCollision(path string, content []byte, sessionID string) (string, error)` — if path exists with DIFFERENT content, return `path` with `_<sha8>` suffix before `.md`

- [x] **D2** Create `internal/summarize/diskwrite_test.go`:
  - Happy path: write `Hello\n` → file exists, content matches
  - Mid-write crash simulation: `.tmp` file alone left after `os.Rename` simulated failure → cleanup expected
  - Directory created via MkdirAll
  - Permission denied → error returned (not panic)
  - Collision: same path different content → new path with `_<sha8>` suffix
  - Collision: same path same content → original path (overwrite OK)

- [x] **D3** Run: `go test -race -short ./internal/summarize/...`
  Commit: `feat(summary): add atomic file write with collision handling (#258)`

## Phase E — Config: WriteToDisk + tilde expand

- [x] **E1** Modify `internal/config/config.go` `SummarizationConfig` struct:
  - Add field `WriteToDisk *bool `yaml:"write_to_disk"`` (pointer for explicit unset detection)
  - Ensure `OutputDir string `yaml:"output_dir"`` field exists (resurrect from silent-ignore state)
  - Add default value: `WriteToDisk: ptr(true)`, `OutputDir: "~/.nano-brain/summaries"`

- [x] **E2** Modify config Load: tilde-expand `OutputDir` at load time. Test in `internal/config/config_test.go`.

- [x] **E3** Remove the existing `TestSummarizationConfig_OutputDirIgnored` test (or invert it to `TestSummarizationConfig_OutputDirHonored`).

- [x] **E4** Update README and `examples/config.yml` to document `write_to_disk` default and `output_dir`.

- [x] **E5** Run: `go test -race -short ./internal/config/...`
  Commit: `feat(config): add write_to_disk flag and tilde-expand output_dir (#258)`

## Phase F — Wire into Persister

- [x] **F1** Modify `internal/summarize/persist.go`:
  - Add fields to `Persister` struct: `writeToDisk bool`, `outputDir string`
  - Update `NewPersister` signature: `NewPersister(db *sql.DB, enqueuer PersisterEnqueuer, writeToDisk bool, outputDir string, logger zerolog.Logger) *Persister`
  - In `Save()`: AFTER `tx.Commit()` succeeds, if `p.writeToDisk == true`:
    - Look up workspace name via `q.GetWorkspaceByHash(meta.WorkspaceHash)`
    - Build path via `buildDiskPath`
    - `ensureDir(filepath.Dir(path))`
    - `resolveCollision(path, content, meta.SessionID)`
    - `writeFileAtomic(path, []byte(summaryMarkdown))`
    - Log INFO: `"summary written to disk"` with path
    - On any error: log WARN, do NOT return error (DB persist already succeeded)

- [x] **F2** Update `cmd/nano-brain/main.go` `buildHarvestSummarizer()`:
  - Read `cfg.Summarization.WriteToDisk` (default true if nil)
  - Read `cfg.Summarization.OutputDir`
  - Pass to `NewPersister`

- [x] **F3** Add integration test `internal/summarize/persist_disk_test.go`:
  - Test: WriteToDisk=true → DB doc + file both exist, content matches
  - Test: WriteToDisk=false → DB doc exists, NO file
  - Test: WriteToDisk=true + read-only output_dir → DB succeeds, file fails, WARN logged, no panic
  - Test: Re-save same session → same file, overwrite OK (idempotent)
  - Test: 2 sessions same title same date → first writes plain, second gets `_<sha8>` suffix

- [x] **F4** Run: `go test -race ./internal/summarize/... ./cmd/nano-brain/...`
  Commit: `feat(summary): wire disk persistence into Persister (#258)`

## Phase G — Backfill CLI

- [x] **G1** Create `cmd/nano-brain/cmd_backfill_summaries.go`:
  - Plain `func runBackfillSummariesCmd(args []string) error` (no Cobra — match `cmd_cleanup_orphan_workspaces.go` pattern)
  - Flags: `--output-dir` (override config), `--workspace` (name or hash, optional), `--since` (RFC3339 date, optional), `--dry-run`
  - Pre-flight: HTTP HEAD `http://localhost:3100/health` → WARN if server running but do not abort
  - Query DB: `SELECT id, workspace_hash, title, content, created_at, metadata FROM documents WHERE collection='session-summary' [+filters]`
  - For each row: extract `session_id` from `metadata` JSONB, look up workspace name, build path, `ensureDir`, `resolveCollision`, `writeFileAtomic`
  - Report: `Found N summaries. Written M files (K skipped — already exist with identical content, S overwritten, F failed).`
  - `--dry-run`: list paths that WOULD be written, no actual writes

- [x] **G2** Register command in `cmd/nano-brain/main.go` switch dispatcher.

- [x] **G3** Create `cmd/nano-brain/cmd_backfill_summaries_test.go`:
  - Setup: schema-per-test PG + insert 5 dummy summary rows
  - Test: full backfill → 5 files created, exit 0
  - Test: `--dry-run` → 0 files, report counts
  - Test: re-run backfill → idempotent (0 new writes, all 5 marked as "already exists")
  - Test: `--workspace=X` filter → only X's summaries exported
  - Test: `--since=date` filter → only newer summaries

- [x] **G4** Update README CLI table with new command.

- [x] **G5** Run: `go test -race ./cmd/nano-brain/...`
  Commit: `feat(cli): add backfill-summaries command for one-shot export (#258)`

## Phase H — End-to-end verification on live server

- [x] **H1** On live `host.docker.internal:3100` server (operator must stop + rebuild + restart):
  - Build binary
  - Stop server
  - Apply branch (operator action)
  - Restart server
  - Watch for new file in `~/.nano-brain/summaries/<workspace>/`

- [x] **H2** Backfill existing 167 summaries:
  ```bash
  nano-brain backfill-summaries --dry-run > /tmp/backfill-preview.txt
  head -50 /tmp/backfill-preview.txt
  nano-brain backfill-summaries
  ls -la ~/.nano-brain/summaries/nano-brain/ | head -10
  ```

- [x] **H3** Verify file content matches DB content:
  - Pick any file
  - Get same doc via API `/api/v1/get`
  - `diff <(cat file.md) <(curl ... | jq -r .content)` → should be empty

- [x] **H4** Open one file in Obsidian (or `cat`) — confirm readable markdown.

- [x] **H5** Document evidence at `docs/evidence/restore-summary-disk-persistence/`:
  - `g1-default-on-new-summary.txt` — log + ls showing auto-write
  - `g2-backfill-output.txt` — backfill dry-run output
  - `g3-content-match.txt` — diff result
  - `g4-opt-out.txt` — `write_to_disk: false` test result (no file created)

## Phase I — Validate ladder

- [x] **I1** `validate:quick`: `go build ./... && go test -race -short ./...` → green.
- [x] **I2** `test:integration`: `go test -race -tags=integration $(go list ./... | grep -v internal/search)` → green.
- [x] **I3** `smoke:e2e`: build binary, start server with default config, harvest 1 session, verify file appears.
- [x] **I4** `self-review:staged-files`: `git status` clean before each commit.

## Phase J — Release notes

- [x] **J1** CHANGELOG.md entry under Unreleased:
  - Note BREAKING-LIKE behavior change: disk writes are now enabled by default
  - Document `write_to_disk: false` opt-out
  - Document `backfill-summaries` for historical sessions
  - Reference issue #258 + this OpenSpec change

## Phase K — PR + review gate + merge + release

- [x] **K1** Push branch.
- [x] **K2** Open PR with title `feat(summary): restore disk persistence for Obsidian compatibility (#258)`. Body links OpenSpec proposal + evidence.
- [x] **K3** Wait for Gemini review. Triage per R31 in `docs/evidence/.../gemini-triage.md`.
- [x] **K4** Address findings (max 3 push cycles).
- [x] **K5** Squash merge.
- [x] **K6** Close issue #258.
- [x] **K7** Tag `v2026.5.30NN` + push.
- [x] **K8** Verify Release workflow → npm publish both packages → confirm new version visible.
- [x] **K9** `openspec archive restore-summary-disk-persistence --yes` + commit.
- [x] **K10** Worktree cleanup.

## Estimated Effort

| Phase | LoC | Estimate |
|-------|-----|----------|
| A — Foundations | 0 | 5 min |
| B — Slugify | ~50 + 80 test | 30 min |
| C — Path | ~50 + 60 test | 25 min |
| D — Atomic write | ~80 + 90 test | 40 min |
| E — Config | ~30 + 30 test | 30 min |
| F — Persister wire | ~50 + 100 test | 1 h |
| G — Backfill CLI | ~150 + 100 test | 1.5 h |
| H — E2E live test | — | 30 min |
| I — Validate | — | 15 min |
| J — Release notes | ~25 | 10 min |
| K — PR + review + merge + release | — | 1-2 h |
| **Total** | **~895 LoC** | **~5-6 hours** |
