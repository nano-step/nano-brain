## Why

nano-brain's search quality and embedding pipeline have several gaps discovered during real-world usage: embedding truncation loses context from the second half of chunks, the chunking strategy doesn't optimize for embedding quality, there's no way to measure or benchmark search relevance, and the embedding pipeline processes documents sequentially with no batching optimization. These issues compound — poor embeddings lead to poor vector search, which degrades hybrid search, which makes the entire memory system less useful for AI agents.

## What Changes

- **Smarter embedding truncation**: Replace the hard `substring(0, 1800)` cut with word/sentence-boundary-aware truncation that preserves semantic completeness
- **Chunk size alignment**: Align chunk size with embedding model's effective context window so chunks don't need truncation at all — currently chunks are 3600 chars but only the first 1800 are embedded, wasting half the stored content for vector search
- **Embedding batch pipeline**: Implement proper batch embedding with configurable concurrency, progress tracking, and resume-on-failure — currently processes one doc at a time with no parallelism
- **Search result scoring transparency**: Add `--explain` flag to CLI that shows which search mode (FTS/vector/hybrid) contributed to each result's score, enabling users to diagnose search quality issues
- **Cross-workspace search**: Allow querying across all workspace DBs from any workspace, with results tagged by project — currently each workspace is fully isolated
- **Embedding model warm-up**: Pre-warm the Ollama embedding model on MCP server start to avoid cold-start latency on first query (currently 1.4s first request vs 150ms warm)
- **Incremental session indexing**: Index new sessions into the DB immediately after harvest instead of requiring a separate `embed` step — currently harvest writes markdown files but doesn't trigger indexing or embedding

## Capabilities

### New Capabilities
- `embedding-pipeline`: Batch embedding with concurrency, progress tracking, resume-on-failure, and model warm-up
- `search-explain`: Transparent scoring with `--explain` flag showing per-result breakdown of FTS score, vector similarity, and RRF fusion contribution
- `cross-workspace-search`: Query across all workspace DBs with project-tagged results

### Modified Capabilities
- `search-pipeline`: Chunk size alignment with embedding window, word-boundary-aware truncation, and scoring transparency
- `storage-limits`: Incremental session indexing after harvest (currently harvest and indexing are decoupled)

## Impact

- **`src/chunker.ts`**: Adjust `maxChunkSize` default and add overlap tuning to align with embedding window
- **`src/codebase.ts`**: Replace `truncateForEmbedding()` with sentence-boundary-aware truncation, add batch concurrency to `embedPendingCodebase()`
- **`src/search.ts`**: Add explain/debug metadata to search results, implement cross-workspace query routing
- **`src/index.ts`**: Add `--explain` CLI flag, add cross-workspace `--all` flag, wire incremental indexing after harvest
- **`src/server.ts`**: Add model warm-up on startup, expose explain metadata in MCP tool responses
- **`src/harvester.ts`**: Trigger document indexing + embedding after successful harvest
- **`src/embeddings.ts`**: Add batch concurrency control, warm-up method, progress callbacks
- **`src/store.ts`**: Support querying multiple DB files for cross-workspace search
- **Config**: New `embedding.batchSize`, `embedding.concurrency`, `embedding.warmup` options in `config.yml`
- **Dependencies**: No new dependencies expected — all changes use existing SQLite, Ollama, and fast-glob
