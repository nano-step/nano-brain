## Why

`/ui/memory` page crashes with `Failed to load documents: 404 Not Found`. Frontend hook `useDocuments` calls `GET /api/v1/query?workspace=<hash>...` expecting a JSON document list. Backend only has `POST /api/v1/query` for hybrid search (BM25+vector). Method + semantic mismatch.

Memory page renders a filtered list of documents (by text/tags/collection). It's NOT a search — it's a paginated list. Hybrid search returns ranked snippets, not full documents.

This blocks `/ui/memory` and dependent flows (DocDrawer document detail/delete).

## What Changes

- Add `GET /api/v1/documents?workspace=<hash>&text=...&tags=...&collection=...` endpoint that lists documents for the workspace with optional filters.
- Reuse existing `ListDocumentsByWorkspace` SQL query; add filter parameters at the handler level (in-memory filter since memory page expects small result sets).
- Update frontend `useDocuments` hook to call `/api/v1/documents` instead of `/api/v1/query`.
- Update frontend type expectation: response is `{documents: [...]}` (wrapped object, consistent with #277 workspaces fix).
- Add `DELETE /api/v1/documents/:id` endpoint (used by DocDrawer to delete a doc).
- Tests: handler test for list + delete, frontend test mock update.
- E2E verify Memory page loads + filters work.

## Capabilities

### New Capabilities
- `documents-list-endpoint`: Defines `GET /api/v1/documents` shape and filters. Defines `DELETE /api/v1/documents/:id` semantics.

### Modified Capabilities
None.

## Impact

- **Code:** new handler `documents.go` (list + delete), routes.go wiring, frontend hook update, frontend test mock.
- **Behavior:** /ui/memory loads. DocDrawer delete works.
- **Risk:** Low — additive endpoints, no existing API behavior changes.
- **Performance:** in-memory filter on `ListDocumentsByWorkspace` rows. Workspaces typically have < 10k docs; acceptable for v1. SQL-level filter is a follow-up if needed.
- **Database:** no migration.

## Out of Scope

- POST /api/v1/documents (create) — already exists as POST /api/v1/write
- PATCH/PUT /api/v1/documents/:id (update) — already exists as POST /api/v1/write (upsert)
- Other FE/BE mismatches discovered during E2E — separate issues
