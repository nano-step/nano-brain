# Self-Review: rails flow continuation follow-up (PR #488, Issue #487)

Date: 2026-06-23
Reviewer: Sisyphus orchestration + targeted regression verification

## Findings

| # | Severity | File | Description | Status |
|---|----------|------|-------------|--------|
| 1 | major | `internal/flow/builder.go` | Reconcile traversal reused the route file as `parentFile`, which filtered out real controller `calls` edges and caused `memory_flow` to stop at route → handler | FIXED |
| 2 | major | `internal/flow/builder_test.go` | No unit regression existed for namespaced Rails controller continuation through reconcile + calls edges | FIXED |
| 3 | major | `internal/server/handlers/flow_test.go` | No handler-path regression existed for `GraphFlow` returning downstream `calls` edges for Rails namespaced controllers | FIXED |

## Verification

- `go test ./internal/flow` ✅
- `go test ./internal/server/handlers -run 'TestGraphFlow_RailsNamespacedController'` ✅
- `go test -race -short ./...` ✅
- `./scripts/harness-check.sh in-progress` ✅

## Summary

- Major: 3 found, 3 fixed
- No new integration or lint failures introduced beyond the branch's documented pre-existing overrides in `docs/evidence/487-pre-merge-override.md`
