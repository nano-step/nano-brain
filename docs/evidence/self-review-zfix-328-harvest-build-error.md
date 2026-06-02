# Self-review — Issue #328 / PR (TBD)

**Date**: 2026-06-02
**Story**: 328 (internal/harvest integration test build error)
**Lane**: tiny | **Change-type**: bug-fix
**Branch**: `fix/328-harvest-build-error`
**Implementing agent**: Sisyphus orchestrator (direct edit — 1-line fix, no delegation needed)

## Scope of changes

| File | Change |
|---|---|
| `internal/harvest/opencode_sqlite_integration_test.go` | 1 line: `q = sqlc.New(pgDB)` → `q := sqlc.New(pgDB)` (line 167) |
| `docs/evidence/328-pre-work-gate.md` | new evidence file |
| `docs/evidence/self-review-zfix-328-harvest-build-error.md` | this file |

**Production code**: zero changes (test-only fix).

## Root cause

The `q` variable was originally declared with `q := sqlc.New(pgDB)` (line 138) **inside a closure** (`successFn := func(...)` at line 137-150). Go's scoping rules confine `q` to that closure.

Line 167 (outside the closure, in the outer test function scope) attempted `q = sqlc.New(pgDB)` — an assignment to an undeclared variable. This is the source of `undefined: q` at line 167 and the cascading error at line 168 where `q.GetDocumentBySourcePath(...)` references it.

Fix: change `=` to `:=` so a new `q` is declared in the outer scope, identical naming to the closure's `q` (which is fine — they're in different scopes).

## self-review:response-shape

**N/A** — test fix, no API surface change.

## self-review:staged-files

```
$ git status
On branch fix/328-harvest-build-error
Changes to be committed:
	modified:   internal/harvest/opencode_sqlite_integration_test.go
	new file:   docs/evidence/328-pre-work-gate.md
	new file:   docs/evidence/self-review-zfix-328-harvest-build-error.md
```

- ✅ No `.opencode/` files
- ✅ No `package-lock.json`
- ✅ Only files semantically related to #328

## Validation ladder

| Layer | Required | Result |
|---|---|---|
| validate:quick (build + race -short) | yes | ✅ ALL PACKAGES PASS |
| `go build -tags=integration ./internal/harvest/...` | yes (this fixes that build) | ✅ no output (build clean) |
| `go test -tags=integration -short ./internal/harvest/...` | yes (this enables the test to RUN) | ✅ `ok internal/harvest 2.280s` |
| self-review:staged-files | yes | ✅ PASS |

## Evidence — before/after for `go test -tags=integration ./...`

**Before this PR**:
```
FAIL  github.com/nano-brain/nano-brain/internal/harvest [build failed]
FAIL  github.com/nano-brain/nano-brain/internal/search [build failed]
FAIL  github.com/nano-brain/nano-brain/internal/server/handlers (TestEventsIntegration_ReindexPublishesSequence, TestListWorkspacesE2E)
```

**After this PR (verified in this worktree)**:
```
ok    github.com/nano-brain/nano-brain/internal/harvest 2.280s    ← FIXED by this PR
ok    github.com/nano-brain/nano-brain/internal/search  1.041s    ← may have been fixed by another change; verify in #327
FAIL  github.com/nano-brain/nano-brain/internal/server/handlers  ← tracked by issue #326
```

So this PR unblocks the harvest integration suite. The search suite is also passing now (possibly resolved by another recent change — will verify under issue #327's worktree). Only `internal/server/handlers` failures remain.

## Backward compat

Zero. The variable is local to a single test function. Changing `=` to `:=` has no observable effect outside the test.

## Conclusion

Minimal 1-character fix (well, `=` → `:=` is 1 character added). Test now compiles and passes. This was a typo / copy-paste accident that escaped CI because integration tests are gated behind `-tags=integration` (not run on master CI by default).

Ready to merge.
