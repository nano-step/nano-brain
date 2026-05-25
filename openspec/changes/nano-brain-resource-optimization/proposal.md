## Why

nano-brain runs as an MCP server inside the ai-sandbox-wrapper Docker container — the same container that hosts OpenCode (the AI coding agent), 4+ Pyright LSP servers, Playwright MCP, GraphQL inspector, database inspector, and sequential-thinking MCP. Current container profile: ~20GB RAM total, 8 vCPUs, but nano-brain competes with all co-tenants for resources. Observed state: 17.3GB of 20GB used, only 2.7GB available, 2.9GB swap active — the system is under severe memory pressure.

nano-brain's ~650MB baseline RAM (reranker model alone is ~500MB, loaded eagerly regardless of usage) is a significant contributor. When nano-brain indexes a codebase, its synchronous Tree-sitter parsing and 30+ `readFileSync` calls on overlay filesystem (2-5x slower than native) block the Node.js event loop for 25-85 seconds, consuming CPU quota that should serve MCP tool requests from the AI agent. The OpenCode process itself uses ~950MB RSS — nano-brain's resource consumption directly degrades the agent's responsiveness.

This is not a theoretical concern: the container is already swapping 2.9GB to disk, which means every memory allocation triggers page faults that slow all processes.

## What Changes

- **Lazy model loading**: Reranker model loaded on first search request (if reranking enabled) instead of eagerly at startup. Saves ~500MB when reranking is unused.
- **Model disposal**: `dispose()` methods call actual native `model.dispose()` on node-llama-cpp. Cleanup handler in server.ts disposes all models on shutdown.
- **Tree-sitter parser pooling**: Reuse Parser instances across files instead of `new Parser()` per file. Pool size bounded by language count.
- **Hash memoization**: Workspace root hash computed once and cached. Eliminates 10+ redundant SHA-256 computations per command.
- **SQLite pragma tuning**: Add `cache_size`, `mmap_size`, and `temp_store` pragmas for faster query performance.
- **Query embedding cache eviction**: Cap `llm_cache` entries of type `qembed` with LRU eviction (max 500 entries).
- **Inference thread limiting**: Expose `nThreads` config for node-llama-cpp to control CPU core usage. Critical in containers where cgroup CPU quota is shared.
- **Single-context mode**: Config option to use 1 inference context per model instead of up to 4, trading throughput for ~60% RAM reduction per model. Default to 1 when container detected.
- **Event loop yield points**: Insert `setImmediate()` between file processing iterations in codebase indexing and symbol extraction loops.
- **Container-aware defaults**: When `isInsideContainer()` returns true (already detected via `host.ts`), automatically apply conservative defaults: single-context mode, lazy model loading, reduced SQLite cache, lower thread count. No config required — just works in Docker.

## Capabilities

### New Capabilities
- `lazy-model-loading`: Defer model initialization to first use with configurable eager/lazy mode per model
- `resource-limits`: Configurable inference thread count, context pool size, cache bounds, and SQLite tuning
- `parser-pooling`: Reusable Tree-sitter parser instances with bounded pool per language
- `container-aware-defaults`: Auto-detect Docker/container environment and apply conservative resource defaults

### Modified Capabilities
- `mcp-server`: Server cleanup handler disposes models; model status reflects lazy loading state (not-loaded → loading → loaded)
- `storage-limits`: Query embedding cache gains LRU eviction bound; SQLite pragma tuning affects storage performance characteristics

## Impact

- **Memory**: Baseline RAM reduced from ~650MB to ~150MB when reranker is lazy-loaded and single-context mode is used. Frees ~500MB for OpenCode and other co-tenant MCP servers in the shared container. Reduces swap pressure (currently 2.9GB swapped).
- **CPU**: Event loop blocking reduced by 70-80% during indexing via yield points and parser pooling. MCP tool requests from the AI agent remain responsive during background indexing — critical since nano-brain shares the container's 8 vCPUs with OpenCode, 4 Pyright instances, and other MCP servers.
- **Container co-tenancy**: Auto-detect Docker environment via existing `isInsideContainer()` in `host.ts` and apply conservative defaults. nano-brain becomes a good neighbor in the shared ai-sandbox-wrapper container.
- **Overlay FS**: Async file I/O mitigates the 2-5x latency penalty of Docker overlay filesystem on `readFileSync` calls.
- **Files changed**: `server.ts` (model lifecycle, cleanup), `store.ts` (pragmas, cache eviction), `embeddings.ts` (dispose, thread config, context pool), `reranker.ts` (lazy loading, dispose), `treesitter.ts` (parser pool), `codebase.ts` (yield points), `symbols.ts` (yield points), `types.ts` (config interfaces), `search.ts` (lazy provider resolution), `host.ts` (resource limit detection)
- **Config**: New `resources` section in `config.yml` with `threads`, `contextPoolSize`, `lazyModels`, `cacheMaxEntries` fields. All optional — container-aware defaults kick in automatically.
- **No breaking changes**: All optimizations are backward compatible. Existing non-container deployments keep current behavior unless explicitly configured.
- **No new dependencies**: All changes use existing node-llama-cpp APIs, Node.js built-ins, and the existing `isInsideContainer()` detection.
