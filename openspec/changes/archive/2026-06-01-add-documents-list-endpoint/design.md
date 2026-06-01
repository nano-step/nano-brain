## Context

Memory page (`web/src/panels/MemoryPanel.tsx`) lists documents filtered by text/tags/collection. The hook `useDocuments` makes a single request to fetch documents.

Frontend Document interface:
```ts
interface Document {
  id, title, collection, tags, updated_at, created_at,
  supersedes_id, superseded_by_id, content, metadata
}
```

But MemoryPanel only consumes: `id, title, collection, tags, updated_at, supersedes_id, superseded_by_id`. Content + metadata are loaded on-demand by DocDrawer via `POST /api/v1/get`.

Backend already has `ListDocumentsByWorkspace` SQL query returning all needed fields except `supersedes_id` + `superseded_by_id`. Need to extend the query or add a new one.

## Goals / Non-Goals

**Goals:**
- Add `GET /api/v1/documents` returning `{documents: [...]}` for Memory page.
- Add `DELETE /api/v1/documents/:id` for DocDrawer delete.
- Support filters: text (title substring), tags (intersection), collection (exact).
- Match frontend Document interface for fields actually consumed.

**Non-Goals:**
- SQL-level filtering (use in-memory; defer until perf issue).
- Full-text search (use existing /query endpoint).
- Pagination (defer).
- Update/create endpoints (already exist).

## Decisions

### D1: Wrapped response shape `{documents: [...]}`

Consistent with #277 workspaces fix. Frontend hook already handles both shapes (`Array.isArray(data) ? data : (data.documents ?? [])`), so wrapping is non-breaking.

### D2: Extend `ListDocumentsByWorkspace` to include supersedes_id

Add `supersedes_id` column to SELECT. `superseded_by_id` is reverse-derived: a doc with id=X is superseded by Y if Y.supersedes_id=X. Need a single query that JOINs documents to itself on supersedes_id. Computed via LEFT JOIN.

### D3: In-memory filter for v1

Memory page has client-side filters too. Backend pre-filters by workspace (already indexed). Text/tags/collection filtering happens in handler iterating rows. Faster than adding 3 indexed paths to SQL; safe up to ~10K docs.

### D4: DELETE handler reuses cascade pattern

Existing `RemoveWorkspace` handler cascades doc → chunks → embeddings via SQL. Single-document delete follows same pattern via existing `DeleteDocument` query (if exists) or new `DeleteDocumentByID` query that cascades through chunks (FK ON DELETE CASCADE).

### D5: Route group placement

Both routes go on `data` group (has workspaceMiddleware). DELETE additionally needs auth/csrf via `write` group if available. Match existing `DELETE /api/v1/workspaces/:hash` pattern (on `api` group with no csrf — it's a manual destructive op).

## Risks / Trade-offs

| Risk | Mitigation |
|---|---|
| In-memory filter slow on large workspaces | Acceptable v1. Add SQL filter when >10K docs reported. |
| superseded_by_id requires self-join | Add LEFT JOIN clause; cost negligible due to FK index. |
| DELETE without csrf | Match existing workspace delete pattern. Document risk in commit. |
