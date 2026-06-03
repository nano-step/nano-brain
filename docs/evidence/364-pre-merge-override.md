# PRE-MERGE override — Issue #364 / PR #366

**Date**: 2026-06-03

## Baseline verification

Ran integration tests + lint on **pure master `c52ab1a`** (the commit PR #366 branched from). Confirmed failures exist BEFORE this PR's single-line change.

### Master baseline integration test failures

```
FAIL github.com/nano-brain/nano-brain/internal/embed [build failed]
FAIL github.com/nano-brain/nano-brain/internal/server/handlers [build failed]
FAIL github.com/nano-brain/nano-brain/internal/harvest
  --- FAIL: TestOpenCodeSQLite_OrphanSession_NoWorktree_Skipped (0.28s)
  --- FAIL: TestOpenCodeSQLite_UnregisteredWorktree_Skipped (0.34s)
```

These failures are in packages **NOT touched by PR #366**. PR #366 only modifies `internal/watcher/watcher.go` (1 line). The watcher package's integration tests PASS on this branch.

### Master baseline lint failures

```
internal/server/handlers/documents_test.go:169:16: Error return value of `json.Unmarshal` is not checked (errcheck)
internal/server/handlers/documents_test.go:195:16: (same)
internal/server/handlers/documents_test.go:223:16: (same)
internal/server/handlers/events_test.go:18:6: func `setupSSERequest` is unused (unused)
internal/server/handlers/events_test.go:31:6: func `readSSEEvent` is unused (unused)
cmd/nano-brain/cmd_detect_changes.go:215:6: func `getChangedLineRanges` is unused (unused)
cmd/nano-brain/cmd_detect_changes.go:228:6: func `parseHunkHeaders` is unused (unused)
internal/server/handlers/graph_neighborhood.go:176:4: S1011: should replace loop with append (gosimple)
cmd/nano-brain/workspaces_test.go:352:27: S1030: should use out.Bytes() instead of []byte(out.String()) (gosimple)
```

All in packages **NOT touched by PR #366**.

## [HARNESS-OVERRIDE] Gate 3.3 — integration tests

Reason: Pre-existing build failures in `internal/embed` and `internal/server/handlers`, plus pre-existing test failures in `internal/harvest/TestOpenCodeSQLite_*`. None caused by PR #366. The watcher package's integration tests pass cleanly.

## [HARNESS-OVERRIDE] Gate 3.4 — golangci-lint

Reason: 9+ pre-existing lint violations in `internal/server/handlers/*_test.go`, `cmd/nano-brain/cmd_detect_changes.go`, `cmd/nano-brain/workspaces_test.go`, `internal/server/handlers/graph_neighborhood.go`. None caused by PR #366. Lint on the touched file (`internal/watcher/watcher.go`) is clean.

## [HARNESS-OVERRIDE] Gate 3.12 — smoke:e2e

Reason: Change-type=bug-fix triggers smoke:e2e requirement per R20. This PR demotes a log line from Info to Debug — there is no runtime surface to exercise. The equivalent verification is structural:

1. `go build ./...` — clean (no API changes)
2. `go test -race -short ./internal/watcher/` — pass (no behavior change)
3. Code inspection: the demoted line (`internal/watcher/watcher.go:394`) is the ONLY caller of `w.logger.Info().Msg("processing file")`; no callers of this log message exist downstream.

A live smoke would require: start daemon, register a workspace with ~1k files, wait 5+ minutes for `reindex_interval` to fire, tail logs, observe absence of `msg="processing file"` at INFO level. This is documented at the issue level (issue #364 "Reproduction" section) but cannot be automated in CI without a long-running test harness, which is out of scope for a 1-line log change.

## All other gates

| Gate | Status |
|---|---|
| 3.1 go build | ✅ PASS |
| 3.2 go test -race -short | ✅ PASS |
| 3.5 Review Verdict PASS | ⏳ pending independent review delegation |
| 3.6 No Gemini comments / triaged | ✅ PASS (Gemini reviewed, no findings) |
| 3.7 CI checks passing | ✅ PASS |
| 3.8 Closes #364 | ✅ PASS |
| 3.9 Targets master | ✅ PASS |
| 3.10 Self-review evidence | ✅ PASS (added: `docs/evidence/self-review-364.md`) |
| 3.11 Commit count ≤ 3 | ✅ PASS (1 commit) |
| 3.13 smoke:ui not required | ✅ SKIP (no web change) |

## Override invocation requirement

Per HARNESS.md R7: only the user account can post the `[HARNESS-OVERRIDE]: <reason>` comment on the PR. This evidence file documents the rationale; user must post the actual override comment on PR #366 to lift gates 3.3 + 3.4 + 3.12.

Suggested PR comment (≥ 20 chars):

> [HARNESS-OVERRIDE]: PR #366 is a 1-line log severity change. Pre-existing failures in embed/handlers/harvest and lint violations in other packages predate this PR (verified against master c52ab1a). See docs/evidence/364-pre-merge-override.md for full baseline. Smoke:e2e exempted — no runtime surface for log-only change.

Ready to merge after override comment posted + independent Review Verdict.
