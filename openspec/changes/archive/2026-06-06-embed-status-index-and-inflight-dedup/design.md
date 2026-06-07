# Design — Embed Status Index + In-Flight Dedup

## Context recap
See `proposal.md`. Two optimizations gated to current scale by Oracle review (session `ses_178ad43a5ffeSJOz81Eo0T79fg`, 2026-06-02).

## Change A — Partial index on `chunks.embed_status`

### Migration file
`migrations/00014_add_chunks_embed_status_index.sql`

```sql
-- +goose Up
-- +goose NO TRANSACTION
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_chunks_embed_status
  ON chunks (embed_status, created_at)
  WHERE embed_status IN ('pending', 'embed_failed');

-- +goose Down
-- +goose NO TRANSACTION
DROP INDEX CONCURRENTLY IF EXISTS idx_chunks_embed_status;
```

### Why partial + composite

- **Partial (`WHERE embed_status IN ('pending','embed_failed')`)**:
  - In steady state ~95%+ of chunks are `'embedded'`. Indexing them wastes space and slows INSERT.
  - The embed worker NEVER queries `WHERE embed_status='embedded'` — that filter would be pointless.
  - Partial index size at 100k chunks with 5% pending: ~5k rows × ~40 bytes = ~200KB. Negligible.
- **Composite `(embed_status, created_at)`**:
  - Worker queries are `WHERE embed_status='pending' ORDER BY created_at ASC LIMIT 1000` (`embeddings.sql:GetPendingChunksAllWorkspaces`).
  - Composite enables index-only ORDER BY, no separate sort node in plan.
  - `created_at` second because `embed_status` has higher selectivity (boolean-ish: pending vs failed).

### Why `CONCURRENTLY`

- Without `CONCURRENTLY`, `CREATE INDEX` takes an `ACCESS EXCLUSIVE` lock on `chunks`. On production deployments with 100k+ chunks this could pause the embed worker + watcher + search queries for seconds.
- `CONCURRENTLY` doesn't block reads/writes, but cannot run inside a transaction — hence `-- +goose NO TRANSACTION`.
- Goose v3 supports this annotation natively (documented in goose docs). **This is the first use in nano-brain** — corrected from earlier draft. No prior migration uses `CONCURRENTLY` or `NO TRANSACTION`.

### Interrupted CONCURRENTLY recovery

If migration 00014 is interrupted mid-build (kill signal, OOM, network blip), Postgres leaves an INVALID index. Goose records 00014 as applied. Recovery:
```bash
psql "$DSN" -c "DROP INDEX CONCURRENTLY IF EXISTS idx_chunks_embed_status;"
goose -dir migrations postgres "$DSN" redo 00014
```
The `IF NOT EXISTS` in the UP migration will NOT rebuild over an invalid index — must drop first. Document in PR description.

### Schema risk

- Zero — adds a new index, touches no rows, no columns, no constraints.
- Down migration cleanly drops the index. If somehow re-applied, `IF NOT EXISTS` makes it idempotent.

### EXPLAIN proof requirement

Acceptance test must include EXPLAIN ANALYZE output (paste in PR description as smoke:e2e SKIP justification).

## Change B — In-flight dedup set in `embed.Queue`

### Data structure
Add to `internal/embed/queue.go` `Queue` struct:

```go
type Queue struct {
    // ... existing fields ...

    // inflight tracks chunk IDs currently in the channel buffer OR being processed.
    // Enqueue skips chunks already present; processChunk removes via defer.
    // Use sync.Map (not RWMutex+map) because access pattern is "lots of unique
    // keys, each touched 2x (Enqueue + processChunk done)" — sync.Map sharded
    // internals win over a single mutex for this pattern.
    inflight sync.Map  // key: uuid.UUID, value: struct{}{}
}
```

### Enqueue change
Current (`queue.go:109`):
```go
func (q *Queue) Enqueue(chunkID uuid.UUID) bool {
    if q.pending.Load() >= rejectionThreshold {
        return false  // backpressure
    }
    select {
    case q.ch <- chunkID:
        q.pending.Add(1)
        q.checkCapacity()
        return true
    default:
        q.logger.Warn()...
        return false
    }
}
```

