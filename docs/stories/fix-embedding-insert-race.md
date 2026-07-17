---
story_id: US-600
title: Prevent stale embedding insert FK violations
status: in-progress
lane: normal
change_type: bug-fix
github_issue: nano-step/nano-brain#600
openspec_change: openspec/changes/fix-embedding-insert-race/
validation:
  unit: pass
  integration: pass-with-baseline-failures
  e2e: pass
review:
  verdict: pass
  reviewer: independent-five-lane
pr:
  url: ""
  bot_rounds: 0
---

# US-600 Prevent stale embedding insert FK violations

## Product Contract

Embedding persistence must skip a deleted source chunk without causing a PostgreSQL foreign-key error, and a direct embedding batch must continue after that skipped item.

## Acceptance Criteria

- A deleted source chunk causes `InsertEmbedding` to return `sql.ErrNoRows`, not SQLSTATE 23503.
- The queue finishes a stale job without retrying or error-level logging.
- The direct endpoint continues later chunks in the same batch after a stale result.

## Validation

| Layer | Expected proof |
| --- | --- |
| Unit | Queue and endpoint stale-result tests |
| Integration | Isolated PostgreSQL stale chunk and lock-race tests |
| E2E | Direct embedding endpoint against a local deterministic embed provider |

## Evidence

- OpenSpec: `docs/evidence/fix-embedding-insert-race/openspec-gap-analysis.md`
- Validation: `docs/evidence/fix-embedding-insert-race/validation.md`
- Smoke E2E: `docs/evidence/fix-embedding-insert-race/smoke-e2e-fix-embedding-insert-race.md`
- Independent review: `docs/evidence/fix-embedding-insert-race/independent-review.md`
