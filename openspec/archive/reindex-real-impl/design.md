## Context

`POST /api/v1/reindex` is registered at `internal/server/routes.go:42` as:

```go
data.POST("/reindex", handlers.TriggerReindex(s.logger))
```

`TriggerReindex` (handlers/reindex.go:21-44) currently only logs and returns 202. No storage interaction, no watcher interaction.

The watcher (`internal/watcher/watcher.go`) runs as a goroutine alongside the HTTP server. It maintains a map of `watchedCollection` entries (keyed by absolute directory path) and a `dirty` map. When the `dirty` map has entries, `processDirty` re-scans those directories. All internal methods that drive rescanning (`processDirty`, `processAll`, `scanCollection`) are unexported.

The embed queue (`internal/embed/queue.go`) has a `scanPending` loop that picks up chunks with `embed_status='pending'` and processes them. It is already wired and running. The existing `ResetEmbedStatus` SQL query resets embed_status for an entire workspace — not scoped to a collection.

**Dependency injection of watcher into the handler:**  
`s.watcher` is a `*watcher.Watcher` already stored on the `Server` struct. Other handlers (e.g., `AddCollection`, `RemoveCollection`, `RenameCollectionHandler`) already receive `s.watcher` as a parameter. The pattern is established: extend `TriggerReindex` to accept `queries` and `watcher`.

## Goals / Non-Goals

**Goals:**
- `POST /api/v1/reindex` marks all chunks for the named collection as `embed_status='pending'` so the embed queue re-embeds them.
- `POST /api/v1/reindex` triggers the watcher to rescan the collection's directory so new/modified files are ingested.
- Handler signature updated; routes updated to pass `s.queries` and `s.watcher`.
- New SQL query scoped to collection (not whole workspace).
- New `TriggerRescan` method exported from watcher for handler use.

**Non-Goals:**
- No database schema migration (no new columns or tables).
- No change to the response shape or HTTP status code.
- No change to `TriggerUpdate` (the whole-workspace update endpoint) — leave it as-is for now.
- No cleanup of stale files (the existing TODO in watcher for deletion handling is separate).

## Decisions

### Decision 1: Where does the collection-scoped "mark pending" happen?

**Options:**
- A) Handler queries storage directly (new SQL query, new sqlc method).
- B) Watcher's new `TriggerRescan` method also handles the mark-pending step.

**Choice: A** — Handler calls storage to mark chunks pending, then calls watcher to rescan. These are two separate concerns. The watcher should not own the "reset embed status" responsibility; that belongs to the storage layer. Keeping them separate makes each piece testable in isolation.

### Decision 2: How does the handler find the collection's directory path for the watcher?

The watcher's `collections` map is keyed by absolute directory path. The handler knows the collection name and workspace hash from the request. It must look up the collection's path from storage.

**Options:**
- A) Add a new SQL query `GetCollectionByName` and pass the result to watcher.
- B) Add a `TriggerRescanByName(collectionName, workspaceHash string)` method to watcher that does the lookup itself using its own `collections` map.

**Choice: B** — The watcher already holds a `collections` map indexed by directory path that maps to `watchedCollection{name, dirPath, workspaceHash, ...}`. A reverse lookup by `(collectionName, workspaceHash)` is O(n) over watched collections (typically tiny). This avoids an extra SQL round-trip and a new query. The watcher's map is the authoritative in-memory state for what is currently being watched.

### Decision 3: New SQL query scope

The existing `ResetEmbedStatus` resets all chunks in a workspace. We need a collection-scoped version. Since `embed_status` lives on `chunks` and collection lives on `documents`, the new query needs a JOIN:

```sql
-- name: ResetEmbedStatusByCollection :exec
UPDATE chunks SET embed_status = 'pending'
FROM documents
WHERE chunks.document_id = documents.id
  AND chunks.workspace_hash = $1
  AND documents.collection = $2;
```

This requires `sqlc generate` to regenerate `internal/storage/sqlc/embeddings.sql.go`.

### Decision 4: Watcher `TriggerRescanByName` implementation

