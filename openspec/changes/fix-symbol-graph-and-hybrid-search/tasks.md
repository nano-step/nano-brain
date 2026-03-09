## 1. Fix db parameter passing to indexCodebase

- [ ] 1.1 In `src/server.ts` `memory_index_codebase` handler (line 662): add `deps.db` as 6th argument to `indexCodebase()` call
- [ ] 1.2 In `src/watcher.ts` line 146: pass `db` (from watcher deps/closure) as 6th argument to `indexCodebase()`
- [ ] 1.3 In `src/watcher.ts` line 164: pass `db` as 6th argument to the multi-workspace `indexCodebase()` call
- [ ] 1.4 In `src/index.ts` line 820: pass `db` as 6th argument to the CLI `indexCodebase()` call
- [ ] 1.5 Verify `db` is accessible at each call site (check closure/parameter availability)

## 2. Fix sanitizeFTS5Query for multi-word queries

- [ ] 2.1 Modify `sanitizeFTS5Query` in `src/store.ts` (lines 12-17): split trimmed input on whitespace, wrap each token in quotes, join with ` OR `
- [ ] 2.2 Handle edge cases: empty tokens after split, tokens containing double quotes (escape with `""`)
- [ ] 2.3 Update existing tests in `test/integration.test.ts` (lines 9-37): change multi-word expectations from `"hello world"` to `"hello" OR "world"`
- [ ] 2.4 Add new test cases: code identifiers in queries, extra whitespace, hyphenated words

## 3. Add splitIdentifier utility

- [ ] 3.1 Create `splitIdentifier(name: string): string[]` in `src/symbol-graph.ts` — splits camelCase, PascalCase, snake_case, and mixed identifiers into lowercase sub-tokens
- [ ] 3.2 Handle acronyms: `parseHTTPResponse` → `["parse", "http", "response"]`
- [ ] 3.3 Add unit tests for splitIdentifier covering camelCase, snake_case, PascalCase, acronyms, single words

## 4. Add searchByName to SymbolGraph

- [ ] 4.1 Add `searchByName(pattern: string, projectHash: string, limit?: number): SymbolRecord[]` method to `SymbolGraph` class
- [ ] 4.2 Implementation: SQL `SELECT ... FROM code_symbols WHERE project_hash = ? AND LOWER(name) LIKE ?` with `%pattern%` for initial candidates
- [ ] 4.3 Post-filter in TypeScript: split both query and candidate names with `splitIdentifier`, rank by sub-token overlap (exact > prefix > substring)
- [ ] 4.4 Add unit tests for searchByName: exact match, partial camelCase match, case-insensitive, no matches, limit parameter

## 5. Integrate symbol search lane into hybridSearch

- [ ] 5.1 In `src/search.ts` `hybridSearch()`: after FTS+vector search loop, add symbol search call using `SymbolGraph.searchByName()` when `options.db` is defined
- [ ] 5.2 Convert symbol matches to `SearchResult[]` by looking up documents table by file path
- [ ] 5.3 Add symbol result set to `allResultSets` and `weights` arrays before `rrfFuse()` call
- [ ] 5.4 Handle case where symbol's file_path has no corresponding document (silently drop)
- [ ] 5.5 Add tests: symbol-only match surfaces in results, symbol+FTS deduplication via RRF, db=undefined skips symbol lane

## 6. Verify and test

- [ ] 6.1 Run `npx vitest run` — all existing + new tests pass
- [ ] 6.2 Run `lsp_diagnostics` on all changed files — 0 errors
- [ ] 6.3 Manual smoke test: run `memory_index_codebase`, then `memory_query` with a code identifier — verify results include the matching file
