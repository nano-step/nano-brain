## Self-Review: Story 2.6 — File Watcher
Date: 2026-05-23
Reviewer: Oracle

## Findings
| # | Severity | File | Description | Status |
|---|----------|------|-------------|--------|
| 1 | critical | main.go | Shutdown deadlock — errgroup gctx never cancelled on SIGTERM | FIXED — signal.NotifyContext |
| 2 | major | watcher.go | File deletion leaves stale documents in DB | DEFERRED — TODO added, by design per story spec |
| 3 | minor | watcher.go | Timer not stopped on fsnotify channel close | FIXED |
| 4 | minor | watcher.go | w.fsw not cleared after Run() exits | FIXED |
| 5 | minor | watcher.go | No defer Rollback on tx | FIXED |
| 6 | minor | watcher_test.go | Mock sourcePathHash map not synchronized | FIXED — comment added |

## Summary
- Critical: 1 found, 1 fixed
- Major: 1 found, 0 fixed (deferred by design — deletion handling out of scope)
- Minor: 4 found, 4 fixed
