---
story_id: US-XXX
title: Story Title
status: planned          # planned | in-progress | blocked | complete
lane: tiny               # tiny | normal | high-risk
change_type: user-feature # user-feature | bug-fix | infrastructure | refactor | docs | dependency-bump
github_issue: ""         # e.g. nano-step/nano-brain#42 (REQUIRED unless tiny + no file changes)
openspec_change: ""      # e.g. openspec/changes/<name>/  (REQUIRED for normal + high-risk)
validation:
  unit: ""               # path to evidence file or "n/a"
  integration: ""        # path to evidence file or "n/a"
  e2e: ""                # path to evidence file (e.g. docs/evidence/smoke-e2e-<slug>.txt) or "n/a"
  platform: ""           # path or "n/a"
  release: ""            # path or "n/a" or "deferred"
review:
  verdict: pending       # pending | PASS | FAIL
  reviewer: ""           # agent name (must ≠ implementer)
  commit: ""             # sha of reviewed commit
pr:
  url: ""
  bot_rounds: 0          # max 3 (R29)
  overridden: false      # set true if [HARNESS-OVERRIDE] applied
---

# US-XXX Story Title

<!-- generated-by: harness-init v0.1.0 -->

## Status

planned

## GitHub Issue

`nano-step/nano-brain#N` — created at Feature Intake step 0. Required for every story
(skip only for tiny-lane changes that never touch remote).

## Lane

tiny | normal | high-risk

## OpenSpec Change

`openspec/changes/<name>/` — leave blank for tiny lane stories.

## Product Contract

Describe the behavior this story must make true.

## Relevant Product Docs

- `docs/product/...`

## Acceptance Criteria

- Criterion 1.
- Criterion 2.
- Criterion 3.

## Design Notes

- Commands:
- Queries:
- API:
- Tables:
- Domain rules:
- UI surfaces:

## Validation

| Layer | Expected proof |
| --- | --- |
| Unit | |
| Integration | |
| E2E | |
| Platform | |
| Release | |

## Change Type

One of: `user-feature` | `bug-fix` | `infrastructure` | `refactor` | `docs` | `dependency-bump` | `search-quality` | `index-schema`.
See `docs/HARNESS.md` § Change Types for E2E/Review gate implications.

## Testing Checklist

- [ ] User-flow test covers primary changed behavior (file: `tests/...`)
- [ ] Error/edge path tested — high-risk only (file: `tests/...`)
- [ ] E2E not applicable — reason: `___` (only valid for infra/refactor/docs/deps)
- [ ] Smoke test for non-user-facing change — command: `___` (only for infra/deps)
- [ ] All listed tests pass (output pasted in Evidence)

## Review

- Reviewer agent: (e.g. Oracle, review-work skill, name of fresh agent)
- Reviewer ≠ implementer: yes / no
- Verdict: `PASS` | `FAIL` | `PENDING`
- Date: YYYY-MM-DD
- Commit: `<sha>`

| Acceptance Criterion | Evidence | Status |
| --- | --- | --- |
| (copy from Acceptance Criteria above) | (test output, command, screenshot path) | ✓ / ✗ |

## PR Bot Review

- PR URL: `___`
- Bot rounds: `N` (max 3 before human escalation)
- Outstanding comments: none / [list]
- Bot approved: yes / no / overridden by [human, reason]

## Harness Delta

Document any harness updates made or proposed because of this story.

## Evidence

Add commands, reports, screenshots, or links after validation exists.
Paste FULL command output (not summary). Include exit codes.
For high-risk web changes, link screenshots in `docs/evidence/<change-name>/`.
