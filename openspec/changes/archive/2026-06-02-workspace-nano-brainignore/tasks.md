# Tasks — Workspace-local `.nano-brainignore` (#317)

## Pre-implementation
- [x] GitHub issue #317 created and labeled (`lane:normal`, `change-type:user-feature`, `enhancement`).
- [x] Worktree created at `.opencode/worktrees/feat-317-workspace-nano-brainignore` on branch `feat/317-workspace-nano-brainignore` from `origin/master`.
- [x] Deep-design pipeline complete: Metis + Oracle Phase 1 + cross-critique + Momus sanity = PASS.
- [x] OpenSpec proposal artifacts authored (proposal.md, design.md, tasks.md, specs/watcher-file-filtering delta).

## Implementation

### Code changes
- [ ] `internal/watcher/filter.go`:
  - [ ] Add `localIgnore *gitignore.GitIgnore` field to `fileFilter` struct.
  - [ ] Change `newFileFilter` signature to return `(*fileFilter, error)`.
  - [ ] Inline-load `<rootDir>/.nano-brainignore` using `gitignore.CompileIgnoreFile` (mirror the existing `.gitignore` block at lines 60-65). On parse error, return the error; on success, assign to `f.localIgnore`.
  - [ ] Add nil-checked `localIgnore.MatchesPath(matchRel)` check in `shouldSkip` between `globalIgnore` (line 88) and `gitignoreMatcher` (line 92).

- [ ] `internal/watcher/watcher.go`:
  - [ ] Update the single `newFileFilter(...)` call site (~line 154) to handle the new `error` return.
  - [ ] On success-with-loaded-local-file: `w.logger.Debug().Str("dir", absPath).Str("path", localIgnPath).Msg("loaded workspace .nano-brainignore")`.
  - [ ] On IO error (file exists but `os.ReadFile` fails — permission denied, path-is-directory, etc.): `w.logger.Warn().Err(err).Str("path", localIgnPath).Msg("workspace .nano-brainignore failed to load, continuing without local matcher")`. Do NOT abort the watch — collection proceeds with nil localIgnore. (Note: `go-gitignore` does NOT report "malformed content" errors — it tolerates any pattern content. Only IO-level errors reach this path.)

### Tests
- [ ] `internal/watcher/filter_test.go` — add 5 new `t.Run` cases following the existing `TestFileFilter_GlobalIgnoreApplies` and `TestFileFilter_GlobalIgnoreCombinesWithPerCollection` patterns:
  - [ ] `TestFileFilter_LocalNanoBrainIgnoreApplies` — file exists with `*.tmp` → `shouldSkip("foo.tmp", false) == true`, `shouldSkip("foo.go", false) == false`.
  - [ ] `TestFileFilter_LocalNanoBrainIgnoreMissing` — no file → nil matcher, no error, baseline behavior preserved.
  - [ ] `TestFileFilter_LocalNanoBrainIgnoreCombinesWithGlobal` — global `*.log` + local `*.tmp` → both skipped independently; non-matching `*.go` passes.
  - [ ] `TestFileFilter_LocalNanoBrainIgnoreCombinesWithGitignore` — `.gitignore` `tmp/` + `.nano-brainignore` `*.snap` → both apply; `tmp/x.go` AND `foo.snap` skipped.
  - [ ] `TestFileFilter_LocalNanoBrainIgnoreUnreadable` — `go-gitignore` does NOT reject content (any bytes compile into a matcher with zero effective patterns), so use an IO-level failure. Preferred approach: create `<rootDir>/.nano-brainignore` as a **directory** (`os.Mkdir`) — cross-platform, reliable, no chmod needed. `os.ReadFile` returns `is a directory` on Linux/macOS and the equivalent on Windows. Assert `newFileFilter` returns a non-nil error AND the returned `*fileFilter` (if non-nil) has nil `localIgnore`, AND `shouldSkip` still applies other filter layers correctly. Use `t.Cleanup(func(){ os.RemoveAll(ignPath) })` only if needed; `t.TempDir()` handles directory removal.
- [ ] Update existing tests if any are broken by `newFileFilter` signature change (only test-file callers exist; should be straightforward).

