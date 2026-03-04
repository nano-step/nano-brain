## Why

nano-brain's search pipeline has critical performance and quality bottlenecks identified through analysis against Dify's production RAG system (131K⭐). Query latency is unnecessarily high due to sequential search operations and missing caches. Vector recall quality is severely limited because only one embedding is created per document (ignoring all chunks beyond the first 1800 chars). These issues compound: slow queries return poor results.

## What Changes

- **Query embedding cache**: Cache query embeddings in the existing `llm_cache` table to eliminate repeated Ollama HTTP calls for identical queries
- **Parallel hybrid search**: Run FTS and vector search concurrently with `Promise.all` instead of sequential loops, cutting hybrid search latency ~50%
- **Per-chunk embedding**: Embed each chunk independently instead of one embedding per document, dramatically improving vector recall for large files
- **Vector search snippets**: Populate snippet text in vector search results by JOINing with the content table, enabling proper reranking
- **Raise embedding truncation limit**: Increase `OLLAMA_MAX_CHARS` from 1800 to 6000 to capture more content per embedding (nomic-embed-text supports 8192 tokens)
- **Larger embedding batch size**: Increase batch size from 10 to 50 for faster indexing throughput
- **Switch default model to mxbai-embed-large**: Higher quality embeddings (1024 dims vs 768) with GPU-accelerated performance
- **BREAKING**: Changing embedding model/dimensions requires full re-embedding of all indexed documents. The `ensureVecTable()` mechanism handles this automatically by detecting dimension mismatch and clearing `content_vectors`.

## Capabilities

### New Capabilities
- `query-embedding-cache`: Cache query embeddings in llm_cache table with TTL-based eviction to eliminate redundant Ollama HTTP calls
- `parallel-hybrid-search`: Run FTS and vector search concurrently across all query variants using Promise.all
- `per-chunk-embedding`: Embed each document chunk independently instead of one embedding per whole document

### Modified Capabilities
- `search-pipeline`: Vector search now returns populated snippets; hybrid search runs in parallel; query embeddings are cached
- `mcp-server`: Default embedding model changes to mxbai-embed-large (1024 dims); truncation limit raised; batch size increased

## Impact

- **Files**: `search.ts`, `store.ts`, `embeddings.ts`, `codebase.ts`, `watcher.ts`, `server.ts`, `types.ts`
- **Database**: `vectors_vec` table will be rebuilt with new dimensions (1024 vs 768). `content_vectors` cleared to trigger re-embedding. `llm_cache` table reused for query embedding cache.
- **Config**: `collections.yaml` embedding model default changes. Existing configs with explicit `model: nomic-embed-text` continue to work.
- **Dependencies**: No new dependencies. `mxbai-embed-large` must be pulled into Ollama on the host.
- **Migration**: Automatic — `ensureVecTable()` detects dimension mismatch and rebuilds. Re-embedding happens in background via existing watcher interval.
