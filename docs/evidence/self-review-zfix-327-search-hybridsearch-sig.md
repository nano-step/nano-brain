# Self-review — Issue #327 / PR (TBD)

**Date**: 2026-06-02
**Story**: 327 (HybridSearch signature mismatch in isolation_test.go)
**Lane**: tiny | **Change-type**: bug-fix
**Branch**: `fix/327-search-hybridsearch-sig`
**Implementing agent**: Sisyphus orchestrator (ast-grep + direct edit — 5-line test fix)

## Scope

| File | Change |
|---|---|
| `internal/search/isolation_test.go` | 5 lines: add `, nil` as 5th arg to HybridSearch calls (lines 315, 331, 347, 360, 483) |
| `docs/evidence/327-pre-work-gate.md` | new evidence |
| `docs/evidence/self-review-zfix-327-search-hybridsearch-sig.md` | this file |

Production code: zero changes. Test file only.

## self-review:response-shape

**N/A** — test fix, no API surface change.

## self-review:staged-files

```
$ git status (post-stage)
On branch fix/327-search-hybridsearch-sig
Changes to be committed:
	modified:   internal/search/isolation_test.go
	new file:   docs/evidence/327-pre-work-gate.md
	new file:   docs/evidence/self-review-zfix-327-search-hybridsearch-sig.md
```

✅ No `.opencode/`, no `package-lock.json`, no binary artifacts.

## Validation evidence

```
$ go build ./...
(no output — success)

$ go test -race -short ./... | grep -E "FAIL|^ok" | tail -10
ok  internal/search           1.022s
ok  internal/server           1.062s
ok  internal/server/handlers  1.778s
... (all packages pass)

$ go test -race -tags=integration -short ./internal/search/...
ok  internal/search  1.574s
```

Before this PR: `FAIL [build failed]`. After: `ok` with passing isolation tests.

## Backward compat

Zero impact. Tests in this file are gated behind `//go:build integration` and were not running in CI's default `validate:quick` step. Their failure was discovered when documenting harness override for PR #322 gate 3.3.

## Why `nil` is correct (not `[]string{}`)

Both work identically — `HybridSearch` checks `len(tags) > 0` to decide whether to dispatch to tag-filtered queries. `nil` slice has length 0, same as `[]string{}`. Idiomatic Go preference is `nil` for the "no value" case.

## Conclusion

5-line test fix, ast-grep-validated for safety. Integration tests for `internal/search` now pass. Unblocks harness gate 3.3 for the search package.
