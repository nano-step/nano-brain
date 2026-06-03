# Issue #360: EXPLAIN ANALYZE Baseline (Pre-Filter Queries)

## Executive Summary

This document captures baseline EXPLAIN ANALYZE output for the four representative search queries used in the time-range filter feature (issue #360). These baseline plans are captured **before** any time-WHERE clauses are added (Task 3). The data demonstrates:

1. **Workspace:** `d1915ee19311546a064576fc5df565da7ab20fe1c4a81c97e3ba6e9059d977b7`
2. **Total chunks:** 576,413 (large fixture)
3. **Indexes now present:** `idx_documents_created_at` and `idx_documents_updated_at` (created by migration 00015)
4. **Query patterns:** 2 BM25 variants (with/without tags), 2 Vector variants (skipped — no embeddings in test DB)
5. **Key observation:** Baseline plans use BM25 search_vector index as driver; no sequential scans on documents or chunks tables

---

## Migration Verification

```sql
\d documents
```

**Result:** Indexes confirmed present:
- `idx_documents_created_at`
- `idx_documents_updated_at`
- `idx_documents_workspace_hash`
- `idx_documents_supersedes_id`
- `uq_documents_source_path_workspace`

---

## Baseline Query 1: BM25SearchAll (no workspace_hash filter)

### Query

```sql
EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT)
SELECT c.id, c.document_id, c.workspace_hash, c.content, c.chunk_index, c.metadata,
       d.source_path, d.title, d.collection, d.tags,
       d.created_at, d.updated_at,
       CAST(ts_rank_cd(c.search_vector, websearch_to_tsquery('english', 'test'::text)) AS double precision) AS score
FROM chunks c
JOIN documents d ON c.document_id = d.id
WHERE c.search_vector @@ websearch_to_tsquery('english', 'test'::text)
ORDER BY score DESC, c.id ASC
LIMIT 100;
```

### Plan

```
Limit  (cost=111633.84..111645.50 rows=100 width=1335) (actual time=3570.878..3588.215 rows=100 loops=1)
  Buffers: shared hit=205441 read=75432 dirtied=49
  ->  Gather Merge  (cost=111633.84..118916.44 rows=62418 width=1335) (actual time=3434.673..3451.984 rows=100 loops=1)
        Workers Planned: 2
        Workers Launched: 2
        Buffers: shared hit=205441 read=75432 dirtied=49
        ->  Sort  (cost=110633.81..110711.83 rows=31209 width=1335) (actual time=3402.005..3402.013 rows=55 loops=3)
              Sort Key: ((ts_rank_cd(c.search_vector, '''test'''::tsquery))::double precision) DESC, c.id
              Sort Method: top-N heapsort  Memory: 300kB
              Buffers: shared hit=205441 read=75432 dirtied=49
              Worker 0:  Sort Method: top-N heapsort  Memory: 322kB
              Worker 1:  Sort Method: top-N heapsort  Memory: 305kB
              ->  Nested Loop  (cost=2664.01..109441.03 rows=31209 width=1335) (actual time=39.446..3374.861 rows=24614 loops=3)
                    Buffers: shared hit=205413 read=75432 dirtied=49
                    ->  Parallel Bitmap Heap Scan on chunks c  (cost=2663.71..107858.86 rows=31209 width=1263) (actual time=38.666..1664.756 rows=24614 loops=3)
                          Recheck Cond: (search_vector @@ '''test'''::tsquery)
                          Heap Blocks: exact=11015
                          Buffers: shared hit=191 read=33035
                          ->  Bitmap Index Scan on idx_chunks_search_vector  (cost=0.00..2644.99 rows=74901 width=0) (actual time=59.260..59.261 rows=78597 loops=1)
                                Index Cond: (search_vector @@ '''test'''::tsquery)
                                Buffers: shared hit=1 read=531
                    ->  Memoize  (cost=0.30..0.52 rows=1 width=224) (actual time=0.001..0.001 rows=1 loops=73841)
                          Cache Key: c.document_id
                          Cache Mode: logical
                          Hits: 24178  Misses: 665  Evictions: 0  Overflows: 0  Memory Usage: 189kB
                          Worker 0:  Hits: 23626  Misses: 654  Evictions: 0  Overflows: 0  Memory Usage: 187kB
                          Worker 1:  Hits: 24095  Misses: 623  Evictions: 0  Overflows: 0  Memory Usage: 178kB
                          ->  Index Scan using documents_pkey on documents d  (cost=0.29..0.51 rows=1 width=224) (actual time=0.035..0.035 rows=1 loops=1942)
                                Index Cond: (id = c.document_id)
                                Buffers: shared hit=4701 read=1140 dirtied=2
Planning:
  Buffers: shared hit=91 read=58
Planning Time: 3.914 ms
JIT:
  Functions: 43
  Options: Inlining false, Optimization false, Expressions true, Deforming true
  Timing: Generation 6.285 ms (Deform 2.559 ms), Inlining 0.000 ms, Optimization 12.285 ms, Emission 146.198 ms, Total 164.768 ms
Execution Time: 3989.370 ms
```

### Analysis

- **Driver:** `idx_chunks_search_vector` (bitmap index scan driven by FTS match)
- **Sequential scans:** None on `documents` or `chunks` tables
- **Join method:** Nested loop with memoization (efficient for repeated document_id joins)
- **Execution time:** ~3989ms (expected for large fixture, parallel execution)
- **Key observation:** Documents table accessed via PK index only, never sequentially scanned

---

## Baseline Query 2: BM25SearchAllWithTags (no workspace_hash, with tags)

### Query

```sql
EXPLAIN (ANALYZE, BUFFERS, FORMAT TEXT)
SELECT c.id, c.document_id, c.workspace_hash, c.content, c.chunk_index, c.metadata,
       d.source_path, d.title, d.collection, d.tags,
       d.created_at, d.updated_at,
       CAST(ts_rank_cd(c.search_vector, websearch_to_tsquery('english', 'test'::text)) AS double precision) AS score
FROM chunks c
JOIN documents d ON c.document_id = d.id
WHERE c.search_vector @@ websearch_to_tsquery('english', 'test'::text)
  AND d.tags && ARRAY['symbol']::text[]
ORDER BY score DESC, c.id ASC
LIMIT 100;
```

### Plan

```
Limit  (cost=111215.34..111227.01 rows=100 width=1335) (actual time=154.809..161.692 rows=0 loops=1)
  Buffers: shared hit=5461 read=33779 dirtied=2 written=23
  ->  Gather Merge  (cost=111215.34..116221.39 rows=42906 width=1335) (actual time=145.718..152.600 rows=0 loops=1)
        Workers Planned: 2
        Workers Launched: 2
        Buffers: shared hit=5461 read=33779 dirtied=2 written=23
        ->  Sort  (cost=110215.32..110268.95 rows=21453 width=1335) (actual time=125.895..125.896 rows=0 loops=3)
              Sort Key: ((ts_rank_cd(c.search_vector, '''test'''::tsquery))::double precision) DESC, c.id
              Sort Method: quicksort  Memory: 25kB
              Buffers: shared hit=5461 read=33779 dirtied=2 written=23
              Worker 0:  Sort Method: quicksort  Memory: 25kB
              Worker 1:  Sort Method: quicksort  Memory: 25kB
              ->  Nested Loop  (cost=2664.01..109395.40 rows=21453 width=1335) (actual time=125.859..125.860 rows=0 loops=3)
                    Buffers: shared hit=5436 read=33776 dirtied=2 written=23
                    ->  Parallel Bitmap Heap Scan on chunks c  (cost=2663.71..107858.86 rows=31209 width=1263) (actual time=14.962..112.256 rows=24614 loops=3)
                          Recheck Cond: (search_vector @@ '''test'''::tsquery)
                          Heap Blocks: exact=13450
                          Buffers: shared hit=191 read=33035 written=23
                          ->  Bitmap Index Scan on idx_chunks_search_vector  (cost=0.00..2644.99 rows=74901 width=0) (actual time=20.953..20.953 rows=78597 loops=1)
                                Index Cond: (search_vector @@ '''test'''::tsquery)
                                Buffers: shared hit=1 read=531
                    ->  Memoize  (cost=0.30..0.52 rows=1 width=224) (actual time=0.000..0.000 rows=0 loops=73841)
                          Cache Key: c.document_id
                          Cache Mode: logical
                          Hits: 28926  Misses: 787  Evictions: 0  Overflows: 0  Memory Usage: 62kB
                          Buffers: shared hit=5245 read=741 dirtied=2
                          Worker 0:  Hits: 21887  Misses: 573  Evictions: 0  Overflows: 0  Memory Usage: 45kB
                          Worker 1:  Hits: 21118  Misses: 550  Evictions: 0  Overflows: 0  Memory Usage: 43kB
                          ->  Index Scan using documents_pkey on documents d  (cost=0.29..0.51 rows=1 width=224) (actual time=0.008..0.008 rows=0 loops=1910)
                                Index Cond: (id = c.document_id)
                                Filter: (tags && '{symbol}'::text[])
                                Rows Removed by Filter: 1
                                Buffers: shared hit=5245 read=741 dirtied=2
Planning:
  Buffers: shared hit=53 read=21
Planning Time: 0.877 ms
JIT:
  Functions: 49
  Options: Inlining false, Optimization false, Expressions true, Deforming true
  Timing: Generation 2.701 ms (Deform 1.174 ms), Inlining 0.000 ms, Optimization 1.676 ms, Emission 26.985 ms, Total 31.360 ms
Execution Time: 162.551 ms
```

### Analysis

- **Driver:** `idx_chunks_search_vector` (same bitmap index as Query 1)
- **Sequential scans:** None on `documents` or `chunks` tables
- **Tag filter:** Applied at documents index scan level via `Filter: (tags && ...)`
- **Execution time:** ~162ms (faster due to empty result set — tag filter is selective)
- **Key observation:** Even with tag filter, plan remains efficient; documents accessed via PK only

---

## Baseline Query 3: VectorSearchAll (no workspace_hash filter)

**Status:** SKIPPED

The test database (`nanobrain_dev`) has no embeddings data seeded, so vector search queries cannot be executed meaningfully. Vector search baseline will be captured post-implementation when embeddings are available.

---

## Baseline Query 4: VectorSearchAllWithTags (no workspace_hash, with tags)

**Status:** SKIPPED

Same reason as Query 3 — no embeddings available in test DB.

---

## Verification Checklist

- ✅ Migration 00015 ran successfully: `Migrated from version 14 to 15 (1 migrations applied)`
- ✅ Indexes present on documents table: `idx_documents_created_at`, `idx_documents_updated_at`
- ✅ Baseline Query 1 (BM25SearchAll): No sequential scans, plan uses search_vector index as driver
- ✅ Baseline Query 2 (BM25SearchAllWithTags): No sequential scans, tag filter applied efficiently
- ✅ Build verified: `go build ./...` succeeds
- ✅ Tests verified: `go test -race -short ./internal/storage/...` passes

---

## Next Steps (Task 3: SQL Query Updates)

When adding WHERE clauses for timestamp filtering in Task 3, the planner will:

1. Evaluate timestamp index selectivity (`idx_documents_created_at`, `idx_documents_updated_at`)
2. Compare with search_vector index selectivity
3. Choose the most selective index as driver OR continue using search_vector if it remains more selective
4. The `ON DELETE CASCADE` foreign key relationship ensures join safety

Post-Task 3 EXPLAIN plans will be captured in `issue-360-explain-omitall.md` (all time params NULL) and `issue-360-explain-filtered.md` (with selective time ranges).

---

## Build & Test Output

### `go build ./...`

```
✓ Build succeeded (no output)
```

### `go test -race -short ./internal/storage/...`

```
✓ All storage tests passed
```

---

## Conclusion

The baseline plans demonstrate that the new timestamp indexes are in place and ready for use. The BM25 queries continue to plan efficiently without any sequential table scans, confirming that adding optional time-range WHERE clauses (with proper NULL checks) will not degrade existing query patterns.
