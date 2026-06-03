# Smoke Test Evidence — async-pr-review gate (#362)

Date: 2026-06-03
Branch: `feat/362-harness-async-pr-review-gate`
Target: PR #359 (real open PR with Gemini review)

## Test 1: Direct invocation, FAIL case (no PR on branch)

Command: `./scripts/check-pr-review.sh async-pr-review` (on fresh branch)

Output:
```
─ ASYNC-PR-REVIEW checks
[FAIL] 3.5.1 No open PR found on current branch

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
[FAIL] Gate: async-pr-review — FAIL

No PR found on current branch. Push the branch and open a PR:
  gh pr create --base master --fill
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

Exit code: 1 (FAIL — matches runner contract).

## Test 2: JSON contract validation

Command: `./scripts/check-pr-review.sh async-pr-review --json`

Output (parsed):
```json
{
  "gate": "async-pr-review",
  "status": "FAIL",
  "checks": [
    {"id": "3.5.1", "name": "No open PR found on current branch", "status": "FAIL"}
  ],
  "next_gate": null,
  "rule_ids_violated": [],
  "instructions_for_agent": "No PR found on current branch. Push the branch and open a PR:\n  gh pr create --base master --fill"
}
```

Contract validation (`./scripts/harness-check.sh validate`):
```
[PASS] Gate 'async-pr-review': contract valid (status=FAIL)
```

Required fields present: `gate`, `status`, `checks`, `rule_ids_violated`. ✅

## Test 3: Real PR — PR #359 (open with Gemini review)

Switched to branch `feat/358-mcp-search-response-pagination` (has open PR #359 with 1 Gemini review event).

Command: `./scripts/check-pr-review.sh async-pr-review --json`

Output (parsed):
```json
{
  "gate": "async-pr-review",
  "status": "FAIL",
  "checks": [
    {"id": "3.5.1", "name": "PR #359 exists and is OPEN", "status": "PASS"},
    {"id": "3.5.2", "name": "No merge conflicts", "status": "PASS"},
    {"id": "3.5.3", "name": "Bot review posted (1 review event(s))", "status": "PASS"},
    {"id": "3.5.4", "name": "1 bot review(s) but no triage file", "status": "FAIL"}
  ],
  "next_gate": null,
  "rule_ids_violated": [],
  "instructions_for_agent": "Bot posted 1 review event(s) but no triage file found under docs/evidence/.\nRun the code-review skill to generate a triage table for PR #359:\n  Identify each comment as VALID:{critical|high|medium|low} or INVALID:<reason>\n  For VALID:critical/high, fix and append 'fixed in commit <sha>' to the row.\nThen re-run this gate."
}
```

This is exactly the gap that motivated #362: PR #359 has a Gemini review but no triage file. The gate correctly catches this and provides actionable instructions.

## Test 4: Runner contract validation across all gates

Command: `./scripts/harness-check.sh validate`

Result:
- `[PASS] Gate 'async-pr-review': contract valid (status=FAIL)` ← new gate
- `[PASS] Gate 'post-merge-npm-release': contract valid (status=FAIL)`
- `[PASS] Gate 'next-ready': contract valid (status=FAIL)`
- Other gates show pre-existing `no JSON output (or timed out after 10s)` — confirmed pre-existing by re-running on master baseline (same 4 FAILs before this change).

No regression introduced.

## Test 5: harness-check.sh dispatch verification

Command: `./scripts/harness-check.sh async-pr-review --json`

Result: dispatches via `exec` to `scripts/check-pr-review.sh`, exits with correct code.

`get_next_gate()` verified: `pre-merge` → `async-pr-review` → `post-merge`.
