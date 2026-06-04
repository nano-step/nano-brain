# Proposal: Symbol-Aware Chunking with LLM Summaries at Index Time

Tracking: #370
Lane: high-risk
Change type: user-feature
Date: 2026-06-04

## Problem

nano-brain currently chunks source files by fixed size (~500 chars) or heading boundaries. This means:

1. **Functions get split mid-body** — a 60-line function may span 3 chunks, none of which is self-contained.
2. **Agents get raw code, not meaning** — `memory_search("what does ExtractEdges do?")` returns a raw snippet. The agent must read 237 lines of source to understand intent.
3. **Embedding quality is poor for code** — embedding models train on natural language; raw Go/Python/TypeScript syntax produces dense but semantically opaque vectors.
4. **Token waste** — retrieval returns 500+ token code chunks; LLM-generated summaries (~100 tokens) carry the same semantic signal with 5x fewer tokens.

## Proposed Solution

Two complementary changes, independently useful but designed to compose:

### Part A — Symbol-Aware Chunking

When indexing `.go`, `.ts`, `.js`, `.py` files, use the existing Tree-sitter extractors (`internal/graph/`) to split content at **function/method/class boundaries** instead of fixed character count.

Each chunk = one complete logical unit (function, method, type declaration). No more mid-function splits.

### Part B — LLM Summary at Index Time (optional, async)

After symbol-aware chunking, optionally generate a 2-4 sentence natural-language summary per function chunk using the existing summarization LLM pipeline (`summarization` config). Store the summary alongside the chunk. Embed the summary (not raw code) for vector search.

Cache by `content_hash` — if the function body hasn't changed, reuse existing summary.

## Scope

**In scope:**
- New `internal/chunker/` package with `SymbolAwareChunker` implementing existing `Chunker` interface
- Hybrid dispatch: symbol-aware for supported languages, existing chunker for everything else
- New `symbol_chunks` table (or extend `chunks` with `symbol_name`, `symbol_kind`, `summary` columns)
- Async summary generation pipeline triggered after symbol chunking
- Config flag to opt in/out of LLM summaries (`indexing.symbol_summaries: true/false`)
- MCP `memory_symbols` tool returns summary field when available
- Reindex support: existing files get re-chunked on next `POST /api/v1/reindex`

**Out of scope:**
- Changing the BM25 or vector search pipeline internals
- Changing existing heading-aware chunker for markdown/text files
- Cross-function call graph enrichment (already handled by `internal/graph/`)
- Real-time (query-time) summary generation

## Acceptance Criteria

1. Indexing a `.go` file produces one chunk per top-level function/method/type — no function is split across multiple chunks.
2. `memory_search("ExtractEdges")` returns the full `ExtractEdges` function body as a single chunk with `symbol_name: "ExtractEdges"`, `symbol_kind: "function"`.
3. When `indexing.symbol_summaries: true`, indexed functions have a non-empty `summary` field within 30s of indexing.
4. Summary is reused (not re-generated) when file content hash is unchanged on reindex.
5. Files without language support (`.md`, `.yaml`, `.json`) continue to use existing chunker — no regression.
6. `go test -race -short ./...` passes. Integration tests pass.
7. Smoke E2E: server starts, file indexed, `memory_search` returns symbol chunk with metadata.

## Risks

| Risk | Severity | Mitigation |
|------|----------|------------|
| Schema migration breaks existing chunks | High | Additive migration only; all new columns nullable/defaulted; backfill `embedding_strategy='raw_code'` for all existing rows |
| Symbol chunker produces too-large chunks (huge functions) | Medium | Skip symbols >8KB in v1; fall back to fixed-size chunker for that symbol |
| LLM summary cost at scale (large monorepos) | Medium | v2 only; cache by content_hash; config flag off by default; async pipeline |
| Embedding drift (summary vs raw code vectors mixed in same index) | High | Workspace-level `embedding_strategy` toggle; v1 defaults to `raw_code` only |
| Tree-sitter grammar gaps (unsupported syntax) | Low | Parse failure or 0 symbols → fall back to fixed-size chunker; log WARN; never block indexing |
| Reindex transition mixed state | Medium | Per-file gradual reindex; edges to deleted chunks auto-pruned on next graph rebuild |
| Chunk ID stability under line shifts | Low | Content-addressed IDs; `symbol_name+source_path` as stable lookup key independent of line numbers |

## Deep Design Summary

Multi-agent analysis (Metis + Oracle + cross-critique + Momus) produced the following key resolutions:

- **v1 ships symbol chunking only** — LLM summaries are v2 after chunk quality validated
- **Byte ranges extracted via Tree-sitter native API** (`node.StartByte()/EndByte()`) inside SymbolAwareChunker — `graph.Edge` struct NOT extended
- **8KB symbols skipped in v1** — sub-chunking deferred to v2
- **Reindex is per-file gradual** — no shadow table, no atomic swap; mixed state acceptable during transition
- **BM25 indexes both raw code + summary** — tsvector updated on summary arrival (v2)
