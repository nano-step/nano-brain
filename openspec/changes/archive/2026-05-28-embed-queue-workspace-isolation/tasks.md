# Tasks: Embed Queue Workspace Isolation

**Tracking:** nano-step/nano-brain#187

> **Oracle revision:** CleanOrphans (task 2), orphan guard (task 4) removed — impossible due to `ON DELETE CASCADE`. SQL queries corrected: `w.hash` not `w.workspace_hash`.

## 0. SQL queries (embeddings.sql)

- [ ] 0.1 Add `GetPendingChunksAllWorkspaces` query — `EXISTS (SELECT 1 FROM workspaces w WHERE w.hash = c.workspace_hash)`
- [ ] 0.2 Add `GetFailedChunksAllWorkspaces` query — same pattern for `embed_failed`
- [ ] 0.3 Add `DeleteEmbeddingsByWorkspace` query — for `reset-embeddings` CLI
- [ ] 0.4 Run `sqlc generate` — verify no compile errors

## 1. Update `QueueQuerier` interface (queue.go)

- [ ] 1.1 Add `GetPendingChunksAllWorkspaces(ctx, limit) ([]uuid.UUID, error)` to interface
- [ ] 1.2 Add `GetFailedChunksAllWorkspaces(ctx, limit) ([]uuid.UUID, error)` to interface
- [ ] 1.3 Remove `GetAllPendingChunks` and `GetAllFailedChunks` from interface (keep SQL queries for admin use)
- [ ] 1.4 Update `mockQuerier` in queue_test.go — add new methods, remove old

## 2. Update `scanByStatus` to workspace-scoped queries

- [ ] 2.1 Replace `GetAllPendingChunks` → `GetPendingChunksAllWorkspaces` in `scanByStatus`
- [ ] 2.2 Replace `GetAllFailedChunks` → `GetFailedChunksAllWorkspaces` in `scanByStatus`
- [ ] 2.3 No changes to `processChunk` — existing `sql.ErrNoRows` handler (queue.go:218-222) already covers concurrent-delete race

## 3. `reset-embeddings` CLI command

- [ ] 3.1 Create `cmd/nano-brain/cmd_reset_embeddings.go`
- [ ] 3.2 Register subcommand in `cmd/nano-brain/main.go`
- [ ] 3.3 Implement `--workspace=<hash>` flag
- [ ] 3.4 Implement `--workspace-path=<path>` flag (derive hash via `storage.WorkspaceHash`)
- [ ] 3.5 Implement `--dry-run` flag (print counts, no writes)
- [ ] 3.6 Error on missing workspace flag (non-zero exit)
- [ ] 3.7 On execute: `DeleteEmbeddingsByWorkspace` + `ResetEmbedStatus` for workspace
- [ ] 3.8 Print summary: "Reset N chunks for workspace <hash>."

## 4. Tests

- [ ] 4.1 `TestScanPending_WorkspaceScoped` — unregistered workspace chunks not enqueued
- [ ] 4.2 `TestScanPending_RegisteredOnly` — only registered workspace chunks enqueued
- [ ] 4.3 `TestScanFailed_WorkspaceScoped` — failed chunks for unregistered workspace not enqueued
- [ ] 4.4 `TestProcessChunk_ChunkDeletedRace` — `GetChunkByID` returns `ErrNoRows` → handled gracefully, pending decremented, no embed call
- [ ] 4.5 `TestResetEmbeddings_Basic` — resets chunks to pending + deletes embeddings for workspace
- [ ] 4.6 `TestResetEmbeddings_DryRun` — no writes, prints counts
- [ ] 4.7 `TestResetEmbeddings_NoFlag` — exits non-zero when no workspace specified
- [ ] 4.8 `TestResetEmbeddings_WorkspacePath` — --workspace-path derives hash and resets correctly
- [ ] 4.9 Run: `CGO_ENABLED=0 go build ./... && go test -race -short ./...` — all pass

## 5. Validate

- [ ] 5.1 `validate:quick`: `go build ./... && go test -race -short ./...`
- [ ] 5.2 `test:integration`: `go test -race -tags=integration ./...`
- [ ] 5.3 User-flow test (workspace-scoped scan):
  - Register workspace A + workspace B
  - Insert pending chunks for workspace B, then deregister B
  - Restart server → observe 0 chunks enqueued for B
  - Workspace A chunks still processed normally
- [ ] 5.4 User-flow test (`reset-embeddings`):
  - `nano-brain reset-embeddings --workspace-path=<path> --dry-run` → prints counts, no writes
  - `nano-brain reset-embeddings --workspace-path=<path>` → resets, chunks set pending
  - Trigger harvest → verify re-embedding starts
