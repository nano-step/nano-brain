# Spec: Embed Queue Workspace Isolation

**Tracking:** nano-step/nano-brain#187

> **Oracle revision:** AC-1, AC-2, AC-4, AC-7 (CleanOrphans, orphan guard) removed — `ON DELETE CASCADE` on `chunks.document_id` makes orphan chunks impossible. AC-3 renumbered to AC-1.

## AC-1: scanPending only re-queues registered-workspace chunks

**Given** chunks exist for workspace hash `A` (registered) and `B` (not in `workspaces` table)  
**When** `scanPending()` runs  
**Then**
- Chunks for workspace `A` are enqueued
- Chunks for workspace `B` are NOT enqueued
- No error logged for workspace `B` chunks (silently skipped)

## AC-4: processChunk handles cascade-deleted chunk gracefully

**Given** a chunk is enqueued by `scanPending`  
**And** the chunk's parent document is deleted (cascading the chunk deletion) between enqueue and processing  
**When** `processChunk` calls `GetChunkByID`  
**Then**
- `sql.ErrNoRows` returned (chunk no longer exists)
- Existing error handler at queue.go:218-222 logs and decrements pending
- No embed API call made
- No panic, no retry storm

## AC-5: After DB reset + re-register, no stale chunks re-queued

**Given** user performs:
1. `DELETE FROM documents WHERE workspace_hash = $ws`  (cascade deletes chunks)
2. Server restarts
**When** `CleanOrphans()` + `scanPending()` run  
**Then**
- 0 chunks enqueued for that workspace
- Logs show 0 pending for that workspace

## AC-6: `reset-embeddings` CLI resets workspace embed state

**Given** a registered workspace with N embedded chunks  
**When** `nano-brain reset-embeddings --workspace=<hash>` runs  
**Then**
- All embeddings for workspace deleted from `embeddings` table
- All chunks for workspace set to `embed_status = 'pending'`
- Output: "Reset N chunks for workspace <hash>."
- Server re-embeds on next harvest/trigger

## AC-7: `reset-embeddings --dry-run` prints plan without executing

**Given** `nano-brain reset-embeddings --workspace=<hash> --dry-run`  
**When** command runs  
**Then**
- Prints count of embeddings + chunks that WOULD be reset
- No DB writes performed

## AC-8: `reset-embeddings` requires explicit workspace flag

**Given** `nano-brain reset-embeddings` with no flags  
**When** command runs  
**Then**
- Error: "must specify --workspace=<hash> or --workspace-path=<path>"
- Exit code non-zero
- No DB writes

## AC-9: `--workspace-path` resolves to hash correctly

**Given** `nano-brain reset-embeddings --workspace-path=/path/to/project`  
**When** command runs  
**Then**
- Path hashed via `storage.WorkspaceHash("/path/to/project")`
- Same behavior as `--workspace=<derived-hash>`

## AC-10: rescanTicker also uses workspace-scoped query

**Given** server running, rescanTicker fires (every 5 min)  
**When** `scanPending()` called by ticker  
**Then**
- Same `GetPendingChunksAllWorkspaces` query used (not global scan)
- Orphaned chunks not re-queued between restarts

## AC-11: Existing `GetAllPendingChunks` / `GetAllFailedChunks` not used by Queue

**Given** the fix is applied  
**When** `Queue.scanByStatus` runs  
**Then**
- `GetPendingChunksAllWorkspaces` used (not `GetAllPendingChunks`)
- `GetFailedChunksAllWorkspaces` used (not `GetAllFailedChunks`)
- Old queries kept in SQL file for potential admin use but removed from `QueueQuerier` interface
