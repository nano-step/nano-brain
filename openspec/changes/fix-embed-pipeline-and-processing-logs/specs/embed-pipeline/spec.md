## ADDED Requirements

### Requirement: Watcher enqueues chunks immediately after writeChunks
Every chunk ID returned by `UpsertChunk` MUST be passed to `eq.Enqueue(id)` immediately after `writeChunks()` completes, if `eq != nil`.

#### Scenario: Watcher processes a changed file and immediately enqueues chunks
- Given the watcher detects a dirty directory and processes files
- When `writeChunks()` upserts new or changed chunks
- Then each chunk ID is passed to `eq.Enqueue()` without waiting for the 5-minute scan

#### Scenario: Embed disabled — watcher skips enqueue silently
- Given `eq` is nil (embed disabled)
- When watcher processes files
- Then no error occurs and no enqueue is attempted

### Requirement: Reindex enqueues actual chunk IDs
`TriggerReindex` MUST use a query that returns chunk IDs (`RETURNING id`) and call `eq.Enqueue(id)` for each. The response field `chunks_enqueued` MUST equal the number of IDs actually sent to the queue.

#### Scenario: Reindex endpoint returns accurate enqueued count
- Given a workspace with 241 pending chunks after reset
- When `POST /api/v1/reindex` is called
- Then the response `chunks_enqueued` equals the number of IDs actually sent to the queue
- And embedding log lines appear within 10 seconds

#### Scenario: Embed disabled — reindex returns zero enqueued
- Given `eq` is nil
- When `POST /api/v1/reindex` is called
- Then the response returns `chunks_enqueued=0` with no error

### Requirement: UpsertChunk resets embed_status on conflict
The `UpsertChunk` SQL ON CONFLICT DO UPDATE clause MUST include `embed_status = 'pending'`.

#### Scenario: Re-indexed unchanged file is re-embedded
- Given a file whose content has not changed (same content_hash already embedded)
- When reindex triggers watcher re-upsert
- Then `embed_status` is reset to `pending` and chunk is re-enqueued

### Requirement: No silent error drops
All errors in the index/embed path MUST be logged at ERROR level. No error may be silently discarded.

#### Scenario: DB error during writeChunks is logged
- Given a transient DB error during UpsertChunk
- When writeChunks encounters the error
- Then an ERR log line is emitted with file path and error text
