# Tasks: Symbol-Aware Chunking (v1)

Tracking: #370
Date: 2026-06-04

## Phase 0: Pre-Discovery (gate before Phase 1)

- [ ] **T0.1** Locate current chunker instantiation point(s)
  - Find where fixed-size and heading chunkers are currently constructed + injected
  - Document: which file, which struct, how injected (constructor? global?)
  - Verify: DI pattern matches nano-brain conventions (constructor injection)
  - Output: comment in T1.1 with exact file paths

- [ ] **T0.2** Verify `graph.Registry.ExtractEdges` interface
  - Confirm ExtractEdges returns symbol names + kinds via `contains` edges
  - Confirm byte ranges must be extracted separately via `node.StartByte()/EndByte()` (D-BYTE)
  - Confirm all 4 language extractors (Go, TS, JS, Python) support `contains` edges
  - Output: note in T2.1 confirming interface contract

## Phase 1: Infrastructure & Schema

- [ ] **T1.1** Create `internal/chunker/` package skeleton
  - `chunker.go`: `Chunker` interface + extended `Chunk` struct (symbol metadata fields)
  - `fixed.go`: move existing fixed-size chunker here (no logic change)
  - `heading.go`: move existing heading-aware chunker here (no logic change)
  - Verify: `go build ./internal/chunker/...` passes

- [ ] **T1.2** Write goose migration `000NN_symbol_chunks.sql`
  - Add columns: `symbol_name TEXT`, `symbol_kind TEXT`, `language TEXT`, `line_start INTEGER`, `line_end INTEGER`, `chunk_type TEXT NOT NULL DEFAULT 'raw'`, `embedding_strategy TEXT NOT NULL DEFAULT 'raw_code'`
  - Non-symbol chunks: symbol_name=NULL (not ''), line_start=NULL (not 0)
  - Explicit backfill UPDATE for existing rows (chunk_type='raw', embedding_strategy='raw_code')
  - Create indexes: `idx_chunks_symbol_name` (partial, WHERE symbol_name IS NOT NULL), `idx_chunks_chunk_type`
  - Write paired DOWN migration: drops indexes + columns (rollback safety)
  - Verify: up migration runs clean on `nanobrain_test` DB, idempotent on re-run; down migration reverts cleanly

- [ ] **T1.3** Regenerate sqlc queries
  - Run `sqlc generate` after migration
  - Add `UpsertChunkSymbol` query (or extend existing upsert to include new columns)
  - Verify: `go build ./internal/storage/...` passes, no sqlc errors

## Phase 2: SymbolAwareChunker

- [ ] **T2.1** Implement `symbol.go` — `SymbolAwareChunker`
  - Parse file ONCE via `gotreesitter` (reuse tree for both symbol extraction + byte slicing)
  - Extract symbol names via `graph.Registry.ExtractEdges()` (contains edges only)
  - Walk AST nodes by name match → extract byte ranges via `node.StartByte()/EndByte()` (NOT via Edge struct)
  - Outermost scope only — closures stay with parent
  - Skip symbols >8192 bytes → call `fallback.Chunk(content[start:end], sourcePath)` for that range (NOTE: FixedChunker.Chunk accepts a sub-slice, not ChunkRange — slice content before calling)
  - On parse failure or 0 symbols → `fallback.Chunk(content, sourcePath)` + WARN log (parse fail=WARN, 0 symbols=INFO)
  - Singleton — instantiated once at server startup via constructor injection
  - Verify: unit tests pass

- [ ] **T2.2** Unit tests for `SymbolAwareChunker`
  - Go fixture: file with 5 functions → 5 chunks, correct boundaries
  - TypeScript fixture: class with methods → each method = 1 chunk
  - Python fixture: functions + nested closure → outermost only
  - Large function (>8KB) → fixed-size fallback, WARN logged
  - Parse failure (corrupt syntax) → fallback, WARN logged, no panic
  - Empty file → fallback (0 symbols)
  - Verify: `go test -race -short ./internal/chunker/...` passes

- [ ] **T2.3** Implement `dispatcher.go` — `Dispatcher`
  - Route `.go`, `.ts`, `.tsx`, `.js`, `.jsx`, `.py` → `SymbolAwareChunker`
  - Route `.md`, `.mdx` → `HeadingChunker`
  - Route everything else → `FixedChunker`
  - Verify: unit test routing by extension

## Phase 3: Wiring & Integration

