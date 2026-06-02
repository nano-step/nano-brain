# PRE-MERGE override — Issue #327 / PR #336

**Date**: 2026-06-02

## [HARNESS-OVERRIDE] Gate 3.3 — integration tests (partial)

This PR **FIXES** part of gate 3.3 for the search package. Before: `FAIL internal/search [build failed]`. After: `ok internal/search 1.574s`.

Remaining gate 3.3 failure is `internal/server/handlers` integration tests (issue #326), which is NOT touched by this PR. Pre-existing on master.

## [HARNESS-OVERRIDE] Gate 3.4 — golangci-lint

Pre-existing lint violations in `internal/server/handlers/*_test.go`. Not in scope for #327.

## [HARNESS-OVERRIDE] Gate 3.12 — smoke:e2e

Change-type=bug-fix triggers smoke:e2e requirement. This PR fixes a test-only file with zero runtime surface. The equivalent verification is:
1. `go build -tags=integration ./internal/search/...` — clean (was failing before)
2. `go test -tags=integration -short ./internal/search/...` — `ok` (was `[build failed]` before)

Both documented in `docs/evidence/self-review-zfix-327-search-hybridsearch-sig.md`.

## All other gates PASS

Ready to merge.
