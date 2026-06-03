# Self-Review — Fix MCP wake_up Collections Filter (#356)

**Branch:** `fix/wake-up-mcp-collections`
**Issue:** [#356](https://github.com/nano-step/nano-brain/issues/356)
**Lane:** normal (bug-fix)
**TRACE_SPEC Tier:** 2 (touches MCP tool surface, observable to clients)

## Actions Taken

1. Reproduced bug — confirmed `memory_wake_up` MCP tool returns `recent_memories: []` while HTTP `/api/v1/wake-up` returns full list, on the same workspace hash (`7f44…`, 71 docs in `memory`).
2. Traced root cause via `grep` + `read` — `internal/mcp/tools.go:818` passed `Collections: nil` to `RecentDocuments` while `internal/server/handlers/wakeup.go:91` passed `Collections: ["memory","session-summary"]`.
3. Inspected the SQL query (`internal/storage/queries/wakeup.sql` → generated `wakeup.sql.go`) — confirmed `AND collection = ANY($3::text[])` is required since PR #340 (fix #338).
4. Drafted OpenSpec proposal + design + tasks + spec under `openspec/changes/fix-mcp-wake-up-collections-filter/`. `openspec validate --strict` → PASS.
5. Implemented 3-line code edit in `tools.go`. Added 1-line regression-warning comment referencing #356/#338/#340.
6. Added integration test `TestMemoryWakeUp_OnlyReturnsMemoryAndSessionSummaryDocs` in `internal/mcp/tools_wakeup_integration_test.go` — sets up 6 docs across 3 collections, asserts only `memory` + `session-summary` are returned.
7. Sanity check: reverted the fix, re-ran test → FAILED with `recent_memories len = 0, want 3` (matches production symptom). Re-applied fix → PASSED.
8. Ran full validation ladder.

## Files Changed

| File | Change | Lines |
|------|--------|-------|
| `internal/mcp/tools.go` | Add `Collections: ["memory","session-summary"]` to `RecentDocumentsParams` struct literal in `registerMemoryWakeUp` + 1-line regression-warning comment | +2 |
| `internal/mcp/tools_wakeup_integration_test.go` | New integration test (build tag `integration`) | +112 (new file) |
| `CHANGELOG.md` | `[Unreleased] ### Fixed` entry | +3 |
| `openspec/changes/fix-mcp-wake-up-collections-filter/proposal.md` | OpenSpec proposal | +45 (new) |
| `openspec/changes/fix-mcp-wake-up-collections-filter/design.md` | Design doc | +55 (new) |
| `openspec/changes/fix-mcp-wake-up-collections-filter/tasks.md` | Tasks list | +20 (new) |
| `openspec/changes/fix-mcp-wake-up-collections-filter/specs/mcp/spec.md` | Spec capability | +25 (new) |

**Touched packages:** `internal/mcp` only.
**Not touched:** `internal/storage`, `internal/server/handlers`, migrations, schema, sqlc-generated code.

## Findings Summary

| Severity | Category | Status |
|----------|----------|--------|
| critical | regression bug producing wrong observable behaviour for MCP clients (always-empty `recent_memories`) | RESOLVED — code fix + integration test |
| major | divergence between HTTP and MCP handlers for same logical operation | RESOLVED — note added in code comment; structural fix (shared service layer) tracked as out-of-scope in `proposal.md` |
| minor | no project-wide regression test covers HTTP vs MCP parity for wake-up | PARTIAL — new test covers MCP path with same setup as HTTP integration tests would; full parity harness deferred |
| minor | pre-existing build failure in `internal/server/handlers/reindex_integration_test.go` (`UpsertWorkspaceParams.RootPath` field renamed to `Path`) on master | OPEN — not introduced by this PR, not in scope to fix |

## Resolution Status

- **Code change:** APPLIED and PASS validation ladder.
- **Regression test:** ADDED and verified to FAIL without the fix.
- **OpenSpec proposal:** VALIDATED (`openspec validate fix-mcp-wake-up-collections-filter --strict` → ok).
- **Validation ladder (validate:quick):** PASS — `go build ./... && go test -race -short ./...` exit 0.
- **Validation ladder (test:integration on `./internal/mcp/...`):** PASS — 2.016s.
- **Validation ladder (test:integration on `./internal/server/handlers/...`):** PRE-EXISTING FAILURE on master (`reindex_integration_test.go:40` uses removed field). Not introduced by this PR. Tracked separately.
- **smoke:e2e:** SKIPPED — MCP tool input schema + output shape unchanged; behavioural correctness covered by integration test.

## Evidence Pointers

- Commit: `bdda167` — `fix(mcp): pass Collections filter to RecentDocuments in memory_wake_up (#356)`
- Sanity-revert proof: see commit message body — without fix `recent_memories len = 0, want 3`; with fix PASS.
- Issue: https://github.com/nano-step/nano-brain/issues/356
- OpenSpec change: `openspec/changes/fix-mcp-wake-up-collections-filter/`
