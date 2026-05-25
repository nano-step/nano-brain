## 1. Schema Migration

- [x] 1.1 Update `llm_cache` CREATE TABLE in `src/store.ts` to include `project_hash TEXT NOT NULL DEFAULT 'global'`, `type TEXT NOT NULL DEFAULT 'general'`, and composite primary key `(hash, project_hash)`
- [x] 1.2 Add migration logic in `createStore()` to detect old schema (missing `project_hash` column) and rebuild table with new columns, preserving existing data with defaults

## 2. Store Interface & Prepared Statements

- [x] 2.1 Update `Store` interface in `src/types.ts`: add optional `projectHash` and `type` params to `getCachedResult`, `setCachedResult`; update `clearQueryEmbeddingCache` to accept optional `projectHash`
- [x] 2.2 Update prepared statements in `src/store.ts`: `getCachedResultStmt` to filter by `(hash, project_hash)`, `setCachedResultStmt` to insert `project_hash` and `type` columns
- [x] 2.3 Update `getQueryEmbeddingCache` to always use `project_hash = 'global'` and `type = 'qembed'`
- [x] 2.4 Update `setQueryEmbeddingCache` to always set `project_hash = 'global'` and `type = 'qembed'`
- [x] 2.5 Update `clearQueryEmbeddingCache` to `DELETE FROM llm_cache WHERE type = 'qembed'` (scoped to type, not full table wipe)
- [x] 2.6 Add scoped cache deletion method for `clearWorkspace`: delete `llm_cache` entries matching `project_hash`

## 3. Search Pipeline Cache Scoping

- [x] 3.1 Update `hybridSearch` in `src/search.ts`: pass `projectHash` and `type = 'expand'` to `getCachedResult`/`setCachedResult` for expansion cache
- [x] 3.2 Update `hybridSearch` in `src/search.ts`: pass `projectHash` and `type = 'rerank'` to `getCachedResult`/`setCachedResult` for reranking cache
- [x] 3.3 Verify query embedding cache in `hybridSearch` continues to use global scope (no projectHash passed to `getQueryEmbeddingCache`/`setQueryEmbeddingCache`)

## 4. MCP Server Cache Integration

- [x] 4.1 Update `memory_vsearch` handler in `src/server.ts` to confirm embedding cache uses global scope
- [x] 4.2 Update `memory_query` handler in `src/server.ts` to pass `currentProjectHash` (or explicit workspace param) through to `hybridSearch` options so expansion/reranking caches are scoped

## 5. CLI Cache Command

- [x] 5.1 Add `handleCache` function in `src/index.ts` with `clear` and `stats` subcommands
- [x] 5.2 Implement `cache clear`: compute workspace projectHash from cwd, delete matching `llm_cache` entries
- [x] 5.3 Implement `cache clear --all`: delete all `llm_cache` entries
- [x] 5.4 Implement `cache clear --type=<type>`: validate type (`embed`→`qembed`, `expand`, `rerank`), delete matching entries
- [x] 5.5 Implement `cache stats`: query `llm_cache` grouped by `(type, project_hash)`, display counts
- [x] 5.6 Register `cache` command in main CLI switch statement
- [x] 5.7 Update `showHelp()` to include `cache` command documentation

## 6. Tests

- [x] 6.1 Update `createMockStore()` in `test/search.test.ts` to match new `getCachedResult`/`setCachedResult` signatures
- [x] 6.2 Update mock store in `test/watcher.test.ts` to match new signatures
- [x] 6.3 Run `npx vitest run` and verify no new test failures introduced
- [x] 6.4 Run `npx tsc --noEmit` and verify no new type errors introduced
