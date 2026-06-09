## Why

The `UpsertChunk` SQL query unconditionally resets `embed_status` to `'pending'` on conflict, causing already-embedded chunks to be re-embedded repeatedly. This wastes embedding API calls, increases processing time, and generates noisy logs showing the same chunks being embedded multiple times.

## What Changes

- Modify the `UpsertChunk` query to preserve `embed_status` when chunk content hasn't changed
- Only reset `embed_status` to `'pending'` when the actual chunk content is different
- Add a guard to prevent unnecessary re-embedding of unchanged chunks

## Capabilities

### New Capabilities

- `embed-status-preservation`: Preserve embed_status during chunk upsert when content is unchanged

### Modified Capabilities

<!-- None - this is a bug fix, not a requirement change -->

## Impact

- **Code**: `internal/storage/queries/chunks.sql` (UpsertChunk query)
- **Behavior**: Chunks that were already embedded will no longer be re-queued for embedding when the watcher re-processes unchanged files
- **Performance**: Reduced embedding API calls, faster re-indexing
- **Logs**: Cleaner logs without repetitive "embedding chunk" messages for the same content
