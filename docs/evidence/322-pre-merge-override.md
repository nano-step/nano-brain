# PR #323 — Pre-Merge Gate Override Evidence

Date: 2026-06-02
PR: https://github.com/nano-step/nano-brain/pull/323
Issue: https://github.com/nano-step/nano-brain/issues/322

## Pre-merge gate output

```
[PASS] 3.1 go build ./...
[PASS] 3.2 go test -race -short ./...
[FAIL] 3.3 go test -race -tags=integration ./... failed
[FAIL] 3.4 golangci-lint found issues
[PASS] 3.5 Review Verdict: PASS in docs/evidence/review-gate-188.md (R27)
[PASS] 3.6 No Gemini comments on PR
[PASS] 3.7 CI checks passing
[PASS] 3.8 PR closes exactly 1 issue (R1)
[PASS] 3.9 PR targets master
[PASS] 3.10 Self-review evidence found
[PASS] 3.11 PR commit count: 2 (R29: ≤ 3)
[SKIP] 3.12 No smoke-e2e-322*.{md,txt}  ← FIXED by adding docs/evidence/smoke-e2e-322.md
[SKIP] 3.13 no web change (smoke:ui not required)
```

After adding `docs/evidence/smoke-e2e-322.md` (SKIP justified per D11), 3.12 will move to PASS.
3.3 and 3.4 require this override.

## [HARNESS-OVERRIDE]: Gate 3.3 — Pre-existing integration test failures, NOT introduced by this PR

`go test -race -tags=integration ./...` fails in 3 packages, none of which this PR touches:

```
=== Files touched by PR #323 ===
$ git diff --name-only origin/master...HEAD
CHANGELOG.md
docs/evidence/...
internal/embed/AGENTS.md
internal/embed/queue.go
internal/embed/queue_integration_test.go      ← NEW, this PR's tests, both PASS
internal/embed/queue_test.go
migrations/00014_add_chunks_embed_status_index.sql
openspec/changes/embed-status-index-and-inflight-dedup/*
```

```
=== Failing files (NOT touched) ===
internal/harvest/opencode_sqlite_integration_test.go:167-168 — undefined: q  [pre-existing build error]
internal/search/isolation_test.go:483 — HybridSearch signature mismatch  [pre-existing API drift]
internal/server/handlers/events_integration_test.go:96 — TestEventsIntegration_ReindexPublishesSequence assertion mismatch  [pre-existing test bug]
internal/server/handlers/workspace_integration_test.go:127 — TestListWorkspacesE2E JSON shape mismatch  [pre-existing test/handler drift]
```

**Verification this PR's integration tests pass cleanly**:
```
$ go test -race -tags=integration ./internal/embed/...
ok  github.com/nano-brain/nano-brain/internal/embed
```
Both `TestQueue_ScanByStatus_SkipsInflightChunks` and `TestMigration_EmbedStatusIndex_Exists` PASS.

**Pre-existing nature documented**: All 4 failing test files have unchanged git mtime predating this PR's `e98d5d2` commit. Failures exist on `master` as of `1615a96`.

**Compensating control**: The integration tests for this PR's surface (embed queue + migration 00014) are in a separate file (`internal/embed/queue_integration_test.go`) and PASS cleanly. The PR description references this override evidence. Reviewers can verify with `git log --oneline origin/master -- <failing-file-path>` showing all failing files predate this branch.

## [HARNESS-OVERRIDE]: Gate 3.4 — golangci-lint findings are pre-existing, NOT introduced by this PR

`golangci-lint run ./...` reports 7 findings. Only 1 was in this PR's new code and was FIXED:

```
=== Originally found in this PR's new code ===
internal/embed/queue_test.go:925:25  errcheck: defer func() { recover() }()
  → FIXED by changing to `defer func() { _ = recover() }()` (amended into commit e98d5d2)
```

```
=== Pre-existing findings (NOT in this PR's diff) ===
internal/server/handlers/documents_test.go:169,195,223  errcheck: json.Unmarshal not checked
internal/server/handlers/events_test.go:18,31  unused: setupSSERequest, readSSEEvent
cmd/nano-brain/cmd_detect_changes.go:215  unused: getChangedLineRanges
```

**Verification**:
```
$ golangci-lint run ./internal/embed/...   # only my package
[no output → clean]
```

**Compensating control**: This PR introduces ZERO new lint findings in its own changed files. Pre-existing findings should be addressed in separate follow-up PRs (out of scope per "Surgical Changes" behavioral guideline — don't clean up adjacent code).

## Verdict

PROCEED with merge after Gemini review cycles. Both overrides are well-documented pre-existing-failure scenarios. This PR's own surface is fully validated:
- `go build ./...` PASS
- `go test -race -short ./...` PASS (all packages)
- `go test -race -tags=integration ./internal/embed/...` PASS (this PR's surface)
- `golangci-lint run ./internal/embed/...` PASS (this PR's surface)
- EXPLAIN ANALYZE confirms index usage
