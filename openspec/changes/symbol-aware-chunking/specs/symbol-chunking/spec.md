# Spec: Symbol-Aware Chunking

Tracking: #370
Date: 2026-06-04

## ADDED Requirements

### Requirement: Dispatcher routes files to correct chunker by extension

The indexer SHALL use a Dispatcher that routes source files to the appropriate chunker based on file extension.

#### Scenario: Go file routed to SymbolAwareChunker

GIVEN a file with extension `.go`
WHEN the Dispatcher processes it
THEN the SymbolAwareChunker MUST be invoked

#### Scenario: Markdown file routed to HeadingChunker

GIVEN a file with extension `.md` or `.mdx`
WHEN the Dispatcher processes it
THEN the HeadingChunker MUST be invoked

#### Scenario: Unsupported extension routed to FixedChunker

GIVEN a file with extension `.yaml`, `.json`, or any unsupported extension
WHEN the Dispatcher processes it
THEN the FixedSizeChunker MUST be invoked

---

### Requirement: SymbolAwareChunker produces one chunk per top-level symbol

For supported languages (Go, TypeScript, JavaScript, Python), each top-level function, method, type declaration, and const/var block MUST produce exactly one chunk with chunk_type = 'symbol'.

#### Scenario: Go file with multiple functions

GIVEN a Go source file containing 5 top-level functions
WHEN the file is indexed via SymbolAwareChunker
THEN exactly 5 chunks MUST be produced
AND each chunk MUST have chunk_type = 'symbol'
AND each chunk MUST have symbol_name, symbol_kind, language, line_start, line_end populated
AND no function body SHALL span more than one chunk

#### Scenario: Symbol metadata is correct

GIVEN a Go source file containing `func ExtractEdges(...)`
WHEN the file is indexed
THEN the resulting chunk MUST have symbol_name = "ExtractEdges", symbol_kind = "function", language = "go"
AND line_start and line_end MUST match the actual function boundaries

#### Scenario: Nested closure stays with parent

GIVEN a Go function containing an inner closure
WHEN the file is indexed
THEN the outer function MUST produce exactly one chunk
AND the closure body MUST be part of the parent chunk content

---

### Requirement: Graceful fallback on parse failure or empty result

When Tree-sitter fails to parse a file or returns 0 symbols, the indexer MUST fall back to the FixedSizeChunker without blocking.

#### Scenario: Tree-sitter parse failure

GIVEN a file with extension `.go` that contains invalid syntax
WHEN the SymbolAwareChunker attempts to parse it
THEN the FixedSizeChunker MUST be used as fallback
AND a WARN log MUST be emitted with the file path and reason
AND the indexing operation MUST succeed (no error returned)

#### Scenario: File with no extractable symbols

GIVEN a Go file containing only comments and blank lines
WHEN the SymbolAwareChunker processes it
THEN the FixedSizeChunker MUST be used as fallback
AND indexing MUST succeed

---

### Requirement: Large symbols (>8KB) fall back to fixed-size chunker

Symbols exceeding 8192 bytes MUST NOT be indexed as a single chunk.

#### Scenario: Symbol body exceeds 8KB

GIVEN a Go function whose body is 10000 bytes
WHEN the SymbolAwareChunker processes it
THEN the FixedSizeChunker MUST be used for that symbol's byte range
AND a WARN log SHOULD be emitted with the symbol name and size
AND other symbols in the same file MUST still be chunked normally

---

### Requirement: Atomic single-pass Tree-sitter parsing

The file MUST be parsed exactly once per indexing operation.

#### Scenario: Single parse pass

GIVEN any supported source file
WHEN SymbolAwareChunker processes it
THEN Tree-sitter MUST parse the file exactly once
AND the same parsed tree MUST be used for both symbol name extraction and byte range extraction

---

### Requirement: Schema migration adds symbol metadata columns to chunks table

The chunks table MUST gain new columns for symbol metadata with an explicit backfill of existing rows.

#### Scenario: Migration adds columns

GIVEN a PostgreSQL database with the existing chunks table
WHEN the migration runs
THEN the table MUST have columns: symbol_name TEXT, symbol_kind TEXT, language TEXT, line_start INTEGER, line_end INTEGER, chunk_type TEXT NOT NULL DEFAULT 'raw', embedding_strategy TEXT NOT NULL DEFAULT 'raw_code'
AND all existing rows MUST be backfilled with chunk_type = 'raw' and embedding_strategy = 'raw_code'

