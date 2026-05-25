## 1. Query Embedding Cache

- [x] 1.1 Add `getQueryEmbeddingCache(query: string)` and `setQueryEmbeddingCache(query: string, embedding: number[])` methods to `Store` class in `store.ts` — use `computeHash('qembed:' + query)` as key, `JSON.stringify(embedding)` as value, store in existing `llm_cache` table
- [x] 1.2 Add `clearQueryEmbeddingCache()` method to `Store` class — deletes all `llm_cache` rows where the hash was generated with `qembed:` prefix (store the prefix pattern or clear all llm_cache on dimension change)
- [x] 1.3 Wrap `embedder.embed(query)` calls in `search.ts` `hybridSearch()` to check cache first, store on miss — replace direct `embedder.embed(q)` with cache-aware helper
- [x] 1.4 Wrap `embedder.embed(query)` call in `server.ts` `memory_vsearch` handler to check cache first, store on miss
- [x] 1.5 Call `clearQueryEmbeddingCache()` inside `ensureVecTable()` in `store.ts` when dimension mismatch is detected (alongside existing `content_vectors` clear)

## 2. Parallel Hybrid Search

- [x] 2.1 Refactor `hybridSearch()` in `search.ts` — replace sequential `for` loop over query variants with `Promise.all(queries.map(...))` where each mapper runs FTS + embed + vec search concurrently
- [x] 2.2 Add error handling per variant — wrap each variant's search in try/catch so one failure returns empty results without blocking others
- [x] 2.3 Preserve existing weight logic — original query (index 0) gets weight 2, expanded variants get weight 1, applied during RRF aggregation after parallel completion

## 3. Per-Chunk Embedding

- [x] 3.1 Modify `embedPendingCodebase()` in `codebase.ts` — for each document hash, re-chunk the body using `chunkMarkdown()` or `chunkSourceCode()` (detect by file extension or collection type), then embed each chunk independently
- [x] 3.2 Store per-chunk embeddings as `insertEmbedding(hash, chunkSeq, chunkPos, embedding, model)` where `chunkSeq` is the zero-indexed chunk number — the existing `hash_seq` format (`hash:seq`) already supports this
- [x] 3.3 Before inserting new chunk embeddings for a document, delete all existing embeddings for that hash (all `hash:*` rows in `vectors_vec`) to handle chunk count changes
- [x] 3.4 Update `getHashesNeedingEmbedding()` in `store.ts` if needed — ensure it returns hashes for documents that have content but no embeddings (or stale embeddings after model change)

## 4. Vector Search Snippets

- [x] 4.1 Modify `searchVec()` SQL in `store.ts` — JOIN with `content` table on hash, return `substr(c.body, 1, 700)` as snippet instead of empty string
- [x] 4.2 For per-chunk results, calculate chunk offset from `seq` in `hash_seq` and extract the relevant portion: `substr(c.body, seq * chunk_size + 1, 700)` — fallback to `substr(c.body, 1, 700)` for seq=0

## 5. Embedding Configuration Changes

- [x] 5.1 Change `OLLAMA_MAX_CHARS` from `1800` to `6000` in `embeddings.ts` (line ~114)
- [x] 5.2 Change `MAX_EMBED_CHARS` from `1800` to `6000` in `codebase.ts` (line ~331)
- [x] 5.3 Change default embedding model from `nomic-embed-text` to `mxbai-embed-large` in `embeddings.ts` (line ~284, the OllamaEmbedder default)
- [x] 5.4 Change default batch size from `10` to `50` in `codebase.ts` `embedPendingCodebase()` parameter default (line ~342)
- [x] 5.5 Change batch size from `10` to `50` in `watcher.ts` `embedPendingCodebase()` call (line ~294)

## 6. Verification

- [x] 6.1 Run `npx tsc --noEmit` from nano-brain root to verify no type errors
- [ ] 6.2 Run the MCP server manually to verify it starts without errors
- [ ] 6.3 Test `memory_query` tool to verify hybrid search returns results with populated snippets
- [ ] 6.4 Verify `ensureVecTable()` detects dimension mismatch (768→1024) and triggers rebuild + cache clear
