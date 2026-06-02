# Embed Status Index + In-Flight Dedup

## Issue
[#322 — perf(embed): add chunks.embed_status partial index + in-flight dedup set](https://github.com/nano-step/nano-brain/issues/322)

## Lane
**normal** — touches embed queue worker hot path + adds new SQL object (partial index) + new struct field (`sync.Map`). Requires OpenSpec proposal, deep-design pass, integration tests. Does NOT change public API or external contracts.

## Why
Research session 2026-06-02 (4 parallel agents: 2 explore + 1 librarian + Oracle cross-check) audited the harvest → embed pipeline. Oracle rejected 10/15 initial findings as premature optimization at current scale (single-dev / small-team, ~10k-100k chunks). Two fixes survived as genuinely worth-doing:

1. **Missing index on `chunks.embed_status`** — 11 queries in `internal/storage/queries/embeddings.sql` filter on `embed_status` (`'pending'`, `'embed_failed'`), but only indexes `idx_chunks_workspace_hash` and `idx_chunks_document_id` exist. The embed queue worker hot path (`internal/embed/queue.go:160` startup scan + `:162` periodic 5-min rescan) does sequential scans on `chunks`. Current scale: ~5ms per scan (tolerable). At 100k+ chunks: noticeable wasted CPU every 5 min.

2. **Embed queue can re-enqueue chunks already in-flight** — `scanByStatus()` runs every 5 min and re-enqueues all `embed_status='pending'` chunks. If a chunk is already in the channel buffer or actively being embedded by a worker, it gets enqueued again → re-embedded. Cost per duplicate: **100-500ms wasted Ollama Embed call** (the single most expensive resource in the pipeline). Initial workspace indexing with thousands of pending chunks is the highest-risk window.

## Desired Outcome
- The 5-minute rescan in the embed worker uses an index for `WHERE embed_status IN ('pending','embed_failed')` lookups, scaling to 100k+ chunks without seq-scan cost.
- A chunk that is already in the embed queue channel OR actively being processed by a worker is **never re-enqueued** by `scanByStatus`. Re-embedding the same chunk twice in flight is impossible.
- Steady-state behavior (no duplicates, no pending chunks) is unchanged — both fixes are pure optimizations with no semantic change to embed/harvest/watcher pipelines.

## Constraints
- Backward compatible: no existing chunk row state must change due to migration. Index creation must NOT lock the table during the migration (use `CREATE INDEX CONCURRENTLY`).
- Migration must be reversible (down migration drops the index cleanly).
- The in-flight set must be cleaned up on **every** processChunk exit path (success, soft failure, hard failure, panic) — use `defer` to guarantee.
- No regression to embed queue throughput in the happy path (no duplicates).
- No new external dependencies. `sync.Map` is from Go stdlib.
- No change to public API, MCP tool surface, or CLI commands.

## Out of Scope
- The other 10/15 audit findings Oracle rejected as premature (EmbedBatch interface, pgx.CopyFrom, sync.Pool for []float32, parallel harvesters, LISTEN/NOTIFY replacement of polling, HNSW retuning, publishStatus debounce — already exists, processChunk DB roundtrip batching, watcher symbol upsert batching, etc.).
- Incremental reindex at the handler level (separate active OpenSpec change `incremental-reindex` for issue #158 covers `POST /api/v1/reindex` handler — zero file overlap with this change).
- Telemetry / metrics for "chunks skipped due to in-flight dedup" — log at DEBUG only; structured metric publishing deferred to a follow-up if signal warrants.

## Acceptance Criteria
1. **Migration applied**: `nano-brain db:migrate` creates `idx_chunks_embed_status` partial index (`WHERE embed_status IN ('pending','embed_failed')`) with `(embed_status, created_at)` composite columns.
2. **Migration reversible**: `goose down` removes the index without error.
3. **No table lock during migration**: migration file uses `-- +goose NO TRANSACTION` annotation so `CREATE INDEX CONCURRENTLY` can run.
4. **EXPLAIN proof**: `EXPLAIN ANALYZE SELECT id FROM chunks WHERE embed_status='pending' ORDER BY created_at LIMIT 1000;` shows `Index Scan using idx_chunks_embed_status`, not `Seq Scan`.
5. **Double Enqueue dedup**: unit test — `q.Enqueue(id); q.Enqueue(id)` results in exactly **1** channel send.
6. **Panic-safe cleanup**: unit test — `processChunk` panics during embed → in-flight set is empty afterward.
7. **scanByStatus dedup**: integration test — chunk ID is in the in-flight set (simulated via direct insert into the map) → `scanByStatus` enqueues 0 sends for that ID.
8. **No regression**: existing embed queue tests (`TestQueue_*` in `internal/embed/queue_test.go`) pass unchanged.
9. **validate:quick passes**: `go build ./... && go test -race -short ./...` exit 0.
10. **test:integration passes**: `go test -race -tags=integration ./internal/embed/...` exit 0.
11. **smoke:e2e SKIPPED with documented reason**: no API surface change → no curl test required.
