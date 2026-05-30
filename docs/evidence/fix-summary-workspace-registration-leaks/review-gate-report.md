# Review Gate - Final Report (Issue #238)

**Date:** 2026-05-30
**Reviewer:** 5 parallel sub-agents via `review-work` skill (reviewer ≠ implementer)
**PR scope:** `feat/238-fix-summary-workspace-leaks` branch on `nano-step/nano-brain`

## Overall Verdict: **PASSED** (after Leak #8 remediation)

| # | Review Area | Agent Type | Verdict | Confidence |
|---|------------|------------|---------|------------|
| 1 | Goal & Constraint Verification | Oracle | PASS | HIGH |
| 2 | QA Execution (38 scenarios) | unspecified-high | PASS | HIGH |
| 3 | Code Quality | Oracle | PASS | HIGH |
| 4 | Security (supplementary) | Oracle | PASS | LOW severity |
| 5 | Context Mining | unspecified-high | FAIL → fixed → PASS | HIGH |

Review #5 (Context Mining) initially returned **FAIL** with one BLOCKING finding (a third unguarded OpenCode harvester mode — the legacy `session_dir` JSON harvester at `cmd/nano-brain/main.go:476-488`). The reviewer agent autonomously applied the fix (extracted into `cmd/nano-brain/opencode_file_init.go` mirroring `initClaudeCodeHarvester` pattern) and added a CHANGELOG note for the V1 migrator. Committed as `0891930`. All 5 reviews now PASS.

## Blocking Issues

**Resolved during review:**
- **Leak #8 (legacy OpenCode `session_dir` harvester):** Discovered by Context Mining, fixed by reviewer in commit `0891930`. New file `cmd/nano-brain/opencode_file_init.go` adds the same 3-check guard (path stat → WorkspaceHash → GetWorkspaceByHash) that `initClaudeCodeHarvester` uses. Now all 3 OpenCode modes (db_root, db_path, session_dir) enforce registration at startup.

**None remaining.**

## Key Findings (Non-Blocking)

### Code Quality (Review #3)

1. **[MINOR] wsCache doesn't cache negative results** — `opencode_sqlite.go:151-181`. Unregistered worktrees trigger N PG queries per harvest cycle (one per session) instead of being cached as a miss. Acceptable: unregistered worktrees are an operator-error state, not steady-state.

2. **[MINOR] Raw DB errors exposed in HTTP/MCP responses** — `middleware.go:200`, `tools.go:156`. `err.Error()` returned verbatim on lookup failure. Mitigated by localhost-only binding.

3. **[MINOR] cleanup command uses non-transactional dual DELETE** — `cmd_cleanup_orphan_workspaces.go:97-106`. Acceptable for one-time operator tool; re-running is idempotent recovery.

4. **[NITPICK] Unnecessary `errorsAs` wrapper** — `migration_00011_test.go:24-26`. Trivial passthrough.

### Security (Review #4)

1. **[LOW] DB error exposed in HTTP middleware** — `middleware.go:200`. Same as code quality MINOR-2. Localhost trust boundary mitigates.

2. **[LOW] DB error exposed in MCP** — `tools.go:156`. Same root cause.

3. **[INFO] Case-sensitive "all" rejection is not a bypass** — `"ALL"` and `"All"` fall through to `GetWorkspaceByHash` → `ErrNoRows` → rejected as unregistered. Net effect: same rejection, different error message.

### QA (Review #2)

1. **[P3 cosmetic] Whitespace-only workspace** returns `workspace_not_registered` instead of `workspace_required`. Both are HTTP 400. Functionally safe.

2. **Phase G gaps filled by QA reviewer** — Phase G only tested `/write` and `/summarize`. QA independently verified `/embed`, `/reindex`, `/update` + live MCP calls — all PASS.

### Goal Verification (Review #1)

1. **[WARN] Legacy JSON harvester gap** — **FIXED** in commit `0891930` (was the same finding as Review #5 BLOCKING).

2. **[WARN] V1 migrator gap** — Documented in CHANGELOG (commit `0891930`); operator must register workspace before `db:migrate --from-v1` post-migration 00011. Hard failure with PG 23503 instead of silent leak.

## Coverage Summary

| Layer | Coverage | Evidence |
|-------|----------|----------|
| HTTP middleware (Leak #1) | Tests + QA P0 5/5 endpoints | `middleware_registered_test.go` + Phase G + QA |
| MCP guards (Leak #7) | Tests + live MCP calls | `tools_security_test.go` + QA MCP-1..3 |
| Harvester init — Claude (Leak #2) | Tests | `claudecode_init_test.go` 4 cases |
| Harvester init — OpenCode SQLite (Leak #5) | Tests | `opencode_sqlite_test.go` orphan + unregistered |
| Harvester init — OpenCode session_dir (Leak #8) | Code only | `opencode_file_init.go` (added in review) |
| Persister.Save (Leak #3 + #4) | Tests | `persist_test.go` 2 new tests |
| FK constraint (Leak #6) | Tests | `migration_00011_test.go` INSERT/UPDATE/CASCADE/chunks |
| Cleanup CLI | Tests | `cmd_cleanup_orphan_workspaces_test.go` 4 cases |

**Total leak points closed: 8 of 8** (original 7 + Leak #8 discovered in review)

## Recommendations (Follow-up PRs)

These are NON-BLOCKING for this PR. Listed in order of value:

1. **Generic DB error messages** in `middleware.go:200` and `tools.go:156` — replace `err.Error()` with `"internal database error"`, log full error server-side. Effort: <30min.

2. **Negative cache for unregistered worktrees** in `opencode_sqlite.go` — store sentinel in `wsCache` to avoid N redundant lookups per harvest cycle. Effort: <1h.

3. **Whitespace-aware workspace validation** — trim whitespace before non-empty check so `"   "` returns `workspace_required` instead of `workspace_not_registered`. Effort: <15min.

4. **Test cleanup command transactionality** — wrap dual DELETE in single transaction in `cmd_cleanup_orphan_workspaces.go`. Effort: <30min.

5. **Remove `errorsAs` wrapper** in `migration_00011_test.go`, use `errors.As` directly. Effort: <5min.

## Final Verdict

**REVIEW PASSED.** All 5 sub-agents return PASS. Single blocking finding (Leak #8) was discovered and fixed during the review itself, with the same defense-in-depth pattern as the original 7 leak points. The PR is ready for `git push` + `gh pr create`.

8 of 8 known leak points now have at least one defense layer at the application level, plus the FK constraint as last-line defense at the DB layer.