```go
// TriggerRescanByName marks the directory for the named collection as dirty,
// causing the watcher's debounce loop to rescan it on the next cycle.
// Returns false if the collection is not currently watched (not an error — the
// handler should still return 202 since the mark-pending step succeeded).
func (w *Watcher) TriggerRescanByName(collectionName, workspaceHash string) bool {
    w.mu.Lock()
    defer w.mu.Unlock()
    for absPath, col := range w.collections {
        if col.name == collectionName && col.workspaceHash == workspaceHash {
            w.dirty[absPath] = true
            return true
        }
    }
    return false
}
```

The watcher's `Run` loop will pick up the dirty entry on the next debounce tick and call `processDirty` → `scanCollection`. This is fire-and-forget from the handler's perspective, matching the existing 202 Accepted semantics.

### Decision 5: Handler signature

```go
func TriggerReindex(queries ReindexQuerier, w *watcher.Watcher, logger zerolog.Logger) echo.HandlerFunc
```

A new minimal interface `ReindexQuerier` (in the handlers package) wraps the single method needed:

```go
type ReindexQuerier interface {
    ResetEmbedStatusByCollection(ctx context.Context, arg sqlc.ResetEmbedStatusByCollectionParams) error
}
```

`*sqlc.Queries` satisfies this interface automatically once the new query is generated.

The route registration becomes:

```go
data.POST("/reindex", handlers.TriggerReindex(s.queries, s.watcher, s.logger))
```

## Exact Execution Flow

```
POST /api/v1/reindex  {root: "my-collection"}
  │
  ├─ middleware: extract workspace hash → c.Set("workspace", wsHash)
  │
  ├─ TriggerReindex handler:
  │    1. Bind request → {workspace, root}
  │    2. Validate root != ""
  │    3. queries.ResetEmbedStatusByCollection(ctx, {workspace_hash, collection})
  │       → UPDATE chunks SET embed_status='pending' FROM documents WHERE ...
  │    4. watcher.TriggerRescanByName(root, workspace)
  │       → marks collection dir dirty in watcher.dirty map
  │    5. Return 202 {status:"queued", message:"Reindex queued for collection X..."}
  │
  ├─ watcher goroutine (debounce cycle):
  │    processDirty() → scanCollection() → processFile() per file
  │    → upsertWithTx() → upsert document + delete+rewrite chunks
  │
  └─ embed queue goroutine (scanPending loop):
       GetPendingChunks() → processChunk() → embed → MarkChunkEmbedded()
```

## Files Changed

| File | Change |
|------|--------|
| `internal/storage/queries/embeddings.sql` | Add `ResetEmbedStatusByCollection` query |
| `internal/storage/sqlc/embeddings.sql.go` | Regenerated by `sqlc generate` |
| `internal/watcher/watcher.go` | Add exported `TriggerRescanByName(name, wsHash string) bool` |
| `internal/server/handlers/reindex.go` | Replace stub with real impl; add `ReindexQuerier` interface |
| `internal/server/routes.go` | Pass `s.queries`, `s.watcher` to `TriggerReindex` |

## Risks / Trade-offs

- **Watcher not running or collection not watched** → `TriggerRescanByName` returns false; handler still returns 202 (mark-pending succeeded). The embed queue will process the pending chunks regardless. The watcher rescan is a best-effort optimization to pick up file changes; it is not required for the embed-status reset to take effect.

- **Large collections** → Marking many chunks pending triggers a large embed queue flush. This is the correct behaviour — the caller explicitly requested a reindex. The embed queue processes concurrently and non-blocking relative to the HTTP handler.

- **sqlc generate required** → Any contributor must run `sqlc generate` after adding the SQL query. This is a standard workflow step for this codebase and is documented in the README. No CI breakage if they forget — the generated file is committed.

- **Race between mark-pending and watcher rescan** → If the watcher rescans and re-upserts documents before the embed queue processes the pending chunks, the upsert will delete and recreate chunks (each created with `embed_status='pending'` by default from the schema). This is safe — the net result is the same.

## Open Questions

None — all design decisions are resolved above.
