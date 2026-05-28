## Why

The embed pipeline (index → chunk → embed) is silently broken: watcher writes chunks to DB but never notifies the embed queue, reindex logs misleading `chunks_enqueued` counts without actually enqueueing, and there are zero file-level processing logs making debugging impossible. This bug has been hit twice before without a durable fix.

## What Changes

- **Watcher gains embed queue reference**: `Watcher` struct gets `eq *embed.Queue`; `writeChunks()` calls `eq.Enqueue(id)` for every new/updated chunk after upsert — no more silent pending accumulation waiting for 5-min scan.
- **Reindex handler actually enqueues**: `TriggerReindex` passes `eq *embed.Queue`; after `ResetEmbedStatusByCollection` it directly calls `eq.Enqueue()` for each reset chunk ID. `chunks_enqueued` count becomes accurate.
- **UpsertChunk resets embed_status on conflict**: SQL ON CONFLICT DO UPDATE now includes `embed_status = 'pending'` so re-indexed unchanged files are correctly re-queued.
- **File-level processing logs (INF)**: watcher logs `processing file path=<path> collection=<name>` before indexing each file; embed queue logs `embedding chunk chunk_id=<id> file=<path>` at INF level (not DEBUG) before calling embedder.
- **Embedder startup validation**: if `eq != nil` but ollama is unreachable at startup, log ERROR with exact URL instead of silently degrading.
- **No try-catch / error swallowing**: all errors surface via zerolog ERROR with full context; no silent drops.

## Capabilities

**New Capabilities**: none (this is a bug fix + observability improvement)

**Modified Capabilities**:
- `embed-pipeline` — watcher→queue wiring, reindex enqueue accuracy, UpsertChunk status reset
- `processing-logs` — new file-level and chunk-level INF logs throughout index/embed path

## Impact

- `internal/watcher/watcher.go` — add `eq` field, `WithEmbedQueue()` builder, enqueue in `writeChunks()`
- `internal/server/handlers/reindex.go` — add `eq *embed.Queue` param, enqueue after reset
- `internal/storage/queries/chunks.sql` + sqlc regeneration — UpsertChunk ON CONFLICT resets `embed_status`
- `internal/embed/queue.go` — promote chunk embed log from DEBUG → INF, add `file` field
- `cmd/nano-brain/main.go` — wire `fw.WithEmbedQueue(eq)` after queue creation
- No API contract changes, no new dependencies