New:
```go
func (q *Queue) Enqueue(chunkID uuid.UUID) bool {
    if q.pending.Load() >= rejectionThreshold {
        return false  // backpressure (unchanged)
    }
    // Skip if already in-flight (in channel or being processed)
    if _, loaded := q.inflight.LoadOrStore(chunkID, struct{}{}); loaded {
        q.logger.Debug().Stringer("chunk_id", chunkID).Msg("skip enqueue: chunk already in-flight")
        return false
    }
    select {
    case q.ch <- chunkID:
        q.pending.Add(1)
        q.checkCapacity()
        return true
    default:
        // Channel full — undo the LoadOrStore so a future Enqueue can retry
        q.inflight.Delete(chunkID)
        q.logger.Warn()...
        return false
    }
}
```

### handleRetry change (CRITICAL — see D12)

Current (`queue.go:354`) re-enqueues directly into `q.ch` without going through Enqueue, which bypasses the LoadOrStore. We must:
1. Change handleRetry to return `bool` indicating whether re-enqueue succeeded.
2. Keep chunk in `inflight` when re-enqueue succeeds (next processChunk invocation will clean it up).
3. Delete from `inflight` when re-enqueue fails (channel full OR max retries reached).

```go
// New signature: returns true if chunk is still "in-flight" after handleRetry returns
// (i.e., re-enqueued successfully); false if processChunk should delete from inflight.
func (q *Queue) handleRetry(ctx context.Context, chunkID uuid.UUID, workspaceHash string) bool {
    q.retriesMu.Lock()
    q.retries[chunkID]++
    count := q.retries[chunkID]
    q.retriesMu.Unlock()

    if count >= maxRetries {
        // Mark embed_failed in DB. Chunk leaves the pipeline; scanByStatus
        // will pick it up again on next failed-rescan cycle.
        if err := q.queries.MarkChunkEmbedFailed(ctx, ...); err != nil { ... }
        q.pending.Add(-1)
        q.clearRetries(chunkID)
        return false  // ← processChunk's defer will Delete from inflight
    }

    select {
    case q.ch <- chunkID:
        // Successful re-enqueue. Chunk MUST stay in inflight so scanByStatus
        // doesn't double-enqueue it.
        return true  // ← processChunk's defer will SKIP Delete
    default:
        q.pending.Add(-1)
        // Channel full. Drop from inflight so scanByStatus can recover.
        return false  // ← processChunk's defer will Delete from inflight
    }
}
```

### processChunk change (CRITICAL — see D12)

Current (`queue.go:224`):
```go
func (q *Queue) processChunk(ctx context.Context, chunkID uuid.UUID) {
    defer q.pending.Add(-1)
    // ... GetChunkByID, embed, InsertEmbedding, MarkChunkEmbedded ...
    // Soft-failure path:
    //   q.handleRetry(ctx, chunkID, chunk.WorkspaceHash)
    //   return
}
```

New: conditional defer driven by `requeued` flag:
```go
func (q *Queue) processChunk(ctx context.Context, chunkID uuid.UUID) {
    requeued := false
    defer func() {
        if !requeued {
            q.inflight.Delete(chunkID)  // success / hard-fail / panic / final-retry path
        }
        // If requeued: chunk is still in q.ch with same chunkID, MUST stay in inflight
    }()
    defer q.pending.Add(-1)
    // ... GetChunkByID, embed, InsertEmbedding, MarkChunkEmbedded ...
    // Soft-failure path:
    //   requeued = q.handleRetry(ctx, chunkID, chunk.WorkspaceHash)
    //   return
}
```

**Invariant:** `inflight.Load(chunkID)` is true ⟺ chunkID is in `q.ch` buffer OR being processed by a worker. `requeued = true` means "chunk is back in the channel buffer with the same ID, so the next processChunk invocation owns the inflight cleanup."

### Why `sync.Map`?

- Access pattern: each key written exactly once (Enqueue) and deleted exactly once (processChunk). No reads.
- `sync.Map` is optimized for "many keys, each touched a few times" (Go docs explicitly call out this pattern).
- A `RWMutex` + `map[uuid.UUID]struct{}` would have contention on the embed worker hot path with 4 workers + scan goroutine all hitting one lock.
- `sync.Map` has a small per-operation overhead vs plain map, but at our scale (10k-100k unique chunks/day) totally noise.

