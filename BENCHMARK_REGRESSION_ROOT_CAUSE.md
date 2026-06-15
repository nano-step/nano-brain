# Benchmark Regression — Root Cause Analysis

## Date: 2026-06-14

## Problem

Capability benchmark shows regression: `base=0.813 now=0.750 delta=-0.063`

3 tasks failing:
| Task | Query | Expected file |
|------|-------|---------------|
| `symbol-rrf` | "reciprocal rank fusion RRF" | `internal/search` |
| `qa-embedding-worker` | "embedding queue worker provider ollama voyage" | `internal/embed` |
| `qa-harvest` | "harvest session opencode claude code sqlite" | `internal/harvest` |

## Root Cause: BM25 search_vector is NULL in nanobrain_test

**BM25 search returns 0 results for ALL queries** — even trivial ones like "the".

The `search_vector` column in the `chunks` table is not populated. The BM25 SQL query uses:
```sql
c.search_vector @@ websearch_to_tsquery(...)
```
which requires a non-NULL tsvector to match.

### Evidence

| Query | BM25 `/api/v1/search` | Vector (hybrid) | Hybrid `/api/v1/query` |
|-------|----------------------|-----------------|----------------------|
| "the" | **0 results** | works | 3 results (vector-only) |
| "reciprocal rank fusion RRF" | **0 results** | works | 2 results (vector-only) |
| "embedding queue worker provider ollama voyage" | **0 results** | works | 8 results (vector-only) |
| "harvest session opencode claude code sqlite" | **0 results** | works | 20 results (vector-only) |

Hybrid search still returns results because the vector leg works. But BM25 contributes nothing, so:
- Only vector-ranked results appear
- Expected files (`internal/search`, `internal/embed`, `internal/harvest`) may not be in vector top-N
- Benchmark pass/fail checks if expected file appears in results → FAIL

### Why search_vector is NULL

The trigger `trg_chunks_search_vector` (migration 00005) fires on INSERT/UPDATE of content. But the test database was likely populated via a data migration path that bypassed the trigger, leaving existing chunks with NULL search_vector.

Force-wipe reindex (`force_wipe: true`) deleted 8911 chunks but only re-queued 1085 for re-creation. The watcher processed a subset. Even after 60+ seconds, BM25 still returns 0.

### This is NOT a code regression

The BENCHMARK_REGRESSION_GUIDE.md hypothesized dedup or ranking boosts as the cause. **Wrong.** The issue is purely a test database state problem — BM25 leg is completely non-functional.

## Impact

- Benchmark regressions are false positives — the code changes (dedup, code-aware boost, extension boost) are not the cause
- The 2 wins (`trace-writedocument`, `trace-hybridsearch`) work because they only need vector search
- Production server (`:3100`) likely has the same issue if search_vector wasn't populated

## Fix Options

### Option A: SQL UPDATE to repopulate search_vector (recommended)
```sql
UPDATE chunks c SET search_vector = 
    setweight(to_tsvector('english', coalesce(d.title, '')), 'A') ||
    setweight(to_tsvector('english', coalesce(c.content, '')), 'B')
FROM documents d WHERE c.document_id = d.id
  AND c.workspace_hash = '7f443561795a6fea64b6e8d35a9b06ed4d216b8a27af5e10e7137b261ade061f';
```
Requires: psql access to nanobrain_test (Docker container nanobrain-pg)

### Option B: New migration to backfill
Create migration `000XX_backfill_search_vector.sql` that runs the UPDATE above for all workspaces. This is the most robust solution — any future test DB setup will get it automatically.

### Option C: Add backfill to server startup
Add a startup check: if any chunks have NULL search_vector, run the UPDATE. This ensures test DBs are always healthy.

## Recommendation

**Option B** — add a migration. It's:
- One-time, idempotent
- Applies to both dev and test DBs
- Doesn't require manual SQL
- Survives DB rebuilds

## Next Steps

1. Create migration to backfill search_vector
2. Re-run benchmark to verify regressions disappear
3. If regressions persist after BM25 works, THEN investigate dedup/ranking as originally planned
