# PRE-MERGE override — Issue #330 / PR #333

**Date**: 2026-06-02
**PR**: #333

## Gate output

```
[PASS] 3.1 go build
[PASS] 3.2 go test -race -short
[FAIL] 3.3 go test -race -tags=integration  ← pre-existing
[FAIL] 3.4 golangci-lint                    ← pre-existing
[PASS] 3.5-3.12 (all green)
[SKIP] 3.13 no web change
Summary: 10 PASS, 2 FAIL, 1 SKIP
```

## [HARNESS-OVERRIDE] Gate 3.3 — integration tests

Pre-existing failures on master, NOT introduced by PR #333. PR touches only:
- `cmd/nano-brain/client.go` (production)
- `cmd/nano-brain/commands_test.go`, `ops_test.go` (test files in `cmd/nano-brain` — passes)
- `docs/evidence/*.md` (docs)

None of `internal/harvest`, `internal/search`, `internal/server/handlers` are touched.

Failures tracked by open issues:
- `internal/harvest` build failure → #325
- `internal/search` build failure → #326
- `internal/server/handlers` test failures (`TestEventsIntegration_ReindexPublishesSequence`, `TestListWorkspacesE2E`) → #327

Same evidence already documented for PR #322 + #332.

## [HARNESS-OVERRIDE] Gate 3.4 — golangci-lint

Pre-existing lint violations in `internal/server/handlers/*_test.go` (errcheck on json.Unmarshal, unused funcs). Not in scope for #330.

## All other gates PASS

Including 3.12 smoke:e2e (4 scenarios documented in `smoke-e2e-330.md` with curl + binary output).

Ready for review-work + merge.