### LoadOrStore semantics

`sync.Map.LoadOrStore(key, value)` returns `(actual, loaded bool)`:
- If key absent → stores value, returns `(value, false)`. Means "I just claimed this slot."
- If key present → returns existing value, doesn't overwrite, returns `(existingValue, true)`. Means "someone else already claimed it."

We use the boolean: if `loaded == true` → skip.

### Channel-full corner case

If `LoadOrStore` succeeds but the channel send fails (default branch), we must `Delete` the key, otherwise the chunk is stuck in-flight set forever and `scanByStatus` will skip it permanently.

### scanByStatus interaction

`scanByStatus` (`queue.go:187`) fetches chunk IDs from PG and calls `Enqueue`. With the new dedup, IDs that are already in-flight will be skipped at Enqueue time — exactly the desired behavior.

Initial scan on startup: `inflight` is empty, all pending chunks are new → all enqueued. Same as before.

5-min rescan during steady state: most pending chunks already in-flight → skipped. New chunks (added since last scan) get enqueued. Correct.

### Memory bound

`inflight` size ≤ channel size + worker concurrency = `cap(q.ch)` + `q.concurrency` = 10000 + 4 = ~10004 entries max in worst case (when channel is full and all workers are mid-process). Each entry: 16 bytes (uuid) + sync.Map overhead ≈ 64 bytes ≈ 640KB. Negligible.

### Worker goroutine panic semantics (existing behavior, unchanged)

Worker goroutines at `queue.go:178-182` have NO `recover()`. If `processChunk` panics:
1. `defer` chain runs: conditional `inflight.Delete(chunkID)` (since `requeued=false` on panic path), then `pending.Add(-1)`
2. Panic propagates up through the goroutine — Go runtime prints stack trace to stderr and crashes that one goroutine
3. Worker semaphore is released by the wrapping `defer func() { <-sem }()` in Run()'s goroutine launcher
4. `wg.Done()` fires via outer defer
5. Run() loop continues processing remaining channel items with N-1 workers

Adding `recover()` is **out of scope** — would change crash semantics and hide bugs. The dedup change preserves this exact behavior; `defer` semantics guarantee cleanup on panic path same as on return path.

## Failure modes & invariants

### Invariants
- **I1**: A chunk ID is in `inflight` set ⟺ it's either in the channel buffer OR being processed in `processChunk`.
- **I2**: Every `processChunk` invocation removes the chunk from `inflight` exactly once before returning (via `defer`, covers panic).
- **I3**: `pending` counter and `inflight` size differ at most transiently (Enqueue → store map → send channel → increment counter is not atomic across all 3, but always converges).

### Failure modes considered

| Scenario | Behavior | Mitigation |
|---|---|---|
| processChunk panics during embed | `requeued=false` → conditional defer fires `inflight.Delete(chunkID)` → cleanup OK | Built into `defer` semantics; panic propagates to stderr (existing behavior) |
| Enqueue: channel send fails (full) after LoadOrStore succeeded | Default branch deletes from inflight | Explicit `q.inflight.Delete(chunkID)` in default arm |
| **handleRetry: re-enqueue succeeds** | `requeued=true` → conditional defer SKIPS `inflight.Delete` → chunk stays in map ⟺ chunk in channel buffer | New behavior in D12 — conditional defer prevents race against next processChunk |
| **handleRetry: re-enqueue fails (channel full)** | `requeued=false` → defer deletes from inflight → scanByStatus will recover from DB | handleRetry's default branch returns `false`; processChunk defer cleans up |
| **handleRetry: max retries reached** | `requeued=false` + `MarkChunkEmbedFailed` in DB → defer deletes from inflight → chunk leaves the hot path | handleRetry returns `false` on max-retry path |
| Server SIGKILL mid-process | inflight lost (in-memory only) → next startup scanPending re-enqueues from DB → eventually processed | Acceptable; DB is source of truth, restart recovers |
| Same chunk enqueued from harvest + watcher + scanByStatus simultaneously | All three race on LoadOrStore — exactly ONE wins, other two return early | sync.Map atomic semantics |
| `MarkChunkEmbedded` succeeds but `inflight.Delete` skipped | Impossible — `defer` runs unconditionally with `requeued=false` on success path | N/A |
| `scanByStatus` runs while chunk is in `inflight` but processChunk wedged | scanByStatus skips it (still in map) → chunk stuck pending forever | **MITIGATION GAP** — see "Stuck-chunk recovery" below |

