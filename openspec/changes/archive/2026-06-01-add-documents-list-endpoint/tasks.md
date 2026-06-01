## 1. SQL changes

- [x] 1.1 Update `ListDocumentsByWorkspace` to also SELECT `supersedes_id` and computed `superseded_by_id` via LEFT JOIN
- [x] 1.2 Add `DeleteDocumentByIDAndWorkspace :exec` query (workspace scope ensures cross-workspace delete protection)
- [x] 1.3 `sqlc generate` to regenerate bindings

## 2. Backend handler

- [x] 2.1 Create `internal/server/handlers/documents.go` with `ListDocuments` constructor
- [x] 2.2 Parse query params: `text`, `tags`, `collection`
- [x] 2.3 Call `ListDocumentsByWorkspace`, filter in-memory by params
- [x] 2.4 Return `{documents: [...]}` wrapped response
- [x] 2.5 Create `DeleteDocument` constructor in same file
- [x] 2.6 Cascade-delete via SQL FK

## 3. Routes

- [x] 3.1 Register `data.GET("/documents", handlers.ListDocuments(...))` in routes.go
- [x] 3.2 Register `data.DELETE("/documents/:id", handlers.DeleteDocument(...))`

## 4. Frontend

- [x] 4.1 Update `web/src/hooks/useDocuments.ts` to call `/api/v1/documents` not `/api/v1/query`
- [x] 4.2 Verify DocDrawer.tsx already uses `/api/v1/documents/:id` for delete

## 5. Tests

- [x] 5.1 `TestListDocuments_ResponseShape` — wrapped object, fields match spec
- [x] 5.2 `TestListDocuments_EmptyWorkspace`
- [x] 5.3 `TestListDocuments_FilterByCollection`
- [x] 5.4 `TestDeleteDocument_Success`
- [x] 5.5 `TestDeleteDocument_NotFound`

## 6. Verification

- [x] 6.1 `go build ./...` exit 0
- [x] 6.2 `go vet ./...` clean
- [x] 6.3 `go test -race -short ./...` ALL PASS
- [x] 6.4 Rebuild dev binary on port 3199
- [x] 6.5 `curl /api/v1/documents?workspace=<hash>` returns wrapped shape
- [x] 6.6 Browser DevTools: /ui/memory loads, lists documents

## 7. PR + Review

- [x] 7.1 Commit + push
- [x] 7.2 Open PR, wait CI + Gemini
- [x] 7.3 Address findings
- [x] 7.4 Merge + close issue #281

## 8. Archive + Release

- [x] 8.1 Archive openspec
- [x] 8.2 Tag next v2026.6.X
