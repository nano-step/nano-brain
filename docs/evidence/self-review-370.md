# Self-Review — Issue #370 / Symbol-Aware Chunking

Date: 2026-06-04
Branch: feat/370-symbol-aware-chunking
Change type: user-feature
Lane: high-risk

## Changes Summary

| Area | Files | Description |
|------|-------|-------------|
| Schema | `internal/storage/queries/chunks.sql` | Added `symbol_name`, `symbol_kind`, `language`, `line_start`, `line_end`, `chunk_type`, `embedding_strategy` to UpsertChunk |
| Schema | `internal/storage/queries/embeddings.sql` | Added `chunk_type` filter to all 4 vector search queries |
| Schema | `internal/storage/queries/search.sql` | Added `chunk_type` filter to all 4 BM25 search queries |
| Generated | `internal/storage/sqlc/*.go` | sqlc-regenerated models and query functions |
| MCP | `internal/mcp/tools.go` | Added `chunk_type` param to memory_query, memory_search, memory_vsearch |
| Watcher | `internal/watcher/watcher.go` | Replaced `chunk.Split` with `chunker.Dispatcher`, passes symbol metadata to UpsertChunk |
| Summarize | `internal/summarize/persist.go` | Minor: passes new chunk fields |
| CLI | `cmd/nano-brain/main.go` | Wires chunker.Dispatcher at startup |
| Cleanup | `cmd/nano-brain/cmd_detect_changes.go` | Removed unused `getChangedLineRanges`/`parseHunkHeaders` |
| Cleanup | `internal/server/handlers/events_test.go` | Removed unused test helpers |
| Cleanup | `internal/server/handlers/documents_test.go`, `graph_overview_test.go` | Fixed unchecked json.Unmarshal |
| Cleanup | `internal/server/handlers/graph_neighborhood.go` | gosimple: variadic append |
| Cleanup | `cmd/nano-brain/workspaces_test.go` | gosimple: `out.Bytes()` |
| Harvest | `internal/harvest/opencode_sqlite_test.go` | Fixed assertions for SQL-level session pre-filtering |

## Response Shape Verification

- `chunk_type` parameter: validated as `"raw"` or `"symbol"` with clear error message
- NULL handling: `sql.NullString{}` when omitted (no filter applied)
- SQL: `AND (sqlc.narg('chunk_type')::text IS NULL OR c.chunk_type = sqlc.narg('chunk_type'))` — safe NULL pass-through
- UpsertChunk: all 15 params mapped correctly in `writeChunks`

## Staged Files Check

Only project source files staged. No `.opencode/`, `package-lock.json`, or runtime state files.

## Validation Ladder

| Layer | Status |
|-------|--------|
| `go build ./...` | PASS |
| `go test -race -short ./...` | PASS |
| `go test -race -tags=integration ./...` | PASS |
| `golangci-lint run ./...` | PASS (0 issues) |

## Risk Assessment

- **Data model change**: New columns are nullable with defaults — backward-compatible. Existing chunks retain NULL for new fields.
- **Search quality**: `chunk_type` filter is opt-in (NULL = no filter). Existing queries unaffected.
- **Public API**: New optional MCP parameter — additive, non-breaking.
- **Migration**: `00016_symbol_aware_chunking.sql` adds columns with defaults. Safe for rolling deploy.

## Unresolved

- Smoke E2E not yet run (requires PR + running server)
- Full Oracle review pending (will happen at PR stage)
