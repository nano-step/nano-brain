# PRE-MERGE override — Issue #328 / PR #335

**Date**: 2026-06-02

## [HARNESS-OVERRIDE] Gate 3.3 — integration tests (partial)

PR #335 actually **FIXES** part of gate 3.3 for the harvest package. Before this PR, `internal/harvest` failed with `[build failed]`. After this PR, `internal/harvest` runs and passes.

The remaining gate 3.3 failures are from `internal/server/handlers` (tracked by issue #326). Those are NOT introduced by this PR — they're pre-existing on master.

Confirmed by independent verification in worktree:
```
$ go test -race -tags=integration -short ./... 2>&1 | grep -E "FAIL|^ok|build failed"
ok    github.com/nano-brain/nano-brain/internal/harvest 2.280s   ← FIXED
ok    github.com/nano-brain/nano-brain/internal/search  1.041s
FAIL  github.com/nano-brain/nano-brain/internal/server/handlers  ← issue #326
```

## [HARNESS-OVERRIDE] Gate 3.4 — golangci-lint

Same pre-existing lint violations in `internal/server/handlers/*_test.go` (errcheck on json.Unmarshal, unused funcs). Not in scope for #328.

## [HARNESS-OVERRIDE] Gate 3.12 — smoke:e2e

Change-type=bug-fix triggers the smoke:e2e requirement. However, this PR fixes a **test-only file** (`_test.go`) — there is no runtime surface, no API change, no behavior change.

The smoke:e2e equivalent for a test-only fix is:
1. `go build -tags=integration ./internal/harvest/...` — clean (was failing before)
2. `go test -tags=integration -short ./internal/harvest/...` — `ok` (was `[build failed]` before)

Both verified in `docs/evidence/self-review-zfix-328-harvest-build-error.md`. Documenting `[HARNESS-OVERRIDE]` since the canonical smoke:e2e procedure (build binary + start server + curl endpoints) doesn't apply to a test-file fix.

## All other gates PASS (8 PASS + 3 SKIP)

Ready to merge.