### Documentation
- [ ] `README.md`:
  - [ ] Rename section "Global ignore patterns (`~/.nano-brain/.nano-brainignore`)" → "Ignore patterns".
  - [ ] Split into two subsections: "Global (`~/.nano-brain/.nano-brainignore`)" and "Workspace-local (`<workspace_root>/.nano-brainignore`)".
  - [ ] Update the "Order of evaluation" list to insert `localIgnore` at position 3.
  - [ ] Add a "Reload behavior" note: restart server OR re-register via `POST /api/v1/init` to pick up changes.
  - [ ] Add the "Why workspace-local" rationale (one paragraph: team-shareable, version-controllable, project-specific overrides).

## Validation ladder (per `docs/HARNESS.md`)

- [ ] `validate:quick` — `go build ./... && go test -race -short ./...` from worktree root. Paste output.
- [ ] `self-review:response-shape` — N/A (no HTTP handler changes; no new response struct).
- [ ] `self-review:staged-files` — `git status` before each commit. Must NOT include `.opencode/`, `package-lock.json`, or files outside this feature.
- [ ] `test:integration` — `go test -race -tags=integration ./...` from worktree root. Paste output.
- [ ] `smoke:e2e` — manual sequence (required for user-feature):
  1. Build: `go build -o /tmp/nb317 ./cmd/nano-brain`.
  2. Start with fresh DB (Docker compose or NANO_BRAIN_CONFIG-pointed test config on port 8899 to avoid clashing with production 3100).
  3. `mkdir -p /tmp/smoke317 && cd /tmp/smoke317 && echo 'package main' > foo.go && echo 'snapshot data' > skip.snap && printf '*.snap\n' > .nano-brainignore`.
  4. `curl -sX POST http://localhost:8899/api/v1/init -H 'Content-Type: application/json' -d '{"root_path":"/tmp/smoke317","name":"smoke317"}'` → capture workspace hash.
  5. Wait for indexing (or `POST /api/v1/reindex`).
  6. `POST /api/v1/search` with `{"workspace":"<hash>","query":"snapshot"}` → MUST NOT include `skip.snap`.
  7. `POST /api/v1/search` with `{"workspace":"<hash>","query":"package main"}` → MUST include `foo.go`.
  8. Tail server logs at DEBUG level → MUST see `loaded workspace .nano-brainignore` for the smoke317 code collection.
- [ ] Capture smoke evidence at `docs/evidence/self-review-feat-317-workspace-nano-brainignore.md` with timestamps + curl outputs.

## Review gate

- [ ] Self-review forbidden per harness rules. After validation ladder + smoke pass, request Oracle review of the diff. Capture verdict in `docs/evidence/self-review-feat-317-workspace-nano-brainignore.md`.

## PR + merge

- [ ] Run `bash scripts/harness-check.sh pre-merge` from worktree.
- [ ] `git push origin feat/317-workspace-nano-brainignore` (verify `git branch --show-current` first).
- [ ] `gh pr create --repo nano-step/nano-brain --title "feat(watcher): workspace-local .nano-brainignore (#317)" --body <fill>` with link to issue, smoke evidence path, and validation ladder output.
- [ ] Address PR bot review (Gemini) loop until clean.

## Post-merge

- [ ] `bash scripts/harness-check.sh post-merge`.
- [ ] `openspec archive workspace-nano-brainignore` (updates `openspec/specs/watcher-file-filtering/spec.md` with the ADDED Requirement).
- [ ] `gh issue close 317 --repo nano-step/nano-brain --comment "Implemented in PR #<NN>. Workspace-local .nano-brainignore supported as of release v<YYYY.M.D.N>."`.
- [ ] Clean up worktree: `git worktree remove .opencode/worktrees/feat-317-workspace-nano-brainignore`.
- [ ] `bash scripts/harness-check.sh next-ready` to confirm gates green for next feature.

## Deferred follow-ups (separate issues — NOT in this PR)

- [ ] Hot-reload of `.nano-brainignore` via fsnotify (sibling of #263 follow-up).
- [ ] Tombstone/reconciliation: delete already-indexed docs that newly match `.nano-brainignore` patterns (companion to existing TODO at `watcher.go:299-302`).
- [ ] Stale `docs/HARNESS_GATES.md` `b-main` references (single-trunk master model — noted by cross-critique, unrelated to this feature).