### Stuck-chunk recovery (edge case)

If a chunk is added to `inflight` but `processChunk` neither completes nor panics (e.g. goroutine deadlock — should be impossible but defensive), the chunk would stay in `inflight` forever and `scanByStatus` would always skip it.

**Decision**: Don't add explicit recovery. Reasons:
1. `processChunk` has no unbounded blocking calls. Ollama Embed has its own 2-min timeout. PG queries use ctx.
2. If a real deadlock bug existed, the embed worker would also be wedged → already a P0 incident, not something to paper over.
3. Adding TTL/cleanup logic to `inflight` would complicate the model for a theoretical case.

**Future option**: If observed in production, add a janitor goroutine that periodically scans `inflight` and removes entries older than `(processChunkTimeout * 2)`. Defer until evidence demands it.

## Test plan

### Unit tests (`internal/embed/queue_test.go`)

1. **`TestQueue_Enqueue_DedupSameID`**:
   - Setup: new Queue with mock embedder
   - Action: `q.Enqueue(id); q.Enqueue(id)` (same UUID twice in quick succession before workers can drain)
   - Assert: `len(q.ch) == 1` (only one channel send)
   - Assert: `q.pending.Load() == 1`

2. **`TestQueue_Enqueue_DifferentIDsBothEnqueued`**:
   - Setup: same
   - Action: `q.Enqueue(id1); q.Enqueue(id2)` (different UUIDs)
   - Assert: `len(q.ch) == 2`

3. **`TestQueue_ProcessChunk_PanicCleansInflight`**:
   - Setup: Queue with embedder that panics
   - Action: `q.processChunk(ctx, id)` (recover panic in test)
   - Assert: `_, ok := q.inflight.Load(id); ok == false`

4. **`TestQueue_ChannelFull_DeletesInflight`**:
   - Setup: Queue with channel cap=1, fill it; trigger Enqueue that hits default branch
   - Assert: `inflight` does not contain the rejected ID (so retry is possible)

5. **`TestQueue_Enqueue_AfterProcessChunkDone_AllowsReEnqueue`**:
   - Setup: Enqueue id, run processChunk to completion, Enqueue id again
   - Assert: second Enqueue succeeds (channel send happens, `inflight` re-claimed)

### Integration test (`internal/embed/queue_integration_test.go` with `//go:build integration`)

6. **`TestQueue_ScanByStatus_SkipsInflightChunks`**:
   - Setup: real PG via `testutil.SetupTestDB`, insert 100 chunks with `embed_status='pending'`, mock embedder that blocks on a channel.
   - Action: start `q.Run(ctx)`. After workers drain the channel (block on embedder), invoke `q.scanByStatus(ctx, "pending")` manually.
   - Assert: total channel sends == 100 (not 200). Verify via counter on mock embedder + `q.inflight` size.
   - Teardown: unblock embedder, wait for processing complete, assert `q.inflight` is empty.

### Migration smoke (manual, captured in PR description)

7. Run on dev PG (host port 5432, `nanobrain_dev`):
   ```bash
   goose -dir migrations postgres "$DSN" up
   psql "$DSN" -c "\d+ chunks" | grep idx_chunks_embed_status
   psql "$DSN" -c "EXPLAIN ANALYZE SELECT id FROM chunks WHERE embed_status='pending' ORDER BY created_at LIMIT 1000;"
   ```
   Paste output in PR.

### Negative test
- `TestQueue_Enqueue_DoesNotBypassBackpressure`: existing test should still pass — backpressure check at top of Enqueue runs BEFORE the dedup check.

## Decisions log

