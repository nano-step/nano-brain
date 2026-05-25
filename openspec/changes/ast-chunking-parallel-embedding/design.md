## Context

nano-brain indexes codebases by chunking source files and generating embeddings. Current implementation:
- `src/chunker.ts` uses regex patterns (FUNCTION_DEF_PATTERNS, blank lines) to split code — can break mid-function
- `src/treesitter.ts` has working tree-sitter parsers for TS/JS/Python but only exposes `parseSymbols()`
- `src/codebase.ts` embeds batches sequentially with `setImmediate` yields — slow for 2000+ file codebases

Roo-Code's architecture (analyzed in `docs/roo-code-indexing-analysis.md`) demonstrates AST-aware chunking and parallel processing as proven patterns.

## Goals / Non-Goals

**Goals:**
- Produce semantically complete chunks that respect AST boundaries
- Reduce initial indexing time for large codebases via parallel embedding
- Reuse existing tree-sitter infrastructure (no new parser dependencies)
- Maintain backward compatibility (no re-embedding required)

**Non-Goals:**
- Adding tree-sitter support for new languages (use existing TS/JS/Python)
- Changing embedding model or vector dimensions
- Modifying MCP tool interfaces
- Real-time incremental re-chunking (still batch-based)

## Decisions

### Decision 1: Expose `parseToAST()` from treesitter.ts

**Choice:** Add a new export `parseToAST(code: string, lang: string): Tree | null` that returns the raw tree-sitter AST.

**Rationale:** `parseSymbols()` extracts specific symbol types but discards the tree. Chunking needs the full AST to traverse and find boundaries. Exposing the tree avoids re-parsing.

**Alternatives considered:**
- Modify `parseSymbols()` to return tree — breaks existing callers
- Duplicate parser initialization in chunker.ts — wasteful, harder to maintain

### Decision 2: AST node types for chunk boundaries

**Choice:** Use these node types as chunk boundaries:
- TypeScript/JavaScript: `function_declaration`, `method_definition`, `class_declaration`, `interface_declaration`, `arrow_function` (top-level only)
- Python: `function_definition`, `class_definition`

**Rationale:** These are the semantic units developers think in. Matches Roo-Code's approach.

**Alternatives considered:**
- Include all statement types — too granular, produces tiny chunks
- Only top-level declarations — misses class methods

### Decision 3: pLimit for concurrency control

**Choice:** Use `p-limit` npm package for concurrent batch processing.

**Rationale:** Well-tested, minimal footprint (2KB), handles edge cases (queue ordering, error propagation). Already a common pattern in Node.js ecosystem.

**Alternatives considered:**
- Inline semaphore implementation — more code to maintain, easy to get wrong
- Worker threads — overkill for I/O-bound embedding calls
- Promise.all with chunked arrays — no backpressure control

### Decision 4: Concurrency = 3, Backpressure = 10

**Choice:** EMBEDDING_CONCURRENCY=3, MAX_PENDING_BATCHES=10

**Rationale:** Conservative defaults that work with most embedding APIs without rate limiting. 3 concurrent batches × 200 chunks = 600 chunks in flight. Backpressure at 10 prevents memory bloat.

**Alternatives considered:**
- Higher concurrency (5-10) — risks API rate limits
- No backpressure — memory issues on very large codebases

### Decision 5: AST chunking as default for supported languages

**Choice:** `chunkWithTreeSitter()` is called first; falls back to `chunkSourceCode()` on failure or unsupported language.

**Rationale:** AST chunks are strictly better when available. Fallback ensures no regression for edge cases.

## Risks / Trade-offs

**[Risk] Tree-sitter parsing slower than regex** → Mitigation: Tree-sitter is compiled to WASM/native, typically <10ms per file. Benchmark during implementation; if >50ms, add caching.

**[Risk] AST chunks may be larger than regex chunks** → Mitigation: 15% tolerance on MAX_BLOCK_CHARS. Large functions that exceed limit recurse into children or fall back to line-based.

**[Risk] Parallel embedding may hit API rate limits** → Mitigation: Conservative default (3 concurrent). Add exponential backoff on 429 responses.

**[Risk] pLimit adds external dependency** → Mitigation: Package is 2KB, well-maintained, MIT licensed. Acceptable trade-off vs. inline implementation bugs.

## Open Questions

1. Should we expose concurrency settings via environment variables? (Leaning yes: `NANO_BRAIN_EMBEDDING_CONCURRENCY`)
2. Should failed batches be retried with exponential backoff or just sequential fallback? (Leaning sequential-only for simplicity)
