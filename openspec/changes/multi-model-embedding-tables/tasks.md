## 1. Config

- [ ] 1.1 Add `TableName string` field to `EmbeddingConfig` struct with koanf tag `table_name` and default `"embeddings"`
- [ ] 1.2 Add config validation in `validate()`: `table_name` must be non-empty and match `^[a-z][a-z0-9_]*$`
- [ ] 1.3 Add `ModelDimension` helper to `EmbeddingConfig` that returns the expected embedding dimension for the configured model (768 for nomic-embed-text, 1024 for bge-m3)

## 2. Database Migration

- [ ] 2.1 Create `00024_embed_1024_table.sql` migration with:
  - `CREATE TABLE embed_1024` with identical columns to `embeddings` but `vector(1024)` for the embedding column
  - `CREATE INDEX ON embed_1024 USING hnsw (embedding vector_cosine_ops)`
  - Reversible `DROP TABLE IF EXISTS embed_1024 CASCADE` in down migration
- [ ] 2.2 Run migration against test database and verify both up and down

## 3. sqlc Queries for embed_1024

- [ ] 3.1 Create `internal/storage/queries/embeddings_1024.sql` with parallel queries for `embed_1024` table:
  - `InsertEmbedding1024` — INSERT into embed_1024
  - `VectorSearch1024` — vector search on embed_1024 with workspace filter
  - `VectorSearchAll1024` — vector search across all workspaces
  - `VectorSearchWithTags1024` — vector search with tags + workspace
  - `VectorSearchAllWithTags1024` — vector search with tags across all
  - `DeleteEmbeddingsByWorkspace1024` — delete by workspace
  - `CountEmbeddingsByWorkspace1024` — count by workspace
- [ ] 3.2 Run `sqlc generate` and verify no errors
- [ ] 3.3 Verify generated Go types have distinct names (e.g., `Embedding1024`, `VectorSearch1024Row`)

## 4. Table-Routing Wrapper

- [ ] 4.1 Create `internal/search/router.go` with `TableRoutingQuerier` struct
- [ ] 4.2 Implement all `Querier` interface methods on `TableRoutingQuerier` (delegating to 768d or 1024d queries based on `tableName`)
- [ ] 4.3 Create `internal/embed/embed_querier.go` with embed-specific routing for `InsertEmbedding`
- [ ] 4.4 Wire the routing wrappers in `cmd/nano-brain/` main construction path

## 5. Embed Queue Wiring

- [ ] 5.1 Update embed `queue.Querier` interface to support both embedding tables (or use a routing wrapper)
- [ ] 5.2 Pass the table-aware querier to Queue constructor in `cmd/nano-brain/`
- [ ] 5.3 Verify `makeVec` / vector dimension helpers use the correct dimension from config

## 6. Search Pipeline Wiring

- [ ] 6.1 Replace direct `*sqlc.Queries` injection with `*TableRoutingQuerier` in search service construction
- [ ] 6.2 Verify MCP tools (`tools.go`) and HTTP handlers (`handlers/search.go`) use the routing wrapper
- [ ] 6.3 Update tests with the routing wrapper (add mock methods for 1024 queries or use the router)

## 7. Default Config Update

- [ ] 7.1 Update `getDefaults()` in `config.go` to set `TableName: "embeddings"` (existing default, no behavior change)
- [ ] 7.2 Update `~/.nano-brain/config.yml` example in README.md to document the `table_name` field
- [ ] 7.3 Update config docstring on `EmbeddingConfig` to explain table selection

## 8. Verification

- [ ] 8.1 Run `go build ./...` — must compile without errors
- [ ] 8.2 Run `go test -race -short ./...` — all tests pass
- [ ] 8.3 Run `go test -race -tags=integration ./...` — integration tests pass (including vector search isolation tests)
- [ ] 8.4 Re-run benchmark to confirm no regression on existing `embeddings` table
