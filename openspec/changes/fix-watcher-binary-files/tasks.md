# Tasks: Fix Watcher Binary File Indexing

Tracking: #252

Tasks ordered for safe incremental commits. Each phase independently committable; run `validate:quick` after each phase.

## Phase A — Foundations

- [ ] **A1** — Confirm existing watcher tests pass on `b-main`:
  ```bash
  cd /Users/tamlh/workspaces/self/AI/Tools/nano-brain/.opencode/worktrees/fix-watcher-binary-file-utf-8-errors-252/fix-watcher-binary-file-utf-8-errors-252
  go test -race -short ./internal/watcher/...
  ```
  Establishes green baseline.

## Phase B — Binary detection helper (closes leak: no detection)

- [ ] **B1** — Create new file `internal/watcher/binary.go` with:
  - `binaryExtensions` package-level `map[string]bool` (hardcoded list per proposal §AC1)
  - `isBinaryExtension(filePath string) bool` — case-insensitive `filepath.Ext` lookup
  - `isBinaryContent(content []byte) bool` — returns `!utf8.Valid(content)`

- [ ] **B2** — Create `internal/watcher/binary_test.go` covering:
  - Each known binary extension returns true (table-driven test)
  - Each known text extension (`.go`, `.md`, `.ts`, `.yml`, `.sql`, `.json`, `.txt`, `.toml`) returns false
  - Unknown extension (`.xyz`) returns false (UTF-8 check handles unknowns)
  - Case insensitivity (`.PNG`, `.Jpg`)
  - `isBinaryContent`: valid UTF-8 returns false; PNG magic bytes `\x89PNG\r\n\x1a\n` returns true; JPEG SOI `\xff\xd8\xff` returns true; mixed valid+invalid returns true

- [ ] **B3** — `go test -race -short ./internal/watcher/...` PASS. Commit: `feat(watcher): add binary file detection helpers (#252)`

## Phase C — Wire into processFile (closes the actual bug)

- [ ] **C1** — Modify `internal/watcher/watcher.go` `processFile()` (around line 341-355):
  - After existing `info.Size() > w.maxFileSize` check (line 341), add `isBinaryExtension(filePath)` check. On true: `w.logger.Info().Str("file", filePath).Msg("skipping binary file (extension)")` and `return`.
  - After `os.ReadFile()` (line 350-354), add `isBinaryContent(content)` check. On true: `w.logger.Warn().Str("file", filePath).Msg("skipping binary file (non-UTF8 content)")` and `return`.

- [ ] **C2** — Verify `go build ./...` passes (no import issues).

- [ ] **C3** — Run watcher unit tests: `go test -race -short ./internal/watcher/...` — must remain green (no regression).

- [ ] **C4** — Commit: `fix(watcher): skip binary files before UTF-8 upsert (#252)`

## Phase D — End-to-end mock-based test (matches existing watcher test pattern)

Revised from original spec: use `mockQuerier` like all other watcher tests (e.g., `TestProcessFile_SkipsLargeFile`). Real-PG integration is unnecessary because the fix is purely watcher-side; the mock verifies `upsertDocCalls.Load() == 0` which is the exact assertion needed.

- [ ] **D1** — Append to `internal/watcher/watcher_test.go`:
  - `TestProcessFile_SkipsBinaryExtension` — write `image.png` to `t.TempDir()` with PNG bytes; call `processFile`; assert `upsertDocCalls == 0`.
  - `TestProcessFile_SkipsBinaryContentDespiteExtension` — write `trap.txt` containing PNG magic bytes; call `processFile`; assert `upsertDocCalls == 0` (UTF-8 safety net catches it).
  - `TestProcessFile_AcceptsValidUTF8` — write `notes.md` with UTF-8 content; call `processFile`; assert `upsertDocCalls == 1` (regression test for text files).

- [ ] **D2** — `go test -race -short ./internal/watcher/... -v` → all 3 new tests PASS + no regression. Commit: `test(watcher): cover binary file skip + valid UTF-8 happy path (#252)`

## Phase E — Validation ladder

- [ ] **E1** — `validate:quick`: `go build ./... && go test -race -short ./...` → green. Paste output to story Evidence.

- [ ] **E2** — `test:integration`: `go test -race -tags=integration ./...` → green for all packages I touched. (Pre-existing `internal/search/isolation_test.go` build break from #221 remains out of scope.)

- [ ] **E3** — `smoke:e2e` (per bug-fix change-type, see HARNESS validation ladder):
  - Build binary
  - Start server on port 8899 against test workspace registered at `/tmp/rrit-binary-test/`
  - Copy real PNG + real JPEG + a .md file into `/tmp/rrit-binary-test/`
  - Wait 5s for debounce
  - `curl /api/v1/search` for the .md content → expect 1 hit
  - `tail logs` → assert 0 `index failed` lines
  - Capture evidence to `docs/evidence/fix-watcher-binary-files/`

- [ ] **E4** — `self-review:staged-files`: `git status` clean before each commit (no `.opencode/`, no `node_modules/`).

## Phase F — PR + review gate

- [ ] **F1** — Push branch: `git push -u origin fix/252-watcher-binary-files`

- [ ] **F2** — Open PR with title `fix(watcher): skip binary files (#252)` and body containing `Closes #252`. Reference OpenSpec change folder + evidence.

- [ ] **F3** — Wait for Gemini PR bot review (org-level GitHub App, auto-fires).

- [ ] **F4** — Triage Gemini comments per R31 rule in `docs/HARNESS.md`. Record in `docs/evidence/fix-watcher-binary-files/gemini-triage.md`.

- [ ] **F5** — On Gemini PASS + Review Gate PASS: merge via squash. Bot review loop max 3 cycles.

- [ ] **F6** — Tag + npm publish (follow existing pattern from #238 / v2026.5.3006):
  ```bash
  git tag -a v2026.5.30XX <squash-sha> -m "..."
  git push origin v2026.5.30XX
  ```
  Verify Release workflow + npm publish both green.

- [ ] **F7** — `openspec archive fix-watcher-binary-files --yes` to finalize specs.

## Estimated Effort

| Phase | LoC | Estimate |
|-------|-----|----------|
| A — Foundations | 0 | 5 min |
| B — Binary helper + unit tests | ~60 (impl) + ~80 (tests) | 30 min |
| C — Wire into processFile | ~15 | 15 min |
| D — Integration test | ~120 | 45 min |
| E — Validate ladder | — | 20 min |
| F — PR + review + merge + tag + publish | — | 1-2 hours |
| **Total** | **~275 LoC** | **~3 hours** |
