# Tasks: Vector Store Abstraction

## Phase 1: Foundation (no behavior change)

### 1.1 Create host resolution utility
- [ ] Create `src/host.ts` with `isInsideContainer()` and `resolveHostUrl()`
- [ ] Cache detection result (check once, reuse)
- [ ] Handle: /.dockerenv, /proc/1/cgroup (docker + containerd)
- [ ] Regex replace: localhost and 127.0.0.1 → host.docker.internal
- [ ] Unit test: mock fs access for container/non-container scenarios

### 1.2 Refactor detectOllamaUrl
- [ ] Replace `detectOllamaUrl()` in embeddings.ts with `resolveHostUrl('http://localhost:11434')`
- [ ] Remove inline Docker detection code from embeddings.ts
- [ ] Verify Ollama connection still works in both Docker and native environments

### 1.3 Define VectorStore interface
- [ ] Create `src/vector-store.ts` with interface, types, and factory function
- [ ] Define: VectorStore, VectorPoint, VectorSearchResult, VectorSearchOptions, VectorStoreHealth
- [ ] Export factory: `createVectorStore(config, db?)`

### 1.4 Extract SqliteVecStore
- [ ] Create `src/providers/sqlite-vec.ts`
- [ ] Move vector-specific code from store.ts: searchVec SQL, insertEmbedding, ensureVecTable, cleanOrphanedEmbeddings
- [ ] Implement VectorStore interface (wrap sync SQLite calls in async)
- [ ] Store.ts constructor accepts VectorStore dependency
- [ ] Verify: all existing tests pass, zero behavior change

## Phase 2: Qdrant Provider + Docker

### 2.1 Docker Compose setup
- [ ] Create `docker-compose.qdrant.yml` in nano-brain project root
- [ ] Qdrant image with persistent volume `nano-brain-qdrant-data`
- [ ] Expose ports 6333 (REST) and 6334 (gRPC)
- [ ] `restart: unless-stopped` for auto-recovery

### 2.2 Implement QdrantVecStore
- [ ] Create `src/providers/qdrant.ts`
- [ ] Add `@qdrant/js-client-rest` as optional peer dependency
- [ ] Implement: ensureCollection (create if not exists, 1024-dim cosine distance)
- [ ] Implement: search (with collection/projectHash payload filters)
- [ ] Implement: upsert + batchUpsert (500 points/batch chunking)
- [ ] Implement: delete + deleteByHash (payload filter on hash field)
- [ ] Implement: health (collection info → vector count, status)
- [ ] Use resolveHostUrl() for URL resolution

### 2.3 Add vector config section
- [ ] Extend config.yml schema: vector.provider, vector.url, vector.apiKey, vector.collection
- [ ] Parse in index.ts config loading
- [ ] Default: provider=sqlite-vec (backward compatible)
- [ ] Validate: warn if provider=qdrant but Qdrant unreachable

### 2.4 Wire into search pipeline
- [ ] search.ts: searchVec() delegates to vectorStore.search() + SQLite metadata JOIN
- [ ] codebase.ts: insertEmbedding routes through vectorStore.upsert/batchUpsert
- [ ] store.ts: clearWorkspace/cleanOrphanedEmbeddings calls vectorStore.deleteByHash

## Phase 3: CLI Commands

### 3.1 `qdrant up`
- [ ] Copy docker-compose.qdrant.yml to ~/.nano-brain/ if not exists
- [ ] Run `docker compose -f ~/.nano-brain/docker-compose.qdrant.yml up -d`
- [ ] Wait for health check: GET http://localhost:6333/healthz (retry 5x, 2s interval)
- [ ] Auto-update config.yml: set vector.provider to qdrant, vector.url to http://localhost:6333
- [ ] Print success message with Qdrant dashboard URL

### 3.2 `qdrant down`
- [ ] Run `docker compose -f ~/.nano-brain/docker-compose.qdrant.yml down`
- [ ] Auto-update config.yml: set vector.provider back to sqlite-vec
- [ ] Print message: data persists in Docker volume

### 3.3 `qdrant status`
- [ ] Check if container is running (docker compose ps)
- [ ] GET Qdrant health endpoint
- [ ] Show: collection name, vector count, dimensions, index status
- [ ] Show: container status, uptime, memory usage

### 3.4 `qdrant migrate`
- [ ] Accept flags: --workspace=<path>, --batch-size=<n>, --dry-run
- [ ] Verify Qdrant is running (health check, fail fast if not)
- [ ] Create collection if not exists (1024-dim, cosine)
- [ ] For each workspace SQLite DB:
  - [ ] Load sqlite-vec extension
  - [ ] Query: SELECT cv.hash, cv.seq, cv.pos, cv.model, cv.project_hash, vv.embedding FROM content_vectors cv JOIN vectors_vec vv ON cv.hash || ':' || cv.seq = vv.hash_seq
  - [ ] Also resolve collection name from documents table (JOIN documents ON cv.hash = d.hash)
  - [ ] Batch upsert into Qdrant (default 500/batch) with payload: hash, seq, pos, model, projectHash, collection
  - [ ] Progress bar: [workspace] 21714/21714 vectors migrated
- [ ] --dry-run: show counts per workspace without writing
- [ ] Summary: total vectors migrated, time elapsed, any errors

### 3.5 Update help text and status command
- [ ] Add `qdrant` subcommand to showHelp() output
- [ ] Update `status` command to show vector provider info (provider name, health, vector count)
- [ ] MCP `memory_status` tool includes vector provider in response

## Phase 4: Validation

### 4.1 Integration testing
- [ ] Test sqlite-vec provider: search, upsert, delete (existing behavior)
- [ ] Test qdrant provider against local Docker instance
- [ ] Test host resolution: localhost → host.docker.internal in container
- [ ] Test config switching: sqlite-vec ↔ qdrant without data loss in SQLite
- [ ] Test migration: verify vector count matches between SQLite and Qdrant
- [ ] Test search quality: same query returns same top-K results from both providers

### 4.2 Documentation
- [ ] Update README with vector provider config section
- [ ] Document quick start: `npx nano-brain qdrant up && npx nano-brain qdrant migrate`
- [ ] Document fallback: `npx nano-brain qdrant down` auto-switches to sqlite-vec
