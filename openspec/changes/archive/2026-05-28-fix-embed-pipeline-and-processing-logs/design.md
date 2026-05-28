## Context

Three separate code paths all write chunks to the DB but none notifies the embed queue:

1. **Watcher** (`writeChunks`) — upserts chunks, returns. No `eq` reference.
2. **Reindex handler** (`TriggerReindex`) — resets `embed_status='pending'` via SQL, logs `chunks_enqueued=N` (misleading), calls `TriggerRescanByName` (marks watcher dir dirty). No `eq.Enqueue()`.
3. **Harvest runner** — writes memory notes as chunks. Has `eq` param but path not verified.

The embed queue only discovers pending chunks via its 5-minute `scanPending` ticker. Until then, chunks sit in the DB unprocessed with no log output.

Additional bug: `UpsertChunk` ON CONFLICT clause omits `embed_status`, so re-indexed unchanged files keep `embed_status='embedded'` and are never re-processed.

## Goals / Non-Goals

**Goals**:
- Chunks enqueued immediately on write (watcher + reindex), not delayed 5 minutes
- `chunks_enqueued` count in reindex response is accurate
- Every file being indexed emits an INF log line
- Every chunk being embedded emits an INF log line with file path
- All errors surface visibly — no silent drops

**Non-Goals**:
- Changing the 5-minute scan (it stays as recovery mechanism)
- Changing embed provider selection or configuration
- Modifying harvest pipeline (separate concern, verify separately)

## Decisions

**D1: Watcher gets `eq` via builder method, not constructor**

`watcher.New()` has many params already; adding `eq` to constructor would require changes in many tests. Builder pattern `fw.WithEmbedQueue(eq)` is consistent with existing `.WithSymbolRegistry()` and `.WithGraphRegistry()` patterns. `eq` can be nil (embed disabled) — `writeChunks` nil-checks before calling `Enqueue`.

**D2: Reindex handler gets `eq *embed.Queue` as explicit param**

Handler already takes `queries`, `w *watcher.Watcher`, `logger`. Adding `eq` is consistent. Alternative (handler calls `TriggerRescanByName` only and relies on watcher to enqueue) would add latency (watcher debounce=2s) and still requires watcher to have `eq`. Direct enqueue from reindex is faster and removes the watcher-as-middleman ambiguity.

**D3: UpsertChunk ON CONFLICT adds `embed_status = 'pending'`**

When a file is re-indexed, existing chunks with same `content_hash` should be re-embedded (user triggered reindex intentionally). Not resetting status means reindex silently does nothing for unchanged files. Risk: if embedder is down, this creates a pending backlog — mitigated by the existing retry/backoff/maxRetries logic.

**D4: File log at INF (not DEBUG)**

Users need to see progress. DEBUG is off by default. The log is one line per file, not per chunk — acceptable volume. Chunk embed log stays at DEBUG for chunk ID, but adds file path context. New `"embedding file"` log at INF before embedder call.

**D5: No SQL migration needed for UpsertChunk fix**

ON CONFLICT clause change is SQL-only in `chunks.sql`, no schema change. Requires `sqlc generate` to regenerate `chunks.sql.go`.

## Risks / Trade-offs

- **Double-enqueue**: Watcher enqueues immediately after writeChunks, AND scanPending may enqueue same ID 5 min later. Mitigated: `processChunk` fetches chunk by ID and checks current `embed_status` — if already `'embedded'`, it will re-embed (correct for reindex intent). For normal watcher updates this is fine — `embed_status` is reset to pending on upsert.
- **eq nil in tests**: Many handler/watcher tests pass `nil` for `eq`. All enqueue calls must be nil-guarded (`if eq != nil { eq.Enqueue(id) }`).
- **Reindex enqueue IDs**: `ResetEmbedStatusByCollection` currently returns `int64` (row count), not the actual chunk IDs. Need a new SQL query `ResetAndReturnChunkIDsByCollection` that returns `RETURNING id` instead of count.

## Migration Plan

1. Add SQL query `ResetAndReturnChunkIDsByCollection` returning chunk IDs
2. Run `sqlc generate`
3. Add `eq *embed.Queue` field + `WithEmbedQueue()` to watcher; nil-guard in `writeChunks`
4. Update `ReindexQuerier` interface + handler to accept and use `eq`
5. Wire in `main.go`: `fw.WithEmbedQueue(eq)` after queue creation
6. Update `UpsertChunk` SQL ON CONFLICT to reset `embed_status`
7. Run `sqlc generate` again
8. Add INF log lines in watcher (per-file) and embed queue (per-chunk with file path)
9. `CGO_ENABLED=0 go build ./... && go vet ./... && go test -short ./...`

Rollback: git revert — no schema migration, no data migration needed.

## Open Questions

- Does harvest runner already correctly call `eq.Enqueue()`? Needs spot-check (not blocking).
- Should `processChunk` skip re-embedding if `embed_status='embedded'` (optimize) or always re-embed (correct for reindex)? Current proposal: always re-embed when queued — consistent with user intent.