#### Scenario: Migration is idempotent

GIVEN a database where the migration has already run
WHEN the migration runs again
THEN no error MUST be returned
AND no data MUST be lost or duplicated

#### Scenario: Indexes created

GIVEN the migration has run
THEN index idx_chunks_symbol_name MUST exist (partial, WHERE symbol_name IS NOT NULL)
AND index idx_chunks_chunk_type MUST exist

---

### Requirement: Reindex replaces fixed-size chunks with symbol chunks per file

When a workspace is reindexed, old fixed-size chunks for each file MUST be replaced with new symbol chunks.

#### Scenario: Manual reindex replaces chunks

GIVEN a workspace with existing chunk_type='raw' chunks for auth.go
WHEN POST /api/v1/reindex is called
THEN new symbol chunks for auth.go MUST be inserted
AND old fixed-size chunks for auth.go MUST be deleted after successful insertion

#### Scenario: Startup WARN for stale workspaces

GIVEN a workspace with chunk_type='raw' chunks
AND indexing.chunking_strategy = 'symbol_aware' is configured
WHEN the server starts
THEN a WARN log MUST be emitted indicating the workspace has stale chunks

---

### Requirement: memory_symbols MCP tool returns summary field

The memory_symbols MCP tool response MUST include a summary field (null in v1).

#### Scenario: memory_symbols returns null summary in v1

GIVEN a workspace with indexed symbol chunks
WHEN memory_symbols is called
THEN each result MUST include "summary": null
AND existing fields (name, kind, language, source_path, line_start, line_end) MUST be unchanged

#### Scenario: Backward compatibility preserved

GIVEN an agent that parses memory_symbols response without expecting summary
WHEN memory_symbols is called
THEN the agent MUST not break (unknown fields ignored per JSON spec)

---

### Requirement: memory_search accepts optional chunk_type filter

The search MCP tools MUST accept an optional chunk_type parameter without changing default behavior.

#### Scenario: Default search returns all chunk types

GIVEN a workspace with both chunk_type='raw' and chunk_type='symbol' chunks
WHEN memory_search is called without chunk_type param
THEN results from BOTH chunk types MUST be returned

#### Scenario: Filtered search returns only requested chunk type

GIVEN a workspace with both chunk_type='raw' and chunk_type='symbol' chunks
WHEN memory_search is called with chunk_type='symbol'
THEN ONLY chunk_type='symbol' chunks MUST be returned

## Overview

When nano-brain indexes source files with supported languages (Go, TypeScript, JavaScript, Python), the indexer MUST split content at function/method/type boundaries using Tree-sitter AST parsing, rather than fixed character count. Each complete symbol becomes one chunk.

## Requirements

### REQ-1: Symbol boundary chunking

WHEN a supported source file (`.go`, `.ts`, `.tsx`, `.js`, `.jsx`, `.py`) is indexed
THEN each top-level function, method, type declaration, and const/var block MUST produce exactly one chunk
AND no function body SHALL be split across multiple chunks
AND each chunk SHALL have `chunk_type = 'symbol'`
AND each chunk SHALL have `symbol_name`, `symbol_kind`, `language`, `line_start`, `line_end` populated

### REQ-2: Dispatcher routing

WHEN a file is indexed
AND the file extension is in `{.go, .ts, .tsx, .js, .jsx, .py}`
THEN the SymbolAwareChunker MUST be used
AND WHEN the file extension is `.md` or `.mdx`
THEN the HeadingChunker MUST be used
AND WHEN the file extension is anything else
THEN the FixedSizeChunker MUST be used

### REQ-3: Graceful fallback

WHEN Tree-sitter fails to parse a file
OR WHEN symbol extraction returns 0 symbols
THEN the FixedSizeChunker MUST be used as fallback
AND a WARN log MUST be emitted with file path and reason
AND indexing MUST NOT be blocked or return an error

### REQ-4: Large symbol fallback

