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

## Bounded & Concurrency-Safe Vue SFC Parsing

Every single-language extractor in `internal/symbol`, `internal/graph`, and
`internal/chunker` constructs a fresh per-call `*gotreesitter.Parser` via
`tsparse.NewParser`, whereas `symbol.VueExtractor` and `graph.VueSFCExtractor`
previously stored a single shared `*gotreesitter.InjectionParser` on the
extractor struct with no timeout configured. `gotreesitter.InjectionParser` is
explicitly not concurrency-safe — `Parse` mutates `ip.parsers` and calls
`releaseResult(ip.prevResult)`, freeing the previous caller's parse trees — so
concurrent `.vue` extraction (including concurrent `POST /api/v1/update`
invocations) triggered deterministic `DATA RACE` and `SIGSEGV` crashes.

- `tsparse.NewInjectionParser()` wraps `gotreesitter.NewInjectionParser()` and
  applies `SetTimeoutMicros(uint64(DefaultTimeout / time.Microsecond))`
  (newly exposed in `gotreesitter` v0.57.0) so both root and nested injected
  parses respect `DefaultTimeout`.
- `symbol.VueExtractor` and `graph.VueSFCExtractor` pre-compile their
  immutable `*gotreesitter.Language` and `*gotreesitter.Query` values at
  construction time, validate `newInjectionParser()` once in the constructor,
  and create a fresh per-call `InjectionParser` in `Extract`, `ExtractEdges`,
  and `ExtractEdgesWithImportContext`, releasing both `result.Tree` and all
  injected child trees in `defer`. Per-call construction is lock-free and
  benchmarked faster than reusing `ip.prevResult` across distinct files.

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
  for every admitted file), safely backfilling symbol documents
  (`?symbol=...`), `graph_edges`, and `function_flowcharts` on unchanged files
  without touching `memory`/`sessions` documents or re-embedding.
- **Parent-file symbol chunks:** `SymbolAwareChunker` chunks stored on an
  unchanged parent `documents` row (`ChunkTypeSymbol` vs `raw` fallback when
  zero symbols parsed) refresh naturally as files are edited on disk. Do NOT
  recommend `POST /api/v1/reset-workspace` + `POST /api/v1/reindex` as a
  recovery recipe: `ResetWorkspace` deletes all workspace documents
  (including `memory` and `sessions` collections), and neither
  `ResetWorkspace` nor `TriggerRescanByName` clears `Watcher.fileCache` on a
  live server, leaving unchanged files un-reindexed until restart or edit.

## Non-Goals

- Automatic startup re-parse of every indexed `.ts`/`.tsx`/`.vue`/`.js`
  file on upgrade: doing so would penalize cold start on large workspaces
  when `POST /api/v1/update` already provides explicit, non-destructive
  backfill.
