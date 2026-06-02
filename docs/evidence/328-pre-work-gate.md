# PRE-WORK gate — Issue #328

**Date**: 2026-06-02
**Issue**: #328 (internal/harvest integration test build error — `undefined: q`)
**Lane**: tiny (1 file, 1-line fix)
**Change-type**: bug-fix
**Branch**: `fix/328-harvest-build-error`

## Gate output

```
[FAIL] 1.1 Open PRs still pending (1)   ← unrelated PR #321
[PASS] 1.2 No active OpenSpec changes
[PASS] 1.3 Issue #328 exists (state: OPEN)
[PASS] 1.4 master is up-to-date (at e03d8cb)
[PASS] 1.5 Validation ladder passes
[PASS] 1.6 Branch 'fix/328-harvest-build-error' is based on master
```

`[HARNESS-OVERRIDE]` 1.1: same as prior PRs — unrelated PR #321.

## Lane justification (tiny)

- 1 file changed: `internal/harvest/opencode_sqlite_integration_test.go`
- 1 line edit: `q = sqlc.New(pgDB)` → `q := sqlc.New(pgDB)` (line 167)
- 0 schema/API/auth changes
- Test-only file (build-tag `//go:build integration`)
- Risk flags: 0

## Skip justifications

- OpenSpec proposal: SKIP (tiny lane)
- smoke:e2e: SKIP — test-only fix, no runtime behavior change
- Review gate: ⚠️ self-verify only — test-only fix in integration suite

## Root cause

Variable shadowing. Inside an inner closure at line 137-150, `q := sqlc.New(pgDB)` was declared in a new scope. At line 167 (outer test scope), `q = sqlc.New(pgDB)` tried to reassign a variable that didn't exist in this scope.

Fix: change `=` to `:=` to declare a fresh `q` in the outer scope.
