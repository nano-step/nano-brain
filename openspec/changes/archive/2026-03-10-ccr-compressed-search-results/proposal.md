## Why

nano-brain's search tools (`memory_query`, `memory_search`, `memory_vsearch`) return full 700-char snippets for every result. A typical 10-result query consumes ~7000 chars of context window — most of which the agent never reads. Inspired by Headroom's CCR (Compress-Cache-Retrieve) pattern, we add an opt-in compact mode that returns summaries and lets agents expand individual results on demand. This could save 60-80% tokens on search interactions while preserving full access to the data.

## What Changes

- Add a **compact response mode** to all three search tools — returns title, path, score, and a 1-line summary (~80 chars) instead of full 700-char snippets
- Add a **`memory_expand`** MCP tool that retrieves the full content for a specific result by its docid or hash
- Add a **server-side result cache** that stores full search results for a configurable TTL (default 15 minutes) so `memory_expand` can retrieve them without re-running the search
- Compact mode is **opt-in** via `compact: true` parameter (MCP) or `--compact` flag (CLI) — verbose remains the default for backward compatibility
- Add a **`compact`** flag to CLI search commands (`query`, `search`, `vsearch`) for parity

## Capabilities

### New Capabilities
- `compact-search-results`: Compressed search result format with 1-line summaries, server-side caching, and on-demand expansion via `memory_expand` tool

### Modified Capabilities
- `search-pipeline`: Add compact output mode and result caching to existing search pipeline
- `mcp-server`: Register new `memory_expand` tool, add `compact` parameter to existing search tools

## Impact

- **src/server.ts** — New `memory_expand` tool registration, `compact` parameter on search tools, compact formatting
- **src/cache.ts** — New file: result cache with TTL
- **src/index.ts** — CLI `--compact` flag for search commands
- **test/** — New tests for compact mode, expansion, caching
- **No breaking changes** — `compact` defaults to false (verbose), opt-in only. Existing `--json` CLI output unchanged
- **No new dependencies** — Uses in-memory Map with TTL for cache
