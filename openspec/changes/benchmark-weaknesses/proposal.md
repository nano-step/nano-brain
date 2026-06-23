## Why

Benchmark against LlamaIndex, Qdrant revealed 3 weaknesses in nano-brain's search and code intelligence:

1. **BM25 AND kills recall** — Natural language queries like "Explain the order lifecycle" use `websearch_to_tsquery` which ANDs all terms. When any word doesn't exist in the index, BM25 returns 0 results. 6 of 20 express-app queries fail this way.

2. **Incoming edges broken** — `memory_graph(direction=in)` uses exact match on `target_node` but call edges store bare names (`BM25Search`) while queries use file-qualified names (`/path/file.go::BM25Search`). 0% callers accuracy.

3. **Graph extraction skipped for existing files** — The watcher's `processFile` returns early when content hash matches DB. Functions like `HybridSearch` were indexed before graph extraction was added → never got edges.

## What Changes

- **Fix 1**: Change BM25 to OR semantics (or add AND→OR fallback)
- **Fix 2**: Add symbol-only fallback in `GetIncomingEdges`
- **Fix 3**: Run graph extraction during `processAll` even for unchanged files

## Impact

- **Search quality**: +30% P@5 on express-app (from 0.53 → ~0.70)
- **Code intel**: Enables `memory_impact` tool (callers accuracy 0% → target >50%)
- **Code intel**: Enables trace for interface dispatch functions
- **Files**: `internal/search/service.go`, `internal/storage/queries/graph.sql`, `internal/watcher/watcher.go`
