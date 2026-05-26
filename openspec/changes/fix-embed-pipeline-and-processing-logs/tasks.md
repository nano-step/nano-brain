# Tasks: fix-embed-pipeline-and-processing-logs

## T1 — New SQL query: ResetAndReturnChunkIDsByCollection
- [ ] Add `-- name: ResetAndReturnChunkIDsByCollection :many` to `internal/storage/queries/embeddings.sql`
- [ ] Query: `UPDATE chunks SET embed_status='pending' WHERE workspace_hash=$1 AND collection=$2 RETURNING id`
- [ ] Run `sqlc generate`
- [ ] Verify generated `ResetAndReturnChunkIDsByCollection` method in `embeddings.sql.go`

## T2 — UpsertChunk resets embed_status on conflict
- [ ] Edit `internal/storage/queries/chunks.sql`: ON CONFLICT DO UPDATE add `embed_status = 'pending'`
- [ ] Run `sqlc generate`
- [ ] Verify change in `chunks.sql.go`

## T3 — Watcher gains embed queue
- [ ] Add `eq *embed.Queue` field to `Watcher` struct (`watcher.go`)
- [ ] Add `WithEmbedQueue(eq *embed.Queue) *Watcher` builder method
- [ ] In `writeChunks()`: after each successful `UpsertChunk`, call `if w.eq != nil { w.eq.Enqueue(id) }`
- [ ] Add `INF processing file path=<path> collection=<name>` log before file read in `processDirty`
- [ ] Nil-guard: if `w.eq == nil`, skip enqueue silently

## T4 — Reindex handler enqueues directly
- [ ] Update `ReindexQuerier` interface: replace `ResetEmbedStatusByCollection` with `ResetAndReturnChunkIDsByCollection`
- [ ] Update `TriggerReindex` signature: add `eq *embed.Queue` param
- [ ] Replace count-only reset with ID-returning query; enqueue each returned ID
- [ ] Update `chunks_enqueued` to count actual enqueued IDs (may be less if queue full)
- [ ] Update handler registration in `routes.go` to pass `eq`

## T5 — Wire in main.go
- [ ] After `eq = embed.NewQueue(...)`, call `fw.WithEmbedQueue(eq)`
- [ ] Verify order: `fw` created → `eq` created → `fw.WithEmbedQueue(eq)`

## T6 — Embed queue: promote logs to INF + add file path
- [ ] In `processChunk`: fetch chunk includes `source_path` (already in `GetChunkByIDRow`?) — verify
- [ ] Add `INF embedding chunk chunk_id=<uuid> file=<path>` before `embedder.Embed()`
- [ ] Promote `"chunk embedded"` log from DEBUG → INF, add `file` field
- [ ] Ensure `"embedding failed"` log includes `file` field

## T7 — Build + vet + test
- [ ] `CGO_ENABLED=0 go build ./...` — clean
- [ ] `CGO_ENABLED=0 go vet ./...` — clean
- [ ] `CGO_ENABLED=0 go test -short ./...` — all pass
- [ ] Fix any broken tests (handler tests need `eq` param updated)

## T8 — Validate + commit
- [ ] `openspec validate fix-embed-pipeline-and-processing-logs`
- [ ] Spot-check harvest runner: confirm it calls `eq.Enqueue()` correctly
- [ ] Commit: `fix(embed): wire watcher+reindex to embed queue, add file-level logs`
