## Context

The embed queue processes chunks asynchronously. When a file is re-indexed (due to file changes, server startup, or periodic polling), the watcher calls `writeChunks` which deletes existing chunks and re-creates them via `UpsertChunk`. The current `UpsertChunk` query uses an UPSERT that unconditionally resets `embed_status = 'pending'` on conflict, even when the chunk content hasn't changed. This causes already-embedded chunks to be re-queued and re-embedded unnecessarily.

Current flow:
1. File indexed → chunks created with `embed_status = 'pending'`
2. Chunks embedded → status updated to `'embedded'`
3. File re-indexed (unchanged content) → `UpsertChunk` resets status to `'pending'`
4. Chunks re-embedded unnecessarily

## Goals / Non-Goals

**Goals:**
- Preserve `embed_status` when chunk content is unchanged during upsert
- Only reset to `'pending'` when chunk content actually changes
- Maintain backward compatibility with existing data
- Minimal code change (single SQL query modification)

**Non-Goals:**
- Changing the embedding queue architecture
- Modifying how the watcher decides to re-index files
- Adding new database columns or migrations
- Changing the embed_status enum values

## Decisions

### Decision 1: Conditional embed_status reset in UPSERT

**Choice**: Use a CASE expression in the UPSERT's SET clause to compare content and conditionally reset embed_status.

**Rationale**: 
- PostgreSQL's UPSERT (INSERT ... ON CONFLICT DO UPDATE) allows conditional logic in the SET clause
- We can compare `EXCLUDED.content` (new content) with `chunks.content` (existing content)
- If they differ, reset to 'pending'; otherwise preserve current status

**Alternative considered**: 
- Adding a trigger to manage embed_status → Rejected: adds complexity, harder to debug, performance overhead
- Checking embed_status before upsert in application code → Rejected: race conditions, more complex logic

### Decision 2: Compare content directly, not content_hash

**Choice**: Compare `EXCLUDED.content != chunks.content` rather than relying on the conflict key.

**Rationale**:
- The conflict is on `(content_hash, workspace_hash, document_id)`
- If content_hash matches, the UPSERT conflict triggers
- But we need to check if content actually changed to decide on embed_status
- Direct content comparison is clearer and handles edge cases

## Risks / Trade-offs

**Risk**: Content comparison might be slightly slower for very large chunks
→ **Mitigation**: Chunks are already truncated to 3000 chars (defaultMaxEmbedChars), so comparison is fast

**Risk**: Existing chunks with `embed_status = 'embedded'` that get upserted with same content will now correctly stay 'embedded'
→ **Mitigation**: This is the desired behavior, not a risk

**Risk**: If a chunk was previously failed (`embed_status = 'embed_failed'`) and content hasn't changed, it won't be retried
→ **Mitigation**: This is correct - if content hasn't changed, there's no reason to retry embedding. Users can force re-embedding via `POST /api/v1/update` if needed.
