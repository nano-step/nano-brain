## ADDED Requirements

### Requirement: Pending counter invariant
The in-memory `pending` atomic counter in the embed queue SHALL equal the database count of chunks with `embed_status='pending'` at all times, modulo in-flight work currently being processed by workers.

#### Scenario: Initial state matches DB
- **WHEN** the queue starts and runs initial scan
- **THEN** `q.pending.Load()` equals `COUNT(chunks WHERE embed_status='pending')`

#### Scenario: Successful embed decrements counter and updates DB atomically per-chunk
- **WHEN** a chunk is successfully embedded and `MarkChunkEmbedded` succeeds
- **THEN** counter is decremented AND DB status changes to `'embedded'` in the same processChunk invocation

### Requirement: Channel-full retry preserves invariant
When `handleRetry` cannot re-enqueue a chunk because the queue channel is full, the system SHALL leave the chunk's DB status as `'pending'` AND SHALL NOT decrement the `pending` counter.

#### Scenario: Retry hits full channel
- **WHEN** a transient embed error triggers handleRetry AND the queue channel is at capacity
- **THEN** the chunk is NOT re-enqueued
- **AND** the chunk's DB `embed_status` remains `'pending'`
- **AND** `q.pending.Load()` is unchanged
- **AND** a WARN log records `"retry re-enqueue failed (channel full)"`

#### Scenario: Scan re-processes channel-full chunk
- **WHEN** the periodic scan runs after a channel-full retry drop
- **THEN** the scan re-enqueues the chunk via the standard `scanPending` path
- **AND** the invariant remains intact (pending counter accurately reflects backlog)

### Requirement: MarkChunkEmbedded failure preserves invariant
When `MarkChunkEmbedded` fails after a successful `InsertEmbedding`, the system SHALL NOT decrement the `pending` counter AND SHALL emit a status event so observers can detect the divergence.

#### Scenario: MarkChunkEmbedded returns error
- **WHEN** `InsertEmbedding` succeeds but the subsequent `MarkChunkEmbedded` call returns an error
- **THEN** `q.pending.Load()` is unchanged
- **AND** the chunk's DB `embed_status` remains `'pending'`
- **AND** an ERROR log is emitted with the chunk ID and underlying error
- **AND** `publishStatus()` is invoked to notify observers

#### Scenario: Scan retries embed after MarkChunkEmbedded failure
- **WHEN** the periodic scan runs after a MarkChunkEmbedded failure
- **THEN** the chunk is re-enqueued and re-processed
- **AND** `InsertEmbedding`'s `ON CONFLICT` clause prevents duplicate embedding rows
- **AND** if `MarkChunkEmbedded` succeeds on retry, status transitions to `'embedded'` and counter decrements
