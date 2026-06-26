# Pre-Merge Override: improve Rails capability benchmark score

Date: 2026-06-24
Issue: #489
Branch: `feat/489-rails-capability-score-clean`
PR: #491

## Scope of this branch

This branch contains three logical changes only:

1. Rails/Ruby graph traversal improvements
   - Ruby constant symbol extraction
   - impact target symbol fallback
   - non-HTTP Rails job/service flow entries
   - MCP flow entry parity with HTTP handler behavior
2. Agent-oriented capability benchmark scoring
   - fixed recall remains a diagnostic layer
   - agent recall uses deterministic query/question/input/symbol retrieval
   - Rails capability benchmark profile added with privacy-safe placeholders
3. Harness artifacts for issue #489
   - OpenSpec proposal/design/spec/tasks
   - story packet
   - self-review, review verdict, and score-only evidence

## Gate 3.3 — Integration Tests

**Status:** PRE-EXISTING / UNRELATED FAILURES

`go test -race -tags=integration ./...` currently fails in packages and scenarios outside this branch's implementation scope:

- `internal/bench/TestBenchmarkNanoBrain` — pre-existing benchmark regression fixture/server-state failure.
- `internal/graph/TestExpressExtractor_Integration` — pre-existing Express middleware expectation mismatch.
- `internal/graph` Ruby integration tests — pre-existing workspace-registration failures in integration fixtures. The raw output includes runtime workspace identifiers and is intentionally not copied here.

This branch does **not** modify:

- `internal/bench/*`
- Express extractor code or fixtures
- workspace registration handlers / integration test harness setup

**Targeted verification for touched Go packages passes:**

```bash
go test -race -short ./internal/flow ./internal/server/handlers ./internal/mcp
go test -race -tags=integration ./internal/storage -run TestGraphImpactQueriesMatchSymbolPart -v
go build ./... && go test -race -short ./...
```

## Gate 3.4 — golangci-lint

**Status:** PRE-EXISTING / UNRELATED FAILURES

After fixing the benchmark runner's unused test helper, `golangci-lint run ./...` reports only existing findings outside this change:

- `tools/ignorecleanup/main.go:193` — unchecked rollback error.
- `internal/watcher/watcher.go:591` — staticcheck `SA4004` on pre-existing loop control.
- `internal/server/handlers/reindex_cfg.go:102` — pre-existing ineffectual assignment.
- `internal/search/reranking/reranker.go:88` — pre-existing unused type.
- `internal/graph/js_cflow.go:262` — pre-existing unused helper.
- `internal/graph/js_cflow.go:447` — pre-existing ineffectual assignment.

There are no lint findings in the benchmark files added/modified by this branch after the test-only HTTP client fix.

## [HARNESS-OVERRIDE] Gate 3.3 — integration tests

Reason: The failing integration suite items are in unrelated benchmark, Express, and workspace-registration scenarios that this branch does not modify. Quick tests and targeted package tests for the changed surfaces pass.

## [HARNESS-OVERRIDE] Gate 3.4 — golangci-lint

Reason: The active lint failures are pre-existing in unrelated files. New benchmark lint findings introduced during development were fixed before final review.

## Branch readiness summary

| Check | Status |
| --- | --- |
| `go build ./...` | PASS |
| `go test -race -short ./...` | PASS |
| capbench compile for core + Rails | PASS |
| Rails agent-oriented score-only run | PASS — overall `0.795` |
| nano-brain agent-oriented score-only run | PASS — overall `1.000` |
| OpenSpec strict validation | PASS |
| In-progress harness gate | PASS |
| Commit count | PASS — 3 commits |
| Privacy grep | PASS |
