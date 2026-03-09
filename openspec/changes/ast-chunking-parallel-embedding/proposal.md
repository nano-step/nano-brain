## Why

Current codebase indexing has two bottlenecks: (1) regex-based chunking in `src/chunker.ts` produces semantically incomplete chunks by splitting at blank lines mid-function, and (2) sequential batch embedding in `src/codebase.ts` is slow for large codebases (2000+ files). Tree-sitter infrastructure already exists in `src/treesitter.ts` but is only used for symbol extraction, not chunking.

## What Changes

- Add `chunkWithTreeSitter()` function that uses existing tree-sitter parsers to extract AST nodes (functions, classes, methods) as chunk boundaries
- Refactor `embedPendingCodebase()` to process batches concurrently using pLimit (3 parallel batches with backpressure)
- Wire AST chunking as default for supported languages (TypeScript, JavaScript, Python), with regex fallback for others
- Add `p-limit` npm dependency for concurrency control

## Capabilities

### New Capabilities

- `ast-chunking`: Tree-sitter based source code chunking that respects AST boundaries (functions, classes, methods) instead of regex heuristics
- `parallel-embedding`: Concurrent batch processing for codebase embedding with configurable concurrency and backpressure

### Modified Capabilities

<!-- No existing spec-level requirements are changing - these are new capabilities -->

## Impact

- `src/chunker.ts` — add `chunkWithTreeSitter()` export alongside existing `chunkSourceCode()`
- `src/treesitter.ts` — expose AST parsing for chunking use (currently only exposes `parseSymbols`)
- `src/codebase.ts` — wire AST chunking at line 338, refactor `embedPendingCodebase()` at line 447
- `package.json` — add `p-limit` dependency
- `test/chunker.test.ts` — add AST chunking tests
- No breaking changes to MCP tools or CLI interface
- No re-embedding required for existing documents (re-embedded on next file change)
