# Self-Review: Issue #497 — recursive fsnotify watches for subdirectories

Change type: **bug-fix** · Lane: code · Scope: `internal/watcher`

## Actions Taken
- Diagnosed (code + live DB + a live probe test) that the file watcher registered
  an fsnotify watch only on each collection **root**. fsnotify is non-recursive,
  so edits in subdirectories fired no event and were indexed only via the
  periodic poll (gated on `hasNewEvents`) or a restart.
- Added `watchedDirs` bookkeeping + a `watchDir` helper that idempotently adds a
  watch per directory during the existing `scanCollection` walk (excluded dirs
  are `SkipDir`'d before `watchDir`, so the fd cost tracks source dirs only).
- Routed fsnotify events to the owning collection root by path-prefix in
  `handleFSEvent` (was: exact `collections[parentDir]`), so the ~2s debounce path
  picks up subdir edits.
- Cleaned up watches on dir removal/rename and in `Unwatch` (prefix sweep);
  reset `watchedDirs` when the fsnotify watcher is recreated in `Run`.

## Files Changed
- `internal/watcher/watcher.go` — `watchedDirs` field, `watchDir` helper,
  recursive add in `scanCollection`, prefix event routing in `handleFSEvent`,
  watch cleanup in `Unwatch`/`Run`/`Watch`.
- `internal/watcher/watcher_test.go` — `TestScanCollection_WatchesSubdirectories`
  (unit) and `TestSubdirEdit_IndexedWithoutRootActivity` (E2E regression).

## Findings Summary
- Concurrency: `watchDir` takes `w.mu`; verified `scanCollection` is never called
  while `w.mu` is held (`processDirty`/`processAll` unlock before calling). No
  deadlock. `fsw` and `watchedDirs` are read/written only under `w.mu`.
- New-subdir gap: a directory created at runtime is walked + watched on the next
  scan; files created before the watch registers are still indexed by that same
  `WalkDir` pass. No gap.
- fd exhaustion: only a logged degrade-to-poll; excluded dirs never reach `watchDir`.

## Resolution Status
- All findings: **RESOLVED**. No open critical/major issues.
- Independent review (oh-my-claudecode:code-reviewer): **Review Verdict: PASS** (see `review-497.md`).

## Validation
- `CGO_ENABLED=0 go build ./...` — PASS
- `go test -race -short ./...` — PASS (all packages)
- `go test -race -tags=integration ./internal/watcher/` — PASS (incl. new regression test)
- Live probe (real running server): subdir file not indexed for 18s with no root
  activity (old behavior reproduced); after fix's regression test, subdir edit is
  indexed off its own event with no root activity.
- Pre-existing/unrelated: `internal/graph` and `internal/bench` integration tests
  fail identically on clean `origin/master` (verified via worktree) — environmental
  (workspace registration / seeded benchmark corpus), not caused by this change.