| ID | Decision | Alternatives considered | Why chosen |
|---|---|---|---|
| D1 | Use partial index `WHERE embed_status IN ('pending','embed_failed')` | Full index on embed_status; index on workspace_hash+embed_status | Partial keeps index ~95% smaller; embed_status=embedded never queried; composite with workspace_hash would help per-workspace queries but `*AllWorkspaces` variants need the global form |
| D2 | Composite `(embed_status, created_at)` | `(embed_status)` alone | Worker queries ORDER BY created_at; composite avoids separate sort step in plan |
| D3 | `CREATE INDEX CONCURRENTLY` + `-- +goose NO TRANSACTION` | Plain `CREATE INDEX` in transaction | Production may have 100k+ chunks; ACCESS EXCLUSIVE lock unacceptable. Goose supports the annotation natively. |
| D4 | Migration number 00014 | 00012 (proposal said) | 00012 + 00013 already exist (graph_edge_references, bm25_include_title) per `ls migrations/`. Renumber to next available. |
| D5 | `sync.Map` for inflight set | `RWMutex + map[uuid.UUID]struct{}` | sync.Map sharded internals win under "write-once read-never" pattern with 4+ concurrent goroutines. Stdlib, no allocation per access. |
| D6 | `LoadOrStore` + boolean check in Enqueue | Separate Load + Store calls | LoadOrStore is atomic; separate Load+Store races. Test 1 would flake. |
| D7 | `defer q.inflight.Delete(id)` at TOP of processChunk | Manual cleanup in each return path | defer covers panic. Manual would miss panic path → invariant I2 violation. |
| D8 | Delete from inflight on channel-full default branch | Leave it stuck (relies on processChunk to clean) | If channel send fails, processChunk never runs for this Enqueue call — must undo to allow retry. |
| D9 | No janitor for stuck inflight entries | Add TTL-based cleanup goroutine | Adds complexity for theoretical case (processChunk has no unbounded blocking). Revisit if observed. |
| D10 | No new metric/counter for "skipped due to inflight" | Expose via /api/status or Prometheus | DEBUG log is sufficient for verification; metric is feature creep beyond P1 fix. Add later if signal warrants. |
| D11 | Smoke:e2e SKIP — no API surface change | Build server + curl `/api/status` to verify embed counters | Embed counters are unchanged in steady state (no duplicates → no skip log). Existing `/api/status` does not expose `inflight` size. Adding it would expand scope. Migration smoke (manual EXPLAIN) covers the index. |
| **D12** | **handleRetry returns `bool`; processChunk uses conditional defer** | (a) Unconditional `defer inflight.Delete` at top of processChunk; (b) handleRetry calls inflight.Delete itself before re-enqueue; (c) handleRetry goes through Enqueue instead of direct `q.ch <- chunkID` | **Oracle BLOCKER review identified handleRetry as a side-channel that bypasses Enqueue's LoadOrStore.** (a) violates I1 — chunk would be in channel buffer but not in inflight set after a retry. (b) has a race window between Delete and channel send. (c) is overkill — would require checking the in-flight set against itself (`LoadOrStore` would always succeed since handleRetry just deleted, so what's the point?). The conditional defer is the simplest correct solution: handleRetry returns "I re-enqueued successfully, keep the chunk in inflight for the next processChunk invocation to clean up." |
| D13 | First use of `-- +goose NO TRANSACTION` in this project | Plain `CREATE INDEX` (would lock table) | Production may have 100k+ chunks. Verified via grep: zero existing migrations use this annotation. Goose v3 supports it natively. Document the interrupted-CONCURRENTLY recovery procedure in PR description. |
| D14 | Don't fix pre-existing CHECK constraint mismatch (`embed_permanently_failed` not in CHECK) | Add migration 00015 expanding CHECK | **Out of scope for #322.** Oracle's secondary finding: migration 00004 defines `CHECK (embed_status IN ('pending', 'embedded', 'embed_failed'))` but code at `embeddings.sql:25` writes `'embed_permanently_failed'`. This UPDATE would silently fail at runtime (PG raises 23514 check_violation). Either no chunk ever reaches max retries today, OR the error is swallowed somewhere. **File as follow-up issue** — fixing here would scope-creep this PR. |
