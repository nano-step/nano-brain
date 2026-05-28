## Why

`POST /api/v1/reindex` accepts a collection name and returns `202 Accepted` but performs no actual work — it logs "reindex queued" and returns. Callers (agents, CLI, MCP tools) believe reindexing was triggered when nothing happened. This defeats the purpose of the endpoint and causes silent data staleness. Fixes [#162](https://github.com/nano-step/nano-brain/issues/162).

## What Changes

- `internal/server/handlers/reindex.go`: `TriggerReindex` is replaced with a real implementation that (1) marks all chunks for the named collection as `embed_status='pending'` via storage, and (2) triggers the watcher to rescan that collection's directory so new/changed files are re-ingested.
- `internal/storage/queries/embeddings.sql`: Add `ResetEmbedStatusByCollection` query — resets `embed_status` to `'pending'` for all chunks whose document belongs to a given collection in a workspace.
- `internal/storage/sqlc/`: Regenerate with `sqlc generate` to expose the new query.
- `internal/watcher/watcher.go`: Add exported `TriggerRescan(collectionName string)` method that marks the collection's directory dirty so the watcher's debounce loop re-scans it on the next cycle.
- `internal/server/routes.go`: Pass `s.queries` and `s.watcher` to `TriggerReindex` (currently the handler takes only a logger).

## Capabilities

### New Capabilities
- `reindex-api`: `POST /api/v1/reindex` actually marks collection chunks pending and triggers a watcher rescan, returning 202 once both operations succeed.

### Modified Capabilities
- (none — no spec-level requirement changes to existing capabilities)

## Impact

- **API contract**: Response shape unchanged (`{ "status": "queued", "message": "..." }`); behavior changes from no-op to real work. Callers that called this endpoint expecting a no-op could see unexpected reprocessing — but those callers were broken anyway.
- **Storage**: New SQL query touches `chunks` table (UPDATE). Scoped to workspace + collection via a JOIN on `documents`. No schema migration required.
- **Watcher**: A new exported method is added; no behavioural change to the existing `Watch`/`Unwatch`/`Run` contract.
- **Handler signature change**: `TriggerReindex(logger)` → `TriggerReindex(queries, watcher, logger)`. Routes must be updated.
- **No new external dependencies.**
