# OpenSpec Gap Analysis — fix-embedding-insert-race

Date: 2026-07-17
Issue: #600
Lane: normal
Change type: bug-fix

## Readiness

- Existing owning change: none found.
- Proposal, design, specification, and tasks: complete.
- Strict validation: `Change 'fix-embedding-insert-race' is valid`.

## Resolved Gaps

| Gap | Resolution |
| --- | --- |
| Could a Go-side recheck prevent the race? | No. The design uses one SQL statement with a `FOR KEY SHARE` parent-row guard. |
| Which writers need handling? | Queue and direct HTTP endpoint are the only production callers. |
| Could the lock block a provider request? | No. It is acquired only for final persistence, after embedding completes. |
| Is a database migration necessary? | No; this changes query behavior only. |

## Independent Design Verdict

PASS — `FOR KEY SHARE` makes deletion and final vector persistence serializable at the relevant row without holding a lock over the provider call. A prior deletion produces `sql.ErrNoRows`; a later deletion waits and then cascades the completed embedding.

## Harness Note

`harness-check.sh` currently accepts `in-progress` but does not implement the skill-documented `--change` option. This is recorded here only; it is outside issue #600's product scope.
