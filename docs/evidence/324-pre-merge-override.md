# PRE-MERGE override — Issue #324 / PR #334

**Date**: 2026-06-02

## Same pre-existing failures as PR #322, #332, #333

Gate 3.3 (integration tests) and Gate 3.4 (golangci-lint) FAIL with the same pre-existing failures tracked by:
- `internal/harvest` build failure → issue #325
- `internal/search` build failure → issue #326
- `internal/server/handlers` integration test failures → issue #327

PR #334 touches only:
- `internal/embed/queue.go`, `internal/embed/queue_test.go` (refactor target)
- `internal/storage/queries/embeddings.sql`, `internal/storage/sqlc/embeddings.sql.go` (refactor target)
- `docs/evidence/*.md`

Zero overlap with the failing test packages. Failures are NOT introduced by this PR.

`[HARNESS-OVERRIDE]` 3.3 + 3.4 documented per harness rules.

## Gate 3.12 SKIP — appropriate

Change-type is `refactor`. Per HARNESS.md change-type table, smoke:e2e is NOT required for refactor lanes. Gate 3.12 correctly SKIPs.

## Review gate — self-verify only

Per HARNESS.md change-type table, refactor lane uses self-verify only (no 5-agent /review-work required). Self-review evidence is comprehensive — see `docs/evidence/self-review-zrefactor-324-remove-dead-permanently-failed.md`.

## All other gates PASS (9 PASS)

Ready to merge.
