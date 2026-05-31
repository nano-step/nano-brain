# Fix Embed Queue ErrNoRows Race

## Issue
[#259 — fix(embed-queue): 'sql: no rows in result set' when fetching enqueued chunk](https://github.com/nano-step/nano-brain/issues/259)

## Lane
normal (2 risk flags: existing-behavior + weak-proof).

## Why
The embed queue worker fails to fetch a chunk by ID when the chunk row has been deleted between enqueue and pop. This happens in three production scenarios:

1. Document re-upserted with new content → old chunks cascade-deleted
2. Workspace deleted → FK CASCADE on `chunks.workspace_hash` removes chunk rows
3. `cleanup-orphan-workspaces` command sweep (issue #252 fix) removes orphan chunks

The worker treats this as a hard error and emits `ERR failed to fetch chunk` per occurrence. Multiple deletions can produce dozens of errors per second, drowning out genuine failures and wasting log volume. The DB state is correct — only the log noise is wrong.

## Desired Outcome
Embed worker treats `sql.ErrNoRows` on `GetChunkByID` as a benign skip (chunk was deleted between enqueue and pop). No ERROR log, no retry, just decrement pending counter and continue.

All other DB errors continue to log ERROR (no behavior change for genuine failures).

## Constraints
- ONE place to change: `internal/embed/queue.go` (1 line + 1 helper branch)
- Test must exercise the race deterministically (enqueue → delete chunk → pop → assert no ERROR log)
- No change to public API, config, schema, or other packages

## Out of Scope
- Removing the pre-existing race entirely (would require enqueue inside same TX as chunk insert — bigger refactor; defer)
- Retry on transient errors (existing retry logic unchanged)
- Other queue races (e.g., embedding provider returning 4xx — covered by separate hardening)

## Acceptance Criteria
1. **Benign skip on ErrNoRows**: When `GetChunkByID` returns `sql.ErrNoRows`, the embed worker emits a DEBUG log (with `chunk_id`) and continues to the next item. Pending counter is still decremented.
2. **ERROR log for real DB errors**: A connection drop, timeout, or any non-ErrNoRows error continues to emit ERROR log with full error context.
3. **Integration test**: Real PostgreSQL test that:
   - Inserts a workspace + document + chunk row
   - Enqueues the chunk ID into the embed queue
   - Deletes the chunk row via DELETE SQL
   - Lets the worker pop the ID
   - Asserts: 0 ERROR log entries match `failed to fetch chunk`, 1 DEBUG entry, pending counter decremented to 0
4. **No regression**: existing `internal/embed/...` tests pass unchanged.
5. **Validate ladder**: `validate:quick` + `test:integration` green.

## Risk Flags
- [x] Existing behavior change (error → debug for one path)
- [x] Weak proof (no current test for race)

2 flags + 0 hard gates → **normal lane** confirmed.
