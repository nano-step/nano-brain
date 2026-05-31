## Context

`internal/embed/queue.go` uses an in-memory `atomic.Int64 pending` counter to report queue backlog via `/api/status`. The DB ground truth is `COUNT(chunks WHERE embed_status='pending')`. The invariant `pending.Load() == count(DB pending)` must hold across all code paths.

Two paths violate this invariant under failure conditions:

**Path A — handleRetry channel-full:**
```go
// queue.go:361-367
select {
case q.ch <- chunkID:
    q.logger.Debug(...)  // OK: chunk back in queue
default:
    q.pending.Add(-1)    // PROBLEM: counter ↓ but DB still 'pending'
    q.logger.Warn(...)
}
```

**Path B — MarkChunkEmbedded failure:**
```go
// queue.go:303-310
if err := q.q.MarkChunkEmbedded(ctx, ...); err != nil {
    q.logger.Error(...)
    q.pending.Add(-1)    // PROBLEM: counter ↓ but DB still 'pending'
    return
}
```

The scan loop (every 5min) re-fetches `embed_status='pending'` chunks and re-enqueues them, incrementing `pending` again. The result is monotonic drift: under sustained transient errors or DB instability, `pending` is decremented per error but re-incremented per scan, accumulating a "stuck" value that approaches the rejection threshold (50,000).

PRs #266 and #267 (merged 2026-05-31) fixed two separate contributors: cascade-deleted chunks (ErrNoRows) and hard-failure HTTP codes (400/401/403/413/422). Those paths correctly decrement pending AND update DB status. The two paths above were not touched.

## Goals / Non-Goals

**Goals:**
- Preserve invariant `pending.Load() == count(DB pending)` across all code paths in queue.go.
- Allow transient errors + channel-full conditions to recover via scan loop without counter drift.
- Make MarkChunkEmbedded failures idempotent on retry (no duplicate embeddings, no leaked pending counts).
- Add regression tests for both modes + an invariant-stress test.

**Non-Goals:**
- No change to embed provider behavior, retry policy, or HTTP error classification.
- No change to scan interval or rejection threshold.
- No new metrics (defer to a separate observability change if needed).
- No schema migration.

## Decisions

### D1: Channel-full retry leaves counter intact
**Decision:** Remove `q.pending.Add(-1)` from the `default` branch in handleRetry channel-full path. Chunk stays in DB as 'pending', counter unchanged, scan re-enqueues in ≤5min.

**Rationale:** The counter represents "chunks that need work". A chunk that failed to re-enqueue still needs work (it's still 'pending' in DB). Decrementing falsifies the counter. The scan loop is the natural recovery path; let it do its job.

**Alternative considered:** Block on channel send instead of `select default`. Rejected — would deadlock worker goroutines if the channel feeds itself (retry path consuming its own producer slot).

### D2: MarkChunkEmbedded failure leaves counter intact + publishStatus
**Decision:** Remove `q.pending.Add(-1)` from the error branch. Add `q.publishStatus()` call so observers see the failure event. Scan re-enqueues; InsertEmbedding's ON CONFLICT clause makes the retry safe.

**Rationale:** Same invariant logic as D1. The chunk's DB status is still 'pending'; the counter must reflect that. publishStatus emits the same event shape as the success path so health-check consumers stay consistent.

**Alternative considered:** Wrap InsertEmbedding + MarkChunkEmbedded in a single transaction. Rejected — InsertEmbedding can be slow (vector op), and a long-held tx blocks other workers + scans. The current "best-effort idempotent" design is simpler and proven.

### D3: Test plan
Three tests in `queue_test.go`:

1. **TestQueue_RetryRequeueChannelFull:**
   - Create queue with channel capacity 1.
   - Insert 2 chunks, fill channel.
   - Trigger transient error on chunk_id_1 → handleRetry.
   - Assert: pending counter unchanged, channel still has 1 item (chunk_id_2), chunk_id_1 status='pending' in DB.

2. **TestQueue_MarkChunkEmbeddedFailure:**
   - Mock embedder to succeed, mock MarkChunkEmbedded to fail.
   - Process one chunk.
   - Assert: pending counter unchanged, chunk has row in embeddings table, chunk status='pending' in DB, publishStatus event emitted.

3. **TestQueue_InvariantPendingMatchesDB:**
   - Insert 100 chunks. Run queue with 4 workers.
   - Inject 30% transient errors + 10% MarkChunkEmbedded failures via mock.
   - Run for 10 seconds with concurrent scan.
   - Assert: at end, `q.pending.Load() == count(chunks WHERE embed_status='pending')` within ±0.

### D4: Documentation
Add `pending` field doc comment in queue.go stating the invariant explicitly. Future maintainers will see the constraint when reviewing changes.

## Risks / Trade-offs

| Risk | Mitigation |
|---|---|
| Higher reported pending count during outages | This IS the correct value; matches DB ground truth. Operators must understand pending=high during embed-provider outage is informational, not a queue bug. |
| Slower convergence when channel chronically full | Scan interval (5min) bounds the latency. If this becomes an issue, future change can shorten scan or add immediate-retry-with-backoff. Out of scope here. |
| Test flakiness on TestQueue_InvariantPendingMatchesDB | Use deterministic seed + bounded goroutine count. Run with `-race -count=10` in CI to catch flakes early. |
| Duplicate publishStatus events on retried chunks | Observers must handle idempotent events. This is already true for the success path's publishStatus (called once per processChunk attempt). No regression. |
