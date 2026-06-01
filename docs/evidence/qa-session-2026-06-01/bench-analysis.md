# Benchmark Baseline — 2026-06-01

**Workspace:** nano-brain (hash `7f443561...`)
**Version under test:** dev (master HEAD post #297, #300)
**Tool:** `nano-brain bench` (internal/bench/*)

## Quality metrics

| Metric | Value | Status |
|---|---|---|
| precision@5 | 0.008 | ⚠️ Measurement artifact (see analysis) |
| recall@10 | 0.060 | ⚠️ Measurement artifact |
| MRR | 0.032 | ⚠️ Measurement artifact |
| query_count | 50 | — |

## Latency metrics

| Metric | Value | Status |
|---|---|---|
| query_p50_ms | 31.75 | ✅ Excellent |
| query_p95_ms | 120.06 | ✅ Excellent |

## Stress test (concurrent writes)

| Metric | Value | Status |
|---|---|---|
| concurrency | 10 writers | — |
| docs_per_writer | 5 | — |
| documents_written | 50 | — |
| documents_verified | 50 | ✅ |
| violations | 0 | ✅ |
| duration_ms | 68.96 | ✅ Excellent |

## Why quality scores look bad — measurement artifact

The bench `generate` command samples random documents and uses their `source_title` (often a filename basename like `spec.md`, `tasks.md`, `documents_test.go`) as the query string. With 3893 docs in the workspace, MANY documents share the same basename:

- 50 unique queries → 49 unique strings (1 dupe)
- Common queries: `spec.md`, `tasks.md`, `proposal.md`, `extractImports` — each appears 5-20+ times in the workspace as different chunks of different OpenSpec proposals + source files
- Bench expects the EXACT source doc to rank top — but search correctly returns OTHER docs with the same title first

This is an evaluation-dataset design issue, NOT a search quality regression. Real-world queries (verified in RRI-T phase 2) work well:
- "chunker safety net" → 4 results, top score 1.0
- "how do we split documents for embedding" → top score 0.583, correct doc
- Vietnamese "Chào thế giới 🚀" round trip → byte-perfect

## Use as regression baseline

This file (`bench-baseline.json`) is now the reference for future delta detection:

```bash
nano-brain bench compare new.json bench-baseline.json
```

Run after any change to: chunker, embed pipeline, search service, BM25 weights, RRF k, recency weight, embedding model. Any **decline** vs. this baseline = regression. Any **improvement** = quality win to lock in.

## Followups (filed as separate issues going forward)

- Improve `bench generate` to produce realistic semantic queries (not just basenames). LLM-generated query-answer pairs from doc content would give true relevance signal.

## Files

- `bench-dataset.json` — the 50-entry Q-A dataset (input to bench run)
- `bench-baseline.json` — the metrics output (reference baseline)
- `bench-analysis.md` — this file
