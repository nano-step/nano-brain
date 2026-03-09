## 1. Tree-sitter AST Exposure

- [ ] 1.1 Add `parseToAST(code: string, lang: string): Tree | null` export to `src/treesitter.ts`
- [ ] 1.2 Add unit tests for `parseToAST()` in `test/treesitter.test.ts`

## 2. AST-Aware Chunking

- [ ] 2.1 Implement `chunkWithTreeSitter()` in `src/chunker.ts` with AST node traversal
- [ ] 2.2 Add chunk boundary detection for TS/JS nodes: `function_declaration`, `method_definition`, `class_declaration`, `interface_declaration`, `arrow_function`
- [ ] 2.3 Add chunk boundary detection for Python nodes: `function_definition`, `class_definition`
- [ ] 2.4 Implement size limit enforcement (MIN=50, MAX=3600, 15% tolerance) with recursion into children
- [ ] 2.5 Implement line-based fallback for oversized leaf nodes
- [ ] 2.6 Add metadata header generation (file path, language, line range)
- [ ] 2.7 Implement segment hash deduplication (SHA-256 of path + lines + size)
- [ ] 2.8 Add fallback to `chunkSourceCode()` for unsupported languages and parse failures

## 3. AST Chunking Integration

- [ ] 3.1 Wire `chunkWithTreeSitter()` into `src/codebase.ts` at line 338 (replace `chunkSourceCode()` call)
- [ ] 3.2 Add language detection to route supported languages to AST chunking

## 4. AST Chunking Tests

- [ ] 4.1 Add tests for single function extraction in `test/chunker.test.ts`
- [ ] 4.2 Add tests for class method chunking
- [ ] 4.3 Add tests for oversized node recursion
- [ ] 4.4 Add tests for fallback to regex chunking
- [ ] 4.5 Add tests for metadata header format

## 5. Parallel Embedding

- [ ] 5.1 Add `p-limit` dependency to `package.json`
- [ ] 5.2 Refactor `embedPendingCodebase()` in `src/codebase.ts` to use pLimit with EMBEDDING_CONCURRENCY=3
- [ ] 5.3 Implement backpressure control with MAX_PENDING_BATCHES=10
- [ ] 5.4 Implement sequential fallback on batch failure
- [ ] 5.5 Add environment variable support for `NANO_BRAIN_EMBEDDING_CONCURRENCY`

## 6. Parallel Embedding Tests

- [ ] 6.1 Add tests for concurrent batch processing in `test/codebase.test.ts`
- [ ] 6.2 Add tests for backpressure behavior
- [ ] 6.3 Add tests for sequential fallback on failure

## 7. Integration Testing

- [ ] 7.1 Add integration test: full index cycle with AST chunks + parallel embedding
- [ ] 7.2 Benchmark indexing time on sample codebase (before/after comparison)