WHEN a symbol body exceeds 8192 bytes
THEN the symbol MUST NOT be indexed as a single chunk
AND the FixedSizeChunker MUST be used for that symbol's byte range
AND a WARN log SHOULD be emitted with symbol name and size

### REQ-5: Nested symbol scope

WHEN a function contains nested closures or inner functions
THEN only the outermost function boundary MUST be used for chunking
AND nested symbols MUST remain part of the parent chunk content

### REQ-6: Schema migration

WHEN the migration runs
THEN the `chunks` table MUST gain columns: `symbol_name TEXT`, `symbol_kind TEXT`, `language TEXT`, `line_start INTEGER`, `line_end INTEGER`, `chunk_type TEXT NOT NULL DEFAULT 'raw'`, `embedding_strategy TEXT NOT NULL DEFAULT 'raw_code'`
AND all existing rows MUST be backfilled: `chunk_type = 'raw'`, `embedding_strategy = 'raw_code'`
AND two indexes MUST be created: `idx_chunks_symbol_name` (partial, WHERE symbol_name IS NOT NULL), `idx_chunks_chunk_type`
AND the migration MUST be idempotent (safe to re-run)

### REQ-7: Reindex support

WHEN `POST /api/v1/reindex` is called for a workspace
THEN source files MUST be re-chunked using SymbolAwareChunker (if enabled)
AND new symbol chunks MUST be inserted before old fixed-size chunks for the same file are deleted
AND on server startup, if a workspace has `chunk_type='raw'` chunks and symbol-aware chunking is enabled, a WARN log MUST be emitted

### REQ-8: MCP memory_symbols update

WHEN `memory_symbols` MCP tool returns results
THEN each result MUST include a `summary` field (null in v1)
AND existing fields (`name`, `kind`, `language`, `source_path`, `line_start`, `line_end`) MUST remain unchanged
AND the response MUST be backward compatible (agents that don't expect `summary` are unaffected)

### REQ-9: memory_search chunk_type filter

WHEN `memory_search`, `memory_query`, or `memory_vsearch` is called
THEN ALL chunk types MUST be returned by default (no filtering)
AND an optional `chunk_type` parameter MUST be accepted to filter results
AND existing search behavior for `chunk_type='raw'` chunks MUST be unchanged

### REQ-10: Atomic parse pass

WHEN SymbolAwareChunker processes a file
THEN the file MUST be parsed exactly once via Tree-sitter
AND the same parsed tree MUST be used for both symbol name extraction and byte range extraction
AND no second parse pass is permitted (prevents race conditions on file modification)

## Acceptance Criteria

| ID | Criteria | Verification |
|----|----------|-------------|
| AC-1 | Go file with 5 functions → 5 symbol chunks, no function split | Integration test: index fixture file, assert chunk count + boundaries |
| AC-2 | `memory_search("ExtractEdges")` returns chunk with `symbol_name="ExtractEdges"`, `chunk_type="symbol"` | Integration test: index go_extractor.go, search |
| AC-3 | Markdown file indexed → `chunk_type='raw'`, heading-aware (no regression) | Integration test: index .md file |
| AC-4 | Unsupported extension (`.yaml`) → fixed-size chunks, `chunk_type='raw'` | Integration test |
| AC-5 | Tree-sitter parse failure → fallback chunker used, WARN logged, no error returned | Unit test with corrupt .go file |
| AC-6 | Symbol >8KB → fixed-size fallback for that symbol, WARN logged | Unit test with generated large function |
| AC-7 | Schema migration idempotent — running twice produces no error | Integration test |
| AC-8 | Existing raw chunks unaffected after migration (content_hash unchanged) | Integration test |
| AC-9 | `memory_symbols` response includes `summary: null` field | Integration test: MCP call |
| AC-10 | `go test -race -short ./...` passes | CI |
| AC-11 | `go test -race -tags=integration ./...` passes | CI |
| AC-12 | Smoke E2E: server starts → file indexed → `memory_search` returns symbol chunk with metadata | Smoke test script |

## Non-Goals (v1)

- LLM summary generation
- `embedding_strategy` switching (always `raw_code` in v1)
- Sub-chunking large symbols (>8KB skipped, not split)
- New language extractors beyond Go/TS/JS/Python
- Cost estimation CLI
- Dual vector indexes
