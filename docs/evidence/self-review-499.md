# Self-Review: Issue #499 — enforce independent code review (R88)

Change type: **infra/process** · Lane: docs+tooling · Scope: harness rule + gate

## Actions Taken
- Added rule **R88**: the code-review pass MUST be performed by a separate
  spawned sub-agent; the implementing agent may NOT self-review/self-approve.
- Made gate 3.5 enforce it: resolve the review file for THIS story
  (`review-<sid>.md` / `review-<sid>-*.md`), require `Review Verdict: PASS`, and
  require a `Reviewer:` that is not self/author/implementer. A stale cross-story
  review no longer satisfies the gate; an unresolvable sid FAILs loudly.
- Updated docs (`HARNESS.md`, `HARNESS_GATES.md`) and both skill copies
  (`.claude` + `.opencode` `harness-check/SKILL.md`); installed the harness-check
  skill for Claude Code.

## Files Changed
- `scripts/harness-check.sh` — gate 3.5 rewrite (story-scoped review + reviewer independence)
- `docs/HARNESS.md` — R88 rule + rationale
- `docs/HARNESS_GATES.md` — gate 3.5 row updated (R27 + R88)
- `.claude/skills/harness-check/SKILL.md` (new), `.opencode/skills/harness-check/SKILL.md` — R88 in Core Rules + Rule IDs

## Findings Summary
- `/simplify` (4 agents): consolidated two `find` calls into one; kept the
  `[[ -n rg_sid ]]` guard (load-bearing) and `story_review=""` init (required
  under `set -u`).
- `/code-review` (independent, 3 finder angles + conventions): found **1
  critical** — prefix glob `review-${sid}*.md` false-matched (sid `49` →
  `review-497.md`). Fixed by matching the sid as a whole segment
  (`review-${sid}.md` / `review-${sid}-*.md`). Verified: sid `36` no longer
  matches `review-368.md`. Minor findings (word-boundary edge cases, detached
  HEAD, multiple `Reviewer:` lines) triaged as acceptable for an honesty
  guardrail; conventions clean.
- Known scope-limit: gates 3.10/3.12 use the same prefix-match style for their
  evidence globs (pre-existing); not changed here.

## Resolution Status
- All findings: **RESOLVED** or consciously deferred (noted above). No open critical/major.
- Independent review verdict recorded in `review-499.md` (Reviewer ≠ author).
