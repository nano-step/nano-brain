# Independent Review: harness-recalibration (R88)

Date: 2026-07-06
Reviewer: harness-reviewer (spawned oh-my-claudecode:code-reviewer sub-agent — independent of the implementing agent)
Scope: commit 9162b97 + review-fix working tree (gate recalibration R90–R92,
pre-merge hook, docs de-OpenCode, baseline seeding)

## Round 1 findings (verdict: FAIL)

| # | Severity | File | Finding | Resolution |
|---|----------|------|---------|------------|
| 1 | major | scripts/harness-check.sh (gate 1.1) | `comm \| head -5` SIGPIPE-crashes checker (exit 141) under pipefail on large overlap — reproduced with 100k lines | FIXED: `\|\| true` |
| 2 | major | .claude/hooks/harness-pre-merge-hook.sh + gate 3.5 | Hook false-blocked `gh pr create` on missing review evidence, contradicting the documented PR-first flow — reproduced live on this branch | FIXED: under HARNESS_FAST, 3.5 SKIPs unless explicit `Review Verdict: FAIL` |
| 3 | major | scripts/harness-check.sh (change-type) | `^docs/` directory pattern classified `docs/foo.go` as docs-only → code gates skipped (gate-evasion vector) | FIXED: extension-only classification |
| 4 | major | scripts/harness-check.sh (gate 3.3) | Baseline growable in-PR — a NEW failure could be snuck past by editing docs/harness-baseline.txt in the same PR (honor-system only) | FIXED: shrink-only enforced via `git diff origin/master...HEAD`; seeding exempt; runs under HARNESS_FAST |
| 5 | minor | .claude/hooks/harness-pre-merge-hook.sh | Bare `[HARNESS-OVERRIDE]` substring bypassed the hook with zero justification | FIXED: requires `[HARNESS-OVERRIDE]: <reason ≥20 chars>` (same bar as gate 3.6) |
| 6 | minor | gate 3.11 | commits ≠ push cycles (R29 semantics proxy) | DEFERRED — pre-existing semantics, unchanged by this PR |
| 7 | info | hook | Total fail-open posture on checker crash / missing jq | ACCEPTED — by design for a safety net |

## Round 2 (re-review of fixes)

All five fixes verified empirically by the reviewer under bash (the script's
interpreter): SIGPIPE repro now exits 0; the fixed hook returns exit 0 on this
branch (was exit 2); change-type matrix verified (`docs/foo.go`→code,
`foo_test.go` alone→test-only, mixed→code); shrink-only grep verified against
`+++` header, `+#` comments, pure-shrink, and the initial-seeding exemption;
override bypass matrix verified (bare→gated, short reason→gated, full→bypass).
The new gate-3.3 flaky guard (isolation re-run of suspect packages) was
reviewed as new code: acceptable, like-for-like with baseline seeding, no
hook-timeout risk (unreachable under HARNESS_FAST).

No new blocking issues introduced.

Review Verdict: PASS
