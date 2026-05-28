# Self-Review: embed-queue-workspace-isolation

**Date:** 2026-05-28
**Branch:** feat/embed-queue-workspace-isolation
**Issue:** nano-step/nano-brain#187
**Change type:** bug-fix
**Lane:** high-risk

## Findings

### RESOLVED — Oracle Finding 1 (CRITICAL): Wrong SQL column name
- `w.workspace_hash` → `w.hash` in all new queries
- Verified against `workspaces.sql.go:ListWorkspacesWithStats`
- Status: FIXED

### RESOLVED — Oracle Finding 2 (BLOCKING): CleanOrphans dead code
- `ON DELETE CASCADE` on `chunks.document_id` prevents orphan chunks
- Removed `CleanOrphanedChunks` SQL, `CleanOrphans()` method, and orphan guard in `processChunk`
- Status: REMOVED (not implemented)

### NOTE — Pre-existing lint (commands_test.go:583,619)
- `w.Write` errcheck in pre-existing test file
- Present on master before this branch
- Not introduced by this change
- Status: PRE-EXISTING, out of scope

## Verification Evidence

```
CGO_ENABLED=0 go build ./...   → exit 0 ✅
go test -race -short ./...     → all pass ✅
go test -race -tags=integration ./... → all pass ✅
golangci-lint ./...            → 2 pre-existing errcheck in commands_test.go ✅
```

## Smoke Test Evidence

```
reset-embeddings (no flags) → exit 1, "must specify --workspace=..." ✅
Server health (port 3199)   → HTTP 200 ✅
Server shutdown             → clean context cancel, no panic ✅
```

## Files Changed

| File | Change |
|------|--------|
| `internal/storage/queries/embeddings.sql` | +3 queries: GetPendingChunksAllWorkspaces, GetFailedChunksAllWorkspaces, DeleteEmbeddingsByWorkspace |
| `internal/storage/sqlc/embeddings.sql.go` | sqlc regenerated |
| `internal/embed/queue.go` | Interface: removed GetAll*, added workspace-scoped; scanByStatus updated |
| `internal/embed/queue_test.go` | mockQuerier updated, workspace-scoped scan tests added |
| `internal/server/handlers/embed.go` | Interface alignment (handler uses querier) |
| `cmd/nano-brain/cmd_reset_embeddings.go` | New: reset-embeddings CLI subcommand |
| `cmd/nano-brain/main.go` | Register reset-embeddings |
| `scripts/harness-check.sh` | Fix openspec status grep to match `in-progress` |
