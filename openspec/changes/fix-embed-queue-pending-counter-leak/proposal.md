## Why

Production embed queue saturates at 9999/10000 backpressure. PRs #266 (#259) and #267 (#260) fixed two contributors (cascade-deleted chunks and oversized chunk hard-failures), but deep code analysis reveals two **independent remaining failure modes** in `internal/embed/queue.go` that keep the in-memory `pending` counter from matching the DB ground truth:

1. **Mode 6 (handleRetry channel-full):** `queue.go:361-367` — when a retry attempt finds the channel full, `q.pending.Add(-1)` decrements the counter but the chunk is **not** re-enqueued and its `embed_status` is **not** updated. The 5min `scanPending` re-enqueues the same chunk (re-incrementing pending). Under sustained transient errors + queue pressure, the counter drifts and saturation accelerates.
2. **Mode 2 (MarkChunkEmbedded failure):** `queue.go:303-310` — if InsertEmbedding succeeds but the subsequent `MarkChunkEmbedded` UPDATE fails (DB error, conn loss, timeout), pending is decremented while the chunk stays `embed_status='pending'` in DB. The chunk is re-enqueued by scan, embedded again (idempotent via ON CONFLICT), and the divergence persists.

Both modes violate the invariant: `q.pending.Load() == COUNT(chunks WHERE embed_status='pending')`.

## What Changes

- Modify `handleRetry` channel-full branch to **NOT** decrement `pending`; let scan re-enqueue naturally. Chunk stays `embed_status='pending'` in DB → invariant preserved.
- Modify `processChunk` MarkChunkEmbedded error branch to **NOT** decrement `pending`; emit `publishStatus` for observability; let next scan re-process. ON CONFLICT on InsertEmbedding ensures idempotency.
- Add 3 new tests in `queue_test.go` covering both modes + an invariant-stress test.
- Update `internal/embed/queue.go` Doc comment on `pending` field stating the invariant.

## Capabilities

### New Capabilities
- `embed-queue-pending-counter-invariant`: Ensures the in-memory `pending` atomic counter equals `COUNT(chunks WHERE embed_status='pending')` across all failure modes. Specifies retry channel-full handling and post-embed status-update failure handling.

### Modified Capabilities
None — this is a defect fix; no new user-facing requirements.

## Impact

- **Code:** `internal/embed/queue.go` (handleRetry + processChunk), `internal/embed/queue_test.go` (3 new tests).
- **Behavior:** No external API change. Backpressure status reported by `/api/status` becomes monotonic + accurate. Operators can trust `queue_pending` as a saturation indicator.
- **Risk:** Low — change removes erroneous decrements; worst case is a slightly higher reported pending count during transient errors (which IS the correct value, matching DB).
- **Dependencies:** None.
- **Database:** No schema change.
