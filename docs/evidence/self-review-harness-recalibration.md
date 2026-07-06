# Self-Review: harness-recalibration

Date: 2026-07-06
Reviewer: author (self-review pass — independent review is separate, see
`review-harness-recalibration.md` per R88)

## Findings

| # | Severity | File | Description | Status |
|---|----------|------|-------------|--------|
| 1 | major | .claude/hooks/harness-pre-merge-hook.sh | Hook blocked on script exit ≠ 0, but runner contract exit 2 = all-SKIP (not a failure) — would false-block | FIXED (block only on `[FAIL]` lines) |
| 2 | minor | scripts/harness-check.sh | Gate 1.1 FAIL message missing a space before `(R90` when overlap list ends with tr-space | ACCEPTED (cosmetic) |
| 3 | minor | scripts/harness-check.sh | Gate 3.10 `*self-review*<slug>*` glob is fuzzy — matched a stale story-362 file for branch slug `harness` | DEFERRED (pre-existing looseness, unchanged by this PR) |
| 4 | info | .claude/hooks/harness-pre-merge-hook.sh | Substring match intercepts any Bash command containing "gh pr create" (e.g. echo tests) | ACCEPTED (false positive cost = clear block message + R7 override exists) |

## Verification run

- `bash -n scripts/harness-check.sh` → SYNTAX OK
- Hook live-tested in-session: non-matching command exit 0; `gh pr create`
  blocked with gate output (exit 2, confirmed via harness PreToolUse
  interception); `[HARNESS-OVERRIDE]` bypassed with note.
- `./scripts/harness-check.sh next-ready|retro` → correct legitimate FAILs,
  no crashes.
- `HARNESS_FAST=1 pre-merge` → 3.1–3.4 SKIP, evidence gates evaluated.

## Summary

- Major: 1 found, 1 fixed
- Minor: 2 found, 0 fixed (1 cosmetic accepted, 1 pre-existing deferred)

## Gemini Verification Triage

PR #537 inline review comments, verified against code before fixing (R31).
All five are mechanical shell-hardening suggestions — cheaper to apply than
to litigate; none was an observed runtime failure.

| Finding | File | Gemini Severity | Verified Verdict | Action |
|---------|------|----------------|-----------------|--------|
| PR#537 echo baseline_added \| head -3 SIGPIPE under set -e | scripts/harness-check.sh | High | VALID:high — set -euo pipefail is active; argument-position substitution makes set -e moot but 141 is possible under pipefail — fixed in commit see-below | printf + \|\| true |
| PR#537 hook grep \| head -20 SIGPIPE | .claude/hooks/harness-pre-merge-hook.sh | Medium | VALID:medium — hook has no set -e so harmless, hardened anyway | \|\| true appended |
| PR#537 echo option injection ($branch_files/$pr_files) | scripts/harness-check.sh | Medium | VALID:medium — filename starting with '-' would be eaten by echo | printf '%s\n' |
| PR#537 CRLF in baseline breaks comm | scripts/harness-check.sh | Medium | VALID:medium — Windows checkout with autocrlf would mismatch | tr -d '\r' added |
| PR#537 echo option injection ($ct_files) | scripts/harness-check.sh | Medium | VALID:medium — same class as above | printf '%s\n' |
