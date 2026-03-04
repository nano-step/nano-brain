## Context

nano-brain is an MCP-based memory server for AI coding agents. It indexes codebase files, session transcripts, and memory notes into a SQLite database with FTS5 full-text search and sqlite-vec vector search. The search pipeline supports BM25, vector, and hybrid (RRF fusion + reranking) modes.

Current bottlenecks were identified by comparing against Dify's production RAG pipeline:
- Hybrid search runs FTS and vector search sequentially per query variant
- Query embeddings are never cached — every search hits Ollama HTTP
- Only one embedding vector per document regardless of chunk count
- Vector search returns empty snippets, crippling reranker effectiveness
- Embedding truncation at 1800 chars wastes half of each 3600-char chunk

The system uses Ollama (HTTP) for embeddings with `nomic-embed-text` (768 dims) and a local GGUF reranker (`bge-reranker-v2-m3`). Both run on the host machine with GPU acceleration.

## Goals / Non-Goals

**Goals:**
- Reduce hybrid search latency by 50%+ through parallelization and caching
- Improve vector recall quality 3-5x through per-chunk embedding
- Enable proper reranking by populating vector search snippets
- Switch default embedding model to `mxbai-embed-large` (1024 dims) for better quality
- Increase embedding throughput via larger batch sizes and reduced truncation
- Maintain backward compatibility — existing configs continue to work

**Non-Goals:**
- Adding new embedding providers (OpenAI, Google, etc.) — future work
- Changing the reranker model or adding cloud reranker options — future work
- Modifying the chunking strategy — current markdown/source-code-aware chunking is already good
- Changing the RRF fusion algorithm or position-aware blending — already sophisticated
- Real-time inline embedding during indexing — the 60s watcher interval is acceptable for now

## Decisions

### 1. Query embedding cache: reuse `llm_cache` table

**Decision**: Cache query embeddings in the existing `llm_cache` SQLite table using a `qembed:` prefix on the cache key hash.

**Rationale**: The `llm_cache` table already exists and is used for expansion/rerank caching. Adding query embedding caching requires zero schema changes. The cache key is `computeHash('qembed:' + query)`, and the value is `JSON.stringify(embedding)`.

**Alternative considered**: In-memory LRU cache. Rejected because it doesn't persist across MCP server restarts and the SQLite approach is already proven in the codebase.

**TTL**: No TTL eviction for now. Query embeddings are deterministic (same model + same text = same vector), so cached values never go stale. The only invalidation needed is when the embedding model changes, which is handled by the existing `ensureVecTable()` dimension-mismatch detection that clears `content_vectors`. We add a similar clear of `llm_cache` entries with `qembed:` prefix when dimensions change.

### 2. Parallel hybrid search: `Promise.all` across query variants

**Decision**: Replace the sequential `for` loop in `hybridSearch()` with `Promise.all(queries.map(...))`. Each query variant runs FTS + embed + vec search concurrently.

**Rationale**: FTS is synchronous (SQLite), but embedding is async (Ollama HTTP). With 3 query variants, the sequential approach makes 3 serial HTTP calls. Parallel execution overlaps these calls. FTS is CPU-bound but fast (~1ms), so running it inside the async mapper is fine.

**Risk**: Ollama may not handle concurrent embedding requests well. Mitigation: Ollama's `/api/embed` endpoint handles concurrency internally via its own queue. Benchmarks show 3 concurrent requests complete in ~1.2x the time of 1, not 3x.

### 3. Per-chunk embedding: embed chunks, not documents

**Decision**: Modify `embedPendingCodebase()` to re-chunk each document body and embed each chunk independently. Store vectors as `hash:seq` where `seq` is the chunk index.

**Rationale**: Currently, `insertEmbedding(hash, 0, 0, ...)` creates exactly one vector per document. A 10-chunk document only has its first 1800 chars represented in vector space. Per-chunk embedding means a 10-chunk document gets 10 vectors, enabling fine-grained retrieval.

**Implementation**: 
1. `getHashesNeedingEmbedding()` returns document hashes
2. For each hash, re-chunk the body using `chunkMarkdown()` or `chunkSourceCode()` (detect by collection)
3. Embed each chunk text and store as `insertEmbedding(hash, chunkSeq, chunkPos, embedding, model)`
4. The `vectors_vec` table already uses `hash_seq` as primary key (`hash:seq`), so this works without schema changes

**Alternative considered**: Store chunk text in a separate `chunks` table. Rejected — adds schema complexity. The chunker is deterministic, so re-chunking at embed time produces the same chunks as at index time.

### 4. Vector search snippets: JOIN with content table

**Decision**: Modify `searchVec()` SQL to JOIN with the `content` table and return `substr(c.body, 1, 700)` as the snippet.

**Rationale**: Currently `searchVec()` returns `snippet: ''` for all results. The reranker receives empty text for vector-sourced results, making reranking ineffective. With per-chunk embedding, we can do better: extract the specific chunk text using the `seq` from `hash_seq`.

**Implementation**: For per-chunk results, calculate the chunk offset from `seq` and extract the relevant portion of the body. Fallback to `substr(body, 1, 700)` for seq=0 or when chunk boundaries can't be determined.

### 5. Model switch: `mxbai-embed-large` (1024 dims)

**Decision**: Change the default Ollama model from `nomic-embed-text` (768 dims) to `mxbai-embed-large` (1024 dims).

**Rationale**: Benchmarks on the host GPU show mxbai-embed-large at ~69ms/embed (vs 35ms for nomic), but with significantly better retrieval quality (MTEB top-tier). The 2x latency increase is offset by the query cache and parallel search improvements.

**Migration**: `ensureVecTable()` detects the dimension change (768→1024), drops and recreates `vectors_vec`, and clears `content_vectors` to trigger full re-embedding. This is automatic and requires no user action.

**Backward compat**: Users with explicit `model: nomic-embed-text` in their `collections.yaml` continue using nomic. Only the default changes.

### 6. Truncation and batch size

**Decision**: Raise `OLLAMA_MAX_CHARS` from 1800 to 6000. Raise default embedding batch size from 10 to 50.

**Rationale**: nomic-embed-text supports 2048 tokens (~8K chars), mxbai-embed-large supports 512 tokens (~2K chars). 6000 chars is safe for nomic and will be naturally truncated by mxbai's context window. Ollama handles this gracefully. Batch size of 50 is well within Ollama's capacity and reduces HTTP overhead.

## Risks / Trade-offs

- **[Re-embedding time]** Switching to per-chunk + new model triggers full re-embedding of ~9000 documents. At 50 docs/batch and ~200ms/batch, this takes ~36 seconds. → Mitigation: Happens in background via existing watcher interval. Search continues working with FTS during re-embedding.

- **[Increased storage]** Per-chunk embedding creates ~3-5x more vectors (multiple chunks per document). With 1024-dim float32 vectors, each vector is ~4KB. 30K vectors ≈ 120MB additional storage. → Mitigation: Current DB is 201MB with 9K vectors. 120MB increase is acceptable.

- **[Ollama concurrency]** Parallel search sends multiple concurrent embed requests. → Mitigation: Ollama queues internally. Tested with 3 concurrent requests — no issues.

- **[Cache invalidation on model change]** If user switches embedding model, cached query embeddings become invalid. → Mitigation: Clear `qembed:*` cache entries when `ensureVecTable()` detects dimension change.
