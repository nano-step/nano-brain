# embed-queue Specification

## Purpose
TBD - created by archiving change embed-status-index-and-inflight-dedup. Update Purpose after archive.
## Requirements
### Requirement: Partial composite index on chunks.embed_status
The `chunks` table SHALL have a partial composite index `idx_chunks_embed_status` covering `(embed_status, created_at)` filtered by `WHERE embed_status IN ('pending', 'embed_failed')`. The index SHALL be created via `CREATE INDEX CONCURRENTLY` so the migration does not lock the table.

#### Scenario: Embed worker pending scan uses index
- **GIVEN** the `chunks` table contains 100,000 rows with mixed `embed_status` values (95% `'embedded'`, 4% `'pending'`, 1% `'embed_failed'`)
- **WHEN** the embed queue worker executes `GetPendingChunksAllWorkspaces` (which filters `WHERE c.embed_status = 'pending'` and orders by `created_at`)
- **THEN** `EXPLAIN ANALYZE` of the query SHALL show `Index Scan using idx_chunks_embed_status` (not `Seq Scan on chunks`)
- **AND** the query result SHALL be ordered by `created_at ASC` without a separate `Sort` node in the plan

#### Scenario: Migration is reversible without lock
- **WHEN** `goose -dir migrations postgres "$DSN" up` runs against a database where the index does not yet exist
- **THEN** the migration SHALL complete without holding an `ACCESS EXCLUSIVE` lock on `chunks` (verified by absence of blocking on a concurrent `SELECT 1 FROM chunks LIMIT 1`)
- **WHEN** `goose -dir migrations postgres "$DSN" down` runs immediately after
- **THEN** the index SHALL be dropped cleanly with `DROP INDEX CONCURRENTLY IF EXISTS`

#### Scenario: Migration is idempotent
- **WHEN** the migration is applied a second time on a database that already has the index
- **THEN** the `CREATE INDEX CONCURRENTLY IF NOT EXISTS` SHALL be a no-op (no error)

### Requirement: In-flight dedup set in embed queue
The `embed.Queue` struct SHALL maintain an in-memory set of chunk IDs that are either currently in the channel buffer or being processed by a worker. `Enqueue` SHALL skip chunks already present in the set. `processChunk` SHALL remove the chunk from the set on every exit path (success, soft failure, hard failure, panic) via `defer`.

#### Scenario: Double Enqueue of same chunk produces single channel send
- **GIVEN** a Queue with channel capacity ≥ 2 and no active workers
- **WHEN** `q.Enqueue(id)` is called twice for the same chunk UUID `id` in succession
- **THEN** the channel SHALL contain exactly **1** message
- **AND** `q.pending.Load()` SHALL return `1`

#### Scenario: Different chunk IDs are not deduplicated
- **GIVEN** a Queue with channel capacity ≥ 2 and no active workers
- **WHEN** `q.Enqueue(id1)` then `q.Enqueue(id2)` are called with distinct UUIDs
- **THEN** the channel SHALL contain exactly **2** messages

#### Scenario: processChunk cleans inflight set even on panic
- **GIVEN** a Queue with an embedder that panics on `Embed()`
- **AND** a chunk ID `id` previously added to the in-flight set via Enqueue
- **WHEN** `q.processChunk(ctx, id)` runs and the embedder panics
- **THEN** the panic SHALL propagate (caller's responsibility to recover)
- **AND** after recovery, the chunk SHALL NOT be present in the in-flight set
- **AND** a subsequent `q.Enqueue(id)` SHALL succeed (channel send happens)

#### Scenario: Channel-full Enqueue does not leak inflight entry
- **GIVEN** a Queue with channel capacity = 1, channel already full
- **WHEN** `q.Enqueue(id)` is called with a new chunk UUID `id`
- **THEN** the channel send SHALL hit the default branch (capacity exceeded)
- **AND** `id` SHALL NOT remain in the in-flight set (so a future Enqueue can retry once channel drains)

#### Scenario: scanByStatus skips chunks already in flight
- **GIVEN** a Queue running with workers blocked on a slow embedder
- **AND** 100 chunks were enqueued from a prior tick, all currently in the channel or being processed
- **WHEN** `scanByStatus(ctx, "pending")` runs and fetches the same 100 chunk IDs from the database
- **THEN** the channel SHALL receive 0 additional sends for those IDs
- **AND** the in-flight set size SHALL remain 100

#### Scenario: New chunk after processChunk completion is enqueued
- **GIVEN** a Queue where `q.Enqueue(id)` was called and `processChunk(ctx, id)` ran to completion
- **WHEN** `q.Enqueue(id)` is called again with the same chunk ID
- **THEN** the second Enqueue SHALL succeed (channel send happens)
- **AND** the in-flight set SHALL contain `id`

### Requirement: Backpressure check precedes dedup check
The Enqueue method SHALL evaluate the backpressure threshold (`q.pending.Load() >= rejectionThreshold`) BEFORE consulting the in-flight set. A chunk SHALL NOT be added to the in-flight set if it is rejected by backpressure.

#### Scenario: Backpressure rejects without polluting in-flight set
- **GIVEN** a Queue where `q.pending.Load()` returns a value ≥ `rejectionThreshold` (50000)
- **WHEN** `q.Enqueue(chunkID)` is called with a new chunk UUID
- **THEN** the Enqueue SHALL return `false` immediately (backpressure)
- **AND** `chunkID` SHALL NOT be present in the in-flight set
- **AND** the channel SHALL NOT receive any new message

### Requirement: handleRetry preserves in-flight set on successful re-enqueue
The `handleRetry` method SHALL return `bool` indicating whether the chunk was successfully re-enqueued into `q.ch`. The processChunk method SHALL use a conditional defer that calls `inflight.Delete(chunkID)` only when handleRetry returns `false` (or when handleRetry was not called at all). This guarantees the in-flight set accurately reflects channel membership across retry cycles.

#### Scenario: Soft failure with successful retry re-enqueue keeps chunk in-flight
- **GIVEN** a Queue with an embedder that returns a soft (retriable) error
- **AND** retry count for `chunkID` is below `maxRetries`
- **AND** the channel has spare capacity
- **WHEN** `processChunk(ctx, chunkID)` runs and triggers the soft-failure path
- **THEN** `handleRetry` SHALL re-enqueue `chunkID` directly into `q.ch` and return `true`
- **AND** processChunk's conditional defer SHALL NOT delete `chunkID` from the in-flight set
- **AND** after processChunk returns, `chunkID` SHALL still be present in the in-flight set
- **AND** the channel SHALL contain `chunkID` for the next worker to pick up

#### Scenario: Soft failure with channel-full re-enqueue removes chunk from in-flight
- **GIVEN** a Queue with an embedder that returns a soft error
- **AND** retry count for `chunkID` is below `maxRetries`
- **AND** the channel is full (cannot accept the re-enqueue)
- **WHEN** `processChunk(ctx, chunkID)` runs and triggers handleRetry
- **THEN** `handleRetry` SHALL log a warning and return `false`
- **AND** processChunk's conditional defer SHALL delete `chunkID` from the in-flight set
- **AND** the next `scanByStatus` cycle SHALL be able to re-enqueue this chunk from the database

#### Scenario: Max retries reached marks failed and removes from in-flight
- **GIVEN** a Queue where retry count for `chunkID` has reached `maxRetries`
- **WHEN** `handleRetry(ctx, chunkID, workspaceHash)` is called
- **THEN** `MarkChunkEmbedFailed` SHALL be invoked
- **AND** `handleRetry` SHALL return `false`
- **AND** processChunk's conditional defer SHALL delete `chunkID` from the in-flight set

