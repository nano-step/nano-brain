## Why

The `query` command (hybrid search) fails to find code identifiers like `instantSellPriceAdjustPercent` despite the data being indexed. Two bugs compound: (1) the MCP server never passes the `db` parameter to `indexCodebase()`, so the tree-sitter symbol graph (`code_symbols`) is never populated — 0 rows in production; (2) `sanitizeFTS5Query` wraps multi-word queries as a single exact phrase, causing FTS to return 0 results for natural-language queries that contain code identifiers. The `search` command (BM25-only) works fine for single-term exact matches, but `query` — the primary user-facing command — is broken for code search.

## What Changes

- **Fix symbol graph indexing**: Pass the `db` instance from the MCP server's `memory_index_codebase` handler to `indexCodebase()`, so the `if (db && isTreeSitterAvailable())` guard passes and tree-sitter symbol extraction actually runs.
- **Add symbol search as a third search lane in hybrid search**: Query `code_symbols` by name during `hybridSearch()`, return matching documents as a result set, and include it in RRF fusion alongside FTS and vector results.
- **Fix FTS query parsing for multi-word queries**: Change `sanitizeFTS5Query` to split multi-word input into individual quoted terms joined by OR, instead of wrapping the entire input as one exact phrase.
- **Add camelCase/snake_case splitting for symbol name matching**: When searching symbols, split both the query and symbol names into sub-tokens (e.g., `instantSellPriceAdjustPercent` → `instant sell price adjust percent`) so partial matches work.

## Capabilities

### New Capabilities
- `symbol-search-lane`: Symbol name matching as a first-class search source in hybrid search. Queries the `code_symbols` table with camelCase-aware splitting, returns results as an RRF-fusible result set.

### Modified Capabilities
- `search-pipeline`: FTS query sanitization changes from single-phrase wrapping to per-term OR joining for multi-word queries. Symbol search lane added to hybrid search pipeline. Existing single-term exact-match behavior preserved.

## Impact

- **`src/server.ts`**: Pass `db` parameter to `indexCodebase()` call in `memory_index_codebase` handler.
- **`src/store.ts`**: Modify `sanitizeFTS5Query()` to handle multi-word queries with OR logic instead of single-phrase wrapping.
- **`src/search.ts`**: Add symbol search function, integrate as third result set in `hybridSearch()` RRF fusion.
- **`src/symbol-graph.ts`**: Add `searchByName()` method with camelCase splitting support.
- **`src/codebase.ts`**: No changes needed (already has symbol graph code, just never called).
- **Existing tests**: `sanitizeFTS5Query` behavior changes — existing tests for single-term queries should still pass, multi-word query tests need updating.
- **Re-indexing required**: Users must run `memory_index_codebase` again after upgrade to populate the symbol graph.
