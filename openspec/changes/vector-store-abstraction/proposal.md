## Why

nano-brain's vector storage uses sqlite-vec (a SQLite virtual table extension) which has critical reliability issues. The SudoX workspace database was corrupted (`database disk image is malformed`) due to an interrupted embed process — a known weakness of SQLite's single-writer WAL model under concurrent/interrupted writes. This corruption is unrecoverable without a full rebuild (re-scan + re-embed all documents, burning Voyage AI API credits).

Beyond reliability, sqlite-vec limits future growth: no concurrent writes, no production-grade ANN indexing (HNSW), no native filtering during vector search, and a practical ~500MB size ceiling. The current implementation is also tightly coupled — `store.ts` mixes vector operations directly with relational queries, making it impossible to swap backends without rewriting the storage layer.

Current state: 4 healthy DBs with ~49K vectors (all voyage-code-3, 1024-dim), 1 corrupted (SudoX). Vectors can be exported directly to Qdrant without re-embedding.

## What Changes

- **Extract `VectorStore` interface**: Define a provider-agnostic contract for vector operations (search, upsert, delete, health) separate from the relational `Store` interface
- **Implement `QdrantVectorStore`**: First provider using Qdrant's REST API via `@qdrant/js-client-rest`, supporting both local Docker and cloud deployments
- **Keep `SqliteVecStore` as fallback**: Wrap existing sqlite-vec logic behind the same interface for zero-dependency local usage
- **Generalize Docker host detection**: Extract existing `detectOllamaUrl()` pattern into a shared `resolveHostUrl()` utility that auto-resolves `localhost` → `host.docker.internal` inside containers, applicable to any provider URL
- **Add `vector` config section**: Provider selection, URL, API key, and collection name in `config.yml`
- **Dual-store architecture**: SQLite keeps metadata, FTS5/BM25, and cache. VectorStore handles only embeddings and similarity search. Clean separation.
- **Qdrant via Docker Compose**: Ship a `docker-compose.qdrant.yml` for one-command Qdrant setup. Assumes user has Docker installed.
- **CLI commands for Qdrant lifecycle**: `npx nano-brain qdrant up|down|status|migrate` to manage the Qdrant container and migrate existing vectors from SQLite.
- **Zero re-embed migration**: Export existing 49K vectors from SQLite `vectors_vec` directly into Qdrant — no Voyage AI API calls needed.

## Capabilities

### New Capabilities
- `vector-store-interface`: Provider-agnostic `VectorStore` interface with search/upsert/delete/health contracts
- `qdrant-provider`: Qdrant vector store implementation with HNSW indexing, concurrent writes, crash recovery, and payload filtering
- `container-host-resolution`: Shared utility that auto-detects Docker/containerd and resolves localhost URLs to host.docker.internal
- `vector-provider-config`: YAML config section for selecting vector backend (qdrant, sqlite-vec, future: pinecone, chroma, weaviate)
- `qdrant-docker-compose`: Bundled `docker-compose.qdrant.yml` for one-command Qdrant setup with persistent volume
- `qdrant-cli`: CLI subcommands: `qdrant up` (start container), `qdrant down` (stop), `qdrant status` (health + vector count), `qdrant migrate` (export SQLite vectors → Qdrant)
- `vector-migration`: Zero-cost migration tool that reads existing vectors from SQLite `vectors_vec` + `content_vectors` and batch-upserts into Qdrant without re-embedding

### Modified Capabilities
- `search-pipeline`: `searchVec()` delegates to the configured VectorStore provider instead of direct sqlite-vec queries
- `embedding-indexing`: `insertEmbedding()` routes through VectorStore provider; batch upsert support for Qdrant
- `ollama-detection`: Existing `detectOllamaUrl()` refactored to use shared `resolveHostUrl()` utility (DRY)
- `cli-help`: Updated help text with `qdrant` subcommand documentation

## Impact

- **New files**: `src/vector-store.ts` (interface + factory), `src/providers/qdrant.ts`, `src/providers/sqlite-vec.ts`, `src/host.ts`, `docker-compose.qdrant.yml`
- **Modified files**: `store.ts` (extract vector ops), `embeddings.ts` (refactor detectOllamaUrl), `search.ts` (use VectorStore), `codebase.ts` (use VectorStore for insertEmbedding), `index.ts` (config parsing + qdrant CLI commands), `types.ts` (new interfaces)
- **Dependencies**: Add `@qdrant/js-client-rest` (optional peer dep — only needed if provider=qdrant)
- **Config**: New `vector:` section in config.yml with provider/url/apiKey/collection fields
- **Database**: No schema changes to SQLite tables. `vectors_vec` virtual table becomes unused when provider≠sqlite-vec.
- **Breaking**: None. Default provider=sqlite-vec preserves current behavior. Qdrant is opt-in.
- **Migration**: `npx nano-brain qdrant migrate` exports existing vectors from SQLite → Qdrant (zero API cost). Existing SQLite metadata/FTS untouched.
