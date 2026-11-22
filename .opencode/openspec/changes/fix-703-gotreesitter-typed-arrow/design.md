# Design

## Root Cause & Upgrade Choice

In `gotreesitter` v0.19.1, the pure-Go GLR/lexer handling for TypeScript
and TSX fails to disambiguate a parenthesized parameter list whose first
parameter has a `: Type` annotation when `async` is absent, collapsing the
enclosing syntax into an `ERROR` root. `gotreesitter` v0.57.0 resolves the
TypeScript/TSX typed-arrow disambiguation and substantially reduces
per-parse arena pressure on large JavaScript files so 5000+ line `.js`
files complete with `root=program`, `hasErr=false`, and
`ParseStoppedEarly()=false`.

`v0.57.0` (published 2026-11-08) is selected over `v0.58.0` to satisfy the
minimum 7-day release age supply-chain requirement while staying on a pure
Go (`CGO_ENABLED=0`) build.

## Existing Workspace Recovery

Unchanged `.ts`, `.tsx`, `.vue`, and `.js` files in already-indexed
workspaces match their stored `mod_time`/`file_size` and `content_hash`, so
normal watcher startup and incremental scans skip them before parsing.
Unlike #584 (where symbol rows existed with missing `end_line` metadata),
symbols and edges dropped by an `ERROR` tree left no database marker that
distinguishes an affected file from a genuinely symbol-free file without
re-parsing every JS/TS/Vue source file.

- **Symbols, graph edges, and CFGs (`POST /api/v1/update`):** `TriggerUpdate`
  runs `Watcher.ReextractEdgesForWorkspace` (replacing `graph_edges` and,
  when `flow.enabled` is true, `function_flowcharts`) and
  `Watcher.ReextractSymbolsForWorkspace` (re-running `extractAndUpsertSymbols`
  for every admitted file), bypassing the `content_hash` early exit without
  wiping or re-embedding existing parent-file chunks.
- **Parent-file symbol chunks:** `SymbolAwareChunker` chunks stored on an
  unchanged parent `documents` row (`chunk_type = 'raw'` fallback when zero
  symbols parsed) are refreshed automatically when the file next changes on
  disk, or across a workspace via `POST /api/v1/reset-workspace` followed
  by `POST /api/v1/reindex` (`embedding_vectors` deduplicated by content
  blob reuse cached vectors for unchanged chunk text). Note that
  `POST /api/v1/reindex` with `force_wipe: true` resets chunk embed status
  and re-extracts symbol docs, while unchanged parent file documents still
  hit the `content_hash` skip in `processFile`.

## Non-Goals

- Automatic startup re-parse of every indexed `.ts`/`.tsx`/`.vue`/`.js`
  file on upgrade: doing so would penalize cold start on large workspaces
  when `POST /api/v1/update` already provides explicit, non-destructive
  backfill.
