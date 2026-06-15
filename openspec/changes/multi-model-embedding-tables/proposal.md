## Why

nano-brain currently hardcodes all embeddings into a single `embeddings` table with a `vector(768)` column, tied to the `nomic-embed-text` model. This prevents switching to higher-quality embedding models (e.g., `bge-m3` with 1024d) without dropping existing embeddings and re-embedding everything from scratch. As the project grows, the ability to experiment with different embedding models — and retain past embeddings when switching — is essential for improving search quality without data loss.

## What Changes

- Add `embedding.table_name` config field in `EmbeddingConfig` (default: `embeddings`) so users can specify which table holds the embeddings for the current model.
- Add a new `embed_1024` table (1024d, `bge-m3`) via Goose migration, alongside the existing `embeddings` table (768d, `nomic-embed-text`).
- Add sqlc queries for insert + vector search on `embed_1024` — parallel to existing queries but referencing the new table.
- Modify the embed queue to write to `config.embedding.table_name` instead of the hardcoded `embeddings` table.
- Modify the vector search pipeline to query `config.embedding.table_name` instead of hardcoding `embeddings` in SQL.
- Update default `.nano-brain/config.yml` to switch to `model: bge-m3`, `table_name: embed_1024`.
- **No breaking changes** — existing `embeddings` table and data are untouched. Rollback by reverting config to `table_name: embeddings`.

## Capabilities

### New Capabilities
- `embedding-storage-routing`: Config-driven routing that selects which DB table to read/write embeddings based on `embedding.table_name`. Includes the config field definition, validation, and a thin routing layer in the embed queue + search pipeline.

### Modified Capabilities
- `embed-queue`: The embed queue's `InsertEmbedding` query currently hardcodes the `embeddings` table. It must accept a config-driven table name so new embeddings go to `embed_1024` (or whatever table the config specifies). The queue itself — chunk scanning, in-flight dedup, retry logic — is unchanged.
- `search-pipeline`: The vector search queries (`VectorSearch`, `VectorSearchAll`, `VectorSearchWithTags`, `VectorSearchAllWithTags`) currently hardcode `FROM embeddings`. They must accept a config-driven table name so searches query the correct embedding table.

## Impact

- **Config**: `embedding.table_name` added to `EmbeddingConfig`. Existing configs without this field default to `embeddings` (backward compatible).
- **DB**: New `embed_1024` migration creates a table structurally identical to `embeddings` but with `vector(1024)`. Existing data unchanged.
- **sqlc queries**: Duplicate set of vector-search queries for `embed_1024` (or generic via dynamic SQL). Alternatively, use dynamic table names via sqlc's `/* name */` annotation or raw SQL in the query layer.
- **Embed queue**: `chunk_embedded` handler reads `cfg.Embedding.TableName` and writes to that table. Affects `insertEmbedding` in `internal/embed/`.
- **Search pipeline**: `service.go` passes `cfg.Embedding.TableName` to the query builder. Affects `VectorSearch` and related calls in `internal/search/`.
- **No new dependencies** — all changes are within the Go codebase + sqlc queries + Goose migration.
- **No API changes** — the REST and MCP interfaces are unchanged.
