# Design: Embed Queue Workspace Isolation

> **Oracle Finding 1 (CRITICAL):** `workspaces` table uses column `hash`, not `workspace_hash`. All queries below use `w.hash`. Verified against `ListWorkspacesWithStats` in workspaces.sql.go.
>
> **Oracle Finding 2 (BLOCKING):** `chunks.document_id` is `NOT NULL REFERENCES documents(id) ON DELETE CASCADE`. Orphan chunks (document deleted → chunk survives) are **impossible** — PostgreSQL cascade-deletes chunks atomically. `CleanOrphanedChunks` and the `processChunk` orphan guard are therefore **removed** from this design. The core fix (workspace-scoped scan) alone solves the reported problem.

## Current State (Broken)

```
Server startup
  └─ Queue.Run(ctx)
       └─ scanPending()                         ← fires immediately
            └─ GetAllPendingChunks(limit=1000)  ← NO workspace filter
                 └─ all pending chunks globally re-queued
                      └─ includes chunks from unregistered workspaces
                           └─ processChunk()
                                └─ embed wasted on stale/unneeded workspace data
```

`rescanTicker` fires every **5 minutes** — cycle repeats indefinitely.

## Target State

```
Server startup
  └─ Queue.Run(ctx)
       └─ scanPending()
            └─ GetPendingChunksAllWorkspaces(limit=1000)  ← NEW: registered-workspace-scoped
                 SELECT c.id FROM chunks c
                 WHERE c.embed_status = 'pending'
                   AND EXISTS (SELECT 1 FROM workspaces w WHERE w.hash = c.workspace_hash)
                 ORDER BY c.created_at ASC
                 LIMIT $1
                 └─ only chunks from registered workspaces re-queued
                      └─ processChunk()
                           └─ GetChunkByID → embed → InsertEmbedding → MarkEmbedded
```

Note: If a chunk is enqueued and its document is concurrently deleted (via cascade), `GetChunkByID` returns `sql.ErrNoRows`. This is already handled at queue.go:218-222 — the existing error path logs and decrements pending. No additional guard needed.

## SQL Changes

### New query: `GetPendingChunksAllWorkspaces`
```sql
-- name: GetPendingChunksAllWorkspaces :many
SELECT c.id FROM chunks c
WHERE c.embed_status = 'pending'
  AND EXISTS (
    SELECT 1 FROM workspaces w
    WHERE w.hash = c.workspace_hash
  )
ORDER BY c.created_at ASC
LIMIT $1;
```

Replaces `GetAllPendingChunks` in `scanByStatus`. Same for failed:

### New query: `GetFailedChunksAllWorkspaces`
```sql
-- name: GetFailedChunksAllWorkspaces :many
SELECT c.id FROM chunks c
WHERE c.embed_status = 'embed_failed'
  AND EXISTS (
    SELECT 1 FROM workspaces w
    WHERE w.hash = c.workspace_hash
  )
ORDER BY c.created_at ASC
LIMIT $1;
```

> **Removed:** `CleanOrphanedChunks` — dead code. `ON DELETE CASCADE` ensures orphan chunks cannot exist.

## Queue.go Changes

### 1. Update `scanByStatus` to use workspace-scoped queries
```go
func (q *Queue) scanByStatus(ctx context.Context, failed bool) int {
    // ...
    if failed {
        ids, err = q.queries.GetFailedChunksAllWorkspaces(ctx, scanBatchSize)
    } else {
        ids, err = q.queries.GetPendingChunksAllWorkspaces(ctx, scanBatchSize)
    }
    // ...
}
```

Update `QueueQuerier` interface: add new methods, remove `GetAllPendingChunks` / `GetAllFailedChunks`.

### 2. No orphan guard needed in `processChunk`

The existing error path at `queue.go:218-222` already handles the race where a chunk is enqueued and then cascade-deleted before `processChunk` runs — `GetChunkByID` returns `sql.ErrNoRows`, which the existing code logs and recovers from. No additional code needed.

## CLI: `reset-embeddings`

```
nano-brain reset-embeddings [--workspace=<hash>] [--workspace-path=<path>]

Flags:
  --workspace=<hash>        workspace hash (from nano-brain status)
  --workspace-path=<path>   derive hash from path (alternative to --hash)
  --dry-run                 print what would be reset, don't execute

Actions:
  1. DELETE FROM embeddings WHERE workspace_hash = $1
  2. UPDATE chunks SET embed_status = 'pending' WHERE workspace_hash = $1
  3. Print: "Reset N chunks for workspace <hash>. Re-embedding will begin on next harvest."
```

Uses existing `ResetEmbedStatus` query (embeddings.sql:30-31) + new `DeleteEmbeddingsByWorkspace` query.
No schema change — `ResetEmbedStatus` already resets all statuses (no WHERE on embed_status).

## sqlc Regeneration

After updating `embeddings.sql`:
```bash
sqlc generate
```
Verify no compile errors, update `QueueQuerier` interface in `queue.go`.

## Test Plan

| Test | What it verifies |
|------|-----------------|
| `TestScanPending_WorkspaceScoped` | Chunks for unregistered workspace NOT re-queued |
| `TestScanPending_RegisteredWorkspace` | Chunks for registered workspace ARE re-queued |
| `TestScanFailed_WorkspaceScoped` | Failed chunks for unregistered workspace NOT re-queued |
| `TestProcessChunk_ChunkDeletedRace` | Chunk deleted between enqueue and processChunk → ErrNoRows handled gracefully |
| `TestResetEmbeddings_Basic` | reset-embeddings resets chunks to pending + deletes embeddings |
| `TestResetEmbeddings_DryRun` | --dry-run prints counts, no DB writes |
| `TestResetEmbeddings_NoFlag` | exits non-zero when no workspace specified |
| `TestResetEmbeddings_WorkspacePath` | --workspace-path derives hash and resets correctly |

## Migration

No DB schema migration needed. All changes are:
- New SQL queries (additive)
- Go code changes (queue.go, cmd)
- sqlc regeneration

`embed_status` column and `workspaces` table already exist.
