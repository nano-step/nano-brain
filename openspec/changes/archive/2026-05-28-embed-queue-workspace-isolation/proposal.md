# Proposal: Embed Queue Workspace Isolation

**Tracking:** nano-step/nano-brain#187  
**Lane:** high-risk  
**Change type:** bug-fix  
**Date:** 2026-05-28

## Problem

After deleting `~/.nano-brain/sessions/` and resetting the database, the embed queue continues processing chunks referencing deleted file paths. This causes:

1. Wasted LLM embedding calls on orphaned/stale data
2. Confusing logs (`INF embedding chunk file=/Users/tamlh/.nano-brain/sessions/xxx/yyy.md`)
3. No safe way to reset embed state for a single workspace
4. Cross-workspace contamination: chunks from workspace A re-queued during workspace B's session

## Root Cause (3 defects)

### Defect 1 — `GetAllPendingChunks` has no workspace filter

```sql
-- current (BROKEN): scans entire chunks table
SELECT id FROM chunks
WHERE embed_status = 'pending'
ORDER BY created_at ASC
LIMIT $1;
```

`Queue.scanPending()` fires on every startup AND every 5 minutes (`rescanInterval`). Any chunk with `embed_status='pending'` is re-queued globally — across all workspaces, including deleted/orphaned ones.

### Defect 2 — `processChunk` has no orphan guard

When a chunk's parent document is deleted, `GetChunkByID` returns `source_path=''` (LEFT JOIN COALESCE). The worker logs the empty path and proceeds to embed — wasting an embed call and inserting a junk embedding with no document context.

### Defect 3 — No workspace-scoped reset CLI

Users have no safe tool to clear embed state for a single workspace. The only option is full DB drop/recreate, which is destructive and error-prone.

## Proposed Fix

### Fix 1 — Add workspace-aware scan to `Queue`

Replace global `GetAllPendingChunks` scan with workspace-scoped query (reuse existing `GetPendingChunks` per registered workspace), or add a new `GetAllPendingChunksScoped` query that joins with `workspaces` table.

**Option A — Per-workspace scan (join workspaces table)**
```sql
-- GetPendingChunksAllWorkspaces :many
SELECT c.id FROM chunks c
WHERE c.embed_status = 'pending'
  AND EXISTS (SELECT 1 FROM workspaces w WHERE w.workspace_hash = c.workspace_hash)
ORDER BY c.created_at ASC
LIMIT $1;
```
Chunks whose workspace was deleted/deregistered are automatically excluded.

**Option B — Orphan cleanup on startup**
On startup, before `scanPending`: mark all chunks with no matching document as `embed_failed`:
```sql
UPDATE chunks SET embed_status = 'embed_failed'
WHERE document_id NOT IN (SELECT id FROM documents);
```

**Decision: Both** — Option A prevents re-queue; Option B cleans up existing orphans.

### Fix 2 — Orphan guard in `processChunk`

```go
chunk, err := q.queries.GetChunkByID(ctx, chunkID)
// ... error check ...

// NEW: orphan guard
if chunk.DocumentID == uuid.Nil {
    q.logger.Warn().Str("chunk_id", chunkID.String()).Msg("orphaned chunk (document deleted) — marking embedded, skipping")
    _ = q.queries.MarkChunkEmbedded(ctx, sqlc.MarkChunkEmbeddedParams{ID: chunkID, WorkspaceHash: chunk.WorkspaceHash})
    q.pending.Add(-1)
    return
}
```

### Fix 3 — `reset-embeddings` CLI command

```bash
nano-brain reset-embeddings --workspace=<hash>
# Resets embed_status='pending' for all chunks in workspace
# Deletes embeddings for workspace
# Triggers immediate re-embed on next harvest
```

## Files Changed

| File | Change |
|------|--------|
| `internal/storage/queries/embeddings.sql` | Add `GetPendingChunksAllWorkspaces` + `CleanOrphanedChunks` queries |
| `internal/storage/sqlc/` | Regenerate via `sqlc generate` |
| `internal/embed/queue.go` | Use workspace-scoped scan; add orphan guard in `processChunk`; add `CleanOrphans()` startup call |
| `cmd/nano-brain/main.go` | Wire `CleanOrphans` call on startup |
| `cmd/nano-brain/cmd_reset_embeddings.go` (new) | `reset-embeddings` CLI subcommand |
| `internal/embed/queue_test.go` | Tests for orphan guard, workspace isolation, cleanup |

## Risks

| Risk | Mitigation |
|------|-----------|
| `CleanOrphans` on startup deletes chunks that are legitimately pending (race with harvest) | Add 60s grace: skip chunks created in last 60s |
| Workspace-scoped scan misses chunks for unregistered but valid workspaces | Option A join guarantees only registered workspaces are scanned |
| `reset-embeddings` CLI accidentally resets wrong workspace | Require explicit `--workspace=<hash>` or `--workspace-path=<path>` |
| sqlc regeneration breaks other callers | Run full `go build ./...` + `go test -race ./...` post-regenerate |