- [ ] **T3.1** Wire `Dispatcher` into indexer
  - Replace direct chunker reference in `internal/watcher/` (or indexer) with `Dispatcher`
  - Constructor injection: `NewDispatcher(symbolChunker, headingChunker, fixedChunker)`
  - Verify: `go build ./...` passes

- [ ] **T3.2** Update chunk storage to persist symbol metadata
  - Ensure `UpsertChunk` (or equivalent sqlc query) writes `symbol_name`, `symbol_kind`, `language`, `line_start`, `line_end`, `chunk_type`, `embedding_strategy`
  - Verify: integration test — index Go file, query DB, assert columns populated

- [ ] **T3.3** Update `POST /api/v1/reindex` to use new Dispatcher
  - New symbol chunks inserted before old fixed-size chunks deleted (per-file, not per-workspace)
  - WARN on startup if workspace has stale `chunk_type='raw'` chunks and symbol-aware enabled
  - Verify: integration test — reindex workspace, assert old chunks replaced

## Phase 4: MCP & API Updates

- [ ] **T4.1** Update `memory_symbols` MCP tool response
  - Add optional `summary` field (always `null` in v1)
  - Query `chunks` table (not `graph_edges`) for symbol metadata
  - Add optional `chunk_type` filter param to `memory_search` / `memory_query` / `memory_vsearch`
  - Verify: MCP integration test — call `memory_symbols`, assert `summary: null` in response

- [ ] **T4.2** Update `memory_search` / `memory_query` / `memory_vsearch`
  - Accept optional `chunk_type` filter param (empty = return all)
  - Validate: invalid chunk_type value returns HTTP 400 with error "chunk_type must be raw|symbol or empty"
  - Existing behavior unchanged when param not provided
  - Verify: integration test — search with and without filter, test invalid value returns 400

## Phase 5: Validation

- [ ] **T5.1** Integration tests
  - Index `internal/graph/go_extractor.go` → assert 10+ symbol chunks
  - `memory_search("ExtractEdges")` → returns chunk with `symbol_name="ExtractEdges"`, `chunk_type="symbol"`
  - Index `.md` file → `chunk_type='raw'`, heading-aware (no regression)
  - Migration idempotency: run twice, no error
  - Existing raw chunks unaffected: content_hash unchanged after migration
  - Verify: `go test -race -tags=integration ./...` passes

- [ ] **T5.2** Smoke E2E test
  - Build binary: `go build -o ./bin/nano-brain ./cmd/nano-brain`
  - Start server on port 3199
  - Index a Go source file via `POST /api/v1/reindex`
  - Call `memory_search` → verify symbol chunks returned with metadata
  - Call `memory_symbols` → verify `summary: null` field present
  - Kill server
  - Paste output in `docs/evidence/smoke-e2e-370.md`

- [ ] **T5.3** Self-review
  - `git diff` — verify only files touched per spec
  - `lsp_diagnostics` on all changed files — no errors
  - Response shape check: verify sqlc-generated storage.Chunk struct has all 7 new fields populated when inserting symbol chunks
  - Verify: `go test -race -short ./...` green

- [ ] **T5.4** Performance benchmark
  - Benchmark chunking speed: fixed-size vs symbol-aware on 100-file corpus from `internal/`
  - Record results in `docs/evidence/bench-370.md`
  - Target: symbol-aware ≤2x slower than fixed-size per file
  - If target missed: document in evidence file + add optimization task to v2 backlog

## Phase 6: Harness Gates

- [ ] **T6.1** Run `./scripts/harness-check.sh in-progress` — must PASS
- [ ] **T6.2** Create story evidence file `docs/evidence/story-370.md`
- [ ] **T6.3** Run `./scripts/harness-check.sh pre-merge` — must PASS
- [ ] **T6.4** Open PR with `Closes #370` in body
- [ ] **T6.5** Address bot review comments
- [ ] **T6.6** Run `./scripts/harness-check.sh post-merge` after merge
- [ ] **T6.7** `openspec archive "symbol-aware-chunking"`

## TODO: If search quality degrades after v1

> Add these tasks if integration tests show symbol chunks ranking poorly or returning irrelevant results.

- [ ] **STRETCH-1** Measure search quality: run 20 representative queries before/after, compare Recall@5
- [ ] **STRETCH-2** If BM25 regression found: tune tsvector weights for `symbol_name` field
- [ ] **STRETCH-3** If vector ranking poor: investigate embedding model behavior on code vs prose

## v2 Tasks (not in this PR)

- LLM summary generation pipeline
- `embedding_strategy` workspace toggle
- 8KB sub-chunking with parent-child relationships
- Cost estimation CLI
