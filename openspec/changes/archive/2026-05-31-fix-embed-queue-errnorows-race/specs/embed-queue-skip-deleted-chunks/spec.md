# embed-queue-skip-deleted-chunks Delta — New Capability

## ADDED Requirements

### Requirement: Embed worker treats deleted chunk as benign skip

The embed queue worker SHALL treat `sql.ErrNoRows` from `GetChunkByID` as a benign skip condition, not an error. Chunks may be deleted between enqueue and pop via:

1. Document re-upsert (old chunks cascade-deleted)
2. Workspace deletion (FK CASCADE on `chunks.workspace_hash`)
3. `cleanup-orphan-workspaces` sweep

In all three cases, the absence of the chunk is the correct DB state — the worker has nothing to do.

When `sql.ErrNoRows` is detected:
- A DEBUG-level log entry is emitted with the `chunk_id` for diagnostic purposes
- The pending counter is decremented
- The worker continues to the next item without retry

All other DB errors (connection drop, timeout, encoding error, etc.) continue to emit ERROR-level logs with full error context, preserving existing behavior.

#### Scenario: Chunk deleted between enqueue and worker pop

- **GIVEN** a chunk `c1` is enqueued for embedding
- **AND** the chunk row is deleted from PostgreSQL (e.g., via document re-upsert) before the worker pops the ID
- **WHEN** the embed worker calls `GetChunkByID(c1)`
- **THEN** the call returns `sql.ErrNoRows`
- **AND** the worker emits a DEBUG log entry containing `chunk_id=c1` and the message "embed-queue: chunk no longer exists (likely cascade-deleted), skipping"
- **AND** the worker does NOT emit an ERROR log
- **AND** the pending counter is decremented by 1
- **AND** the worker continues to the next item

#### Scenario: Real DB error still logs ERROR

- **GIVEN** an embed worker is processing items
- **WHEN** `GetChunkByID` returns an error other than `sql.ErrNoRows` (e.g., connection drop, query timeout, malformed UUID)
- **THEN** the worker emits an ERROR log entry with the chunk_id AND the underlying error wrapped
- **AND** the pending counter is decremented
- **AND** the worker continues
