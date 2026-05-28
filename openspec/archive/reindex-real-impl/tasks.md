## 1. Storage: Add collection-scoped embed reset query

- [x] 1.1 Add `ResetEmbedStatusByCollection` query to `internal/storage/queries/embeddings.sql` — `UPDATE chunks SET embed_status = 'pending' FROM documents WHERE chunks.document_id = documents.id AND chunks.workspace_hash = $1 AND documents.collection = $2`
- [x] 1.2 Run `sqlc generate` to regenerate `internal/storage/sqlc/embeddings.sql.go` with the new `ResetEmbedStatusByCollection` function and `ResetEmbedStatusByCollectionParams` struct
- [x] 1.3 Verify `CGO_ENABLED=0 go build ./...` passes after sqlc regeneration

## 2. Watcher: Expose TriggerRescanByName

- [x] 2.1 Add exported method `TriggerRescanByName(collectionName, workspaceHash string) bool` to `internal/watcher/watcher.go` — iterates `w.collections` under `w.mu`, marks the matching directory dirty, returns true if found
- [x] 2.2 Verify `CGO_ENABLED=0 go build ./...` passes after watcher change

## 3. Handler: Replace stub with real implementation

- [x] 3.1 Add `ReindexQuerier` interface to `internal/server/handlers/reindex.go` with single method `ResetEmbedStatusByCollection(ctx, arg) error`
- [x] 3.2 Update `TriggerReindex` signature to `TriggerReindex(queries ReindexQuerier, w *watcher.Watcher, logger zerolog.Logger) echo.HandlerFunc`
- [x] 3.3 Implement handler body: bind request → validate root → call `queries.ResetEmbedStatusByCollection` → call `w.TriggerRescanByName` → return 202
- [x] 3.4 Verify `CGO_ENABLED=0 go build ./...` passes with new handler

## 4. Routes: Wire dependencies

- [x] 4.1 Update `internal/server/routes.go` line 42 from `handlers.TriggerReindex(s.logger)` to `handlers.TriggerReindex(s.queries, s.watcher, s.logger)`
- [x] 4.2 Verify `CGO_ENABLED=0 go build ./...` passes

## 5. Tests

- [x] 5.1 Add unit test for `watcher.TriggerRescanByName` — test: collection found marks dirty and returns true; collection not found returns false
- [x] 5.2 Add handler test for `TriggerReindex` — test: valid request calls `ResetEmbedStatusByCollection` and `TriggerRescanByName`; missing root returns 400
- [x] 5.3 Run full validation ladder: `CGO_ENABLED=0 go build ./... && go vet ./... && go test -race -short ./...`
