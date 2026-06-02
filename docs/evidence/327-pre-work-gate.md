# PRE-WORK gate — Issue #327

**Date**: 2026-06-02
**Issue**: #327 (internal/search isolation_test.go HybridSearch signature mismatch)
**Lane**: tiny | **Change-type**: bug-fix
**Branch**: `fix/327-search-hybridsearch-sig`

## Lane (tiny)

- 1 test file changed: `internal/search/isolation_test.go`
- 5 line edits (add `, nil` to each HybridSearch call)
- 0 production code changes
- Risk flags: 0

## Root cause

`SearchService.HybridSearch` signature was extended at some point to accept a `tags []string` parameter:

```go
func (s *SearchService) HybridSearch(ctx context.Context, query string, workspace string, maxResults int, tags []string) ([]Result, error)
```

But `isolation_test.go` (build-tagged `//go:build integration`) was never updated — 5 call sites still pass only 4 args, causing build failure:

```
internal/search/isolation_test.go:483:72: not enough arguments in call to svc.HybridSearch
    have (context.Context, string, string, number)
    want (context.Context, string, string, int, []string)
```

## Fix

Add `, nil` (= no tag filter, equivalent to passing `[]string{}`) to all 5 call sites in `isolation_test.go`:
- Line 315: `svc.HybridSearch(ctx, wsAlpha.keyword, wsAlpha.hash, 20, nil)`
- Line 331: `svc.HybridSearch(ctx, wsBeta.keyword, wsBeta.hash, 20, nil)`
- Line 347: `svc.HybridSearch(ctx, wsBeta.keyword, wsAlpha.hash, 20, nil)`
- Line 360: `svc.HybridSearch(ctx, wsAlpha.keyword, wsBeta.hash, 20, nil)`
- Line 483: `svc.HybridSearch(ctx, other.keyword, queried.hash, 20, nil)`

Used `ast-grep` for safe AST-aware replacement (matched pattern `svc.HybridSearch(ctx, $A, $B, 20)` → `svc.HybridSearch(ctx, $A, $B, 20, nil)`).

`nil` is the appropriate value because:
- Pre-existing test intent was to test isolation behavior WITHOUT tag filtering
- Tests still cover the un-tagged code path
- `[]string{}` vs `nil` is functionally identical in HybridSearch (both fall through to base queries)

## Skip justifications

- OpenSpec proposal: SKIP (tiny lane)
- smoke:e2e: SKIP — test-only file change, no runtime surface
- Review gate: ⚠️ self-verify only (per HARNESS.md change-type table for bug-fix in test files)

## After-fix verification

```
$ go test -race -tags=integration -short ./internal/search/...
ok  github.com/nano-brain/nano-brain/internal/search  1.574s
```

Before: `FAIL ... [build failed]`. After: `ok` with passing tests.
