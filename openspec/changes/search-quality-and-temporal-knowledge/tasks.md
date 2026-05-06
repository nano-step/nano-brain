## 1. Schema Migration

- [ ] 1.1 Add `domain_type TEXT DEFAULT 'general'` and `last_reinforced_at TEXT` columns to `documents` table via safe additive migration in `src/store/index.ts`
- [ ] 1.2 Verify migration runs without data loss on existing databases

## 2. Fix Supersede Bug + Demotion Factor

- [ ] 2.1 Fix `supersedeDocument` call in `src/mcp/tools/memory.ts` (line 480) and `src/cli/commands/write.ts` (line 83) to pass actual new doc ID instead of `0`
- [ ] 2.2 Reduce `supersede_demotion` multiplier from `0.3` to `0.05` in `src/search.ts`

## 3. Workspace Isolation — FTS

- [ ] 3.1 Remove implicit `'global'` from all FTS SQL filters in `src/store/index.ts` — change `IN (?, 'global')` to `= ?`
- [ ] 3.2 Add `includeGlobal?: boolean` to `StoreSearchOptions`; when `scope === 'all'` skip workspace filter

## 4. Workspace Isolation — Qdrant Vector

- [ ] 4.1 Add `project_hash` field to Qdrant point payload on every upsert in `src/store/vectors.ts`
- [ ] 4.2 Pass `project_hash` payload filter to Qdrant `search()` call for non-global queries
- [ ] 4.3 Implement one-time backfill job: read `(id, project_hash)` from SQLite, update Qdrant points in batches of 100
- [ ] 4.4 Trigger backfill on server startup when unfiltered points are detected (async, non-blocking)

## 5. Temporal Metadata in SearchResult

- [ ] 5.1 Add `createdAt?: string` to `SearchResult` interface in `src/types.ts`
- [ ] 5.2 Populate `createdAt` from `documents.created_at` in FTS search path (`src/fts-worker.ts`)
- [ ] 5.3 Populate `createdAt` from `documents.created_at` in vector search path (`src/store/vectors.ts`)
- [ ] 5.4 Populate `createdAt` in hybrid search result merging (`src/search.ts`)

## 6. Length Normalization

- [ ] 6.1 Add `length_norm_anchor: number` (default `2000`) to `SearchConfig` in `src/types.ts`
- [ ] 6.2 After RRF fusion in `hybridSearch`, apply log-based length penalty: `score *= 1 / (1 + Math.log2(Math.max(1, charLength / anchor)))`
- [ ] 6.3 Pass document `char_length` (or content length) through search pipeline so penalty can be applied

## 7. Recency Boost

- [ ] 7.1 Add `recency_weight: number` (default `0.3`) and `recency_half_life_days: number` (default `180`) to `SearchConfig`
- [ ] 7.2 Implement `applyRecencyBoost(results, config)` function in `src/search.ts`
- [ ] 7.3 Apply recency boost only to results where `collection IN ('sessions', 'memory')` after length normalization
- [ ] 7.4 Verify codebase collection results are not affected

## 8. Validation

- [ ] 8.1 Run a test query and verify top results come from correct workspace
- [ ] 8.2 Verify superseded doc does not appear in top-10 results
- [ ] 8.3 Verify recent session ranks above old session on same topic query
- [ ] 8.4 Run `lsp_diagnostics` on all modified files — zero errors
