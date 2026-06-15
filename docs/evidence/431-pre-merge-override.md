# Pre-Merge Override: feat/multi-framework-extraction

Date: 2026-06-15
PR: #434 (branch: feat/multi-framework-extraction)
Issue: #431

## Gate 3.3 — Integration Tests

**Status:** PRE-EXISTING FAILURE (not introduced by this PR)

**Failing tests:**
1. `TestBenchmarkNanoBrain` — requires running benchmark server on port 3199
2. `TestResolveWorkspaceParam_CaseInsensitive` — test expectation mismatch

**Evidence:**
- These tests fail on master branch as well
- My new integration test `TestExpressExtractor_Integration` passes successfully
- Graph package integration tests: PASS

**Verification:**
```
go test -race -tags=integration ./internal/graph/... -run Express -v
=== RUN   TestExpressExtractor_Integration
    express_integration_test.go:34: Extracted 15 edges
    --- PASS: TestExpressExtractor_Integration (0.05s)
PASS
```

## Gate 3.4 — golangci-lint

**Status:** PRE-EXISTING FAILURE (not introduced by this PR)

**Failing lint:**
- `internal/graph/http_router_helpers.go:10:5: var 'httpVerbs' is unused`

**Evidence:**
- This variable was already unused before my changes (verified via `git show HEAD:internal/graph/http_router_helpers.go`)
- My new files (`express_extractor.go`, `ts_router_helpers.go`) have no lint errors
- The unused variable is in a file I did not modify

## Summary

Both gate 3.3 and 3.4 failures are pre-existing on master and unrelated to the Express.js route extractor implementation. This PR:
- Adds 4 new files (express_extractor.go, ts_router_helpers.go, tests, fixtures)
- Modifies 1 file (main.go) to register the extractor
- All new code passes build, unit tests, and integration tests
- No new lint errors introduced

## Override Request

Per HARNESS.md R7, requesting `[HARNESS-OVERRIDE]` for gates 3.3 and 3.4 due to pre-existing failures unrelated to this PR.
