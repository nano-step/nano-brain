# Benchmark Regression Investigation Guide

## Context

nano-brain is a Go 1.23 hybrid search server (BM25 + pgvector + RRF fusion).
Test server runs at **http://localhost:3199**, database: `nanobrain_test`.
Workspace hash: `7f443561795a6fea64b6e8d35a9b06ed4d216b8a27af5e10e7137b261ade061f`

## Current Benchmark Result

Run: `go test -v -tags=capbench -run TestCapability ./benchmarks/capability/`

```
OVERALL: base=0.813 now=0.750 delta=-0.063   REGRESSION
```

## Regressions (3 tasks)

| Task | Category | Query | Expected file | Was | Now |
|------|----------|-------|--------------|-----|-----|
| `symbol-rrf` | symbol-lookup | "reciprocal rank fusion RRF" | `internal/search` | 1.0 | 0.0 |
| `qa-embedding-worker` | search-qa | "embedding queue worker provider ollama voyage" | `internal/embed` | 1.0 | 0.0 |
| `qa-harvest` | search-qa | "harvest session opencode claude code sqlite" | `internal/harvest` | 1.0 | 0.0 |

## Wins (2 tasks, do not break these)

| Task | Was | Now |
|------|-----|-----|
| `trace-writedocument` | 0.0 | 1.0 |
| `trace-hybridsearch` | 0.0 | 1.0 |

## Recent Search Pipeline Changes (suspects)

These were added in `internal/search/service.go` in the post-RRF pipeline:

```go
merged  := DynamicRRFMerge(bm25Results, vectorResults, rrfK)
deduped := DeduplicateResults(merged)                          // NEW
codeAware := ApplyCodeAwareBoost(deduped, query, 1.2, 1.3)   // NEW
extBoosted := ApplyExtensionBoost(codeAware, 1.1, 0.9)       // NEW
boosted := ApplyRecencyBoost(extBoosted, ...)
```

Key files:
- `internal/search/dedup.go` — deduplicates by DocumentID (keep highest score) then by content hash (keep shorter path)
- `internal/search/ranking.go` — `ApplyCodeAwareBoost` (query keyword in path/title → 1.2x/1.3x) and `ApplyExtensionBoost` (code files 1.1x, .md files 0.9x)
- `internal/search/service.go` — wires the above into HybridSearch

## Investigation Steps

### 1. Check what the search actually returns for failing queries

```go
// POST http://localhost:3199/api/v1/query
{
  "query": "reciprocal rank fusion RRF",
  "workspace": "7f443561795a6fea64b6e8d35a9b06ed4d216b8a27af5e10e7137b261ade061f",
  "max_results": 10
}
```

Do the same for the other two queries. Check:
- Are results from `internal/search`, `internal/embed`, `internal/harvest` present at all?
- What scores do they have vs what's ranked above them?
- Are they being dropped by dedup (wrong DocumentID dedup)?

### 2. Check the benchmark runner's pass/fail logic

File: `benchmarks/capability/runner.go`

The runner calls `callQuery` which POSTs to `/api/v1/query`. It then checks if any result's `source_path` contains the expected file prefix.

Check:
- What `collection` filter is applied? (`code` only? `all`?)
- Is the `max_results` multiplier large enough to include the expected results?

### 3. Hypothesis: DeduplicateResults drops the correct chunk

`DeduplicateResults` in `dedup.go`:
- Level 1: same `DocumentID` → keep highest-scored chunk
- Level 2: same content hash → keep shorter `source_path`

If a different chunk from the same document scores higher and was already in the result set, the `internal/embed` / `internal/harvest` / `internal/search/rrf.go` chunk may be deduplicated away.

**Test**: Temporarily disable dedup in service.go and re-run benchmark. If regressions disappear, dedup is the culprit.

### 4. Hypothesis: Ranking boosts push wrong results above threshold

`ApplyCodeAwareBoost` boosts results where query keywords appear in source path or title.
`ApplyExtensionBoost` gives 1.1x to `.go` files.

If the correct results have lower RRF scores and other `.go` files score higher after boosts, the correct results may fall outside `max_results`.

**Test**: Check if correct results appear in top 20 but not top 5/10.

## How to Fix

### Option A — Fix dedup (if it's the cause)
- Level 1 dedup may be too aggressive: if a document has many chunks, only the highest-scored is kept. But the highest-scored chunk may not be from the expected file.
- Consider deduping by (workspace, collection, source_path) instead of DocumentID — keeps the best chunk per FILE rather than per DOCUMENT.

### Option B — Increase fetch limit before dedup
Currently `fetchLimit = maxResults * 3` (min 30). Increase to `maxResults * 5` so dedup has more candidates to work with.

### Option C — Apply dedup after truncation
Move dedup to after `reranked[:maxResults]` truncation — only dedup the final result window, not the full candidate set.

## Running the Benchmark

```bash
# Full capability benchmark
go test -v -tags=capbench -run TestCapability ./benchmarks/capability/

# Static integration benchmark (requires DB)
go test -race -tags=integration -run TestBenchmarkNanoBrain ./internal/bench/ -v

# Unit tests only (no DB needed)
go test -race -short ./internal/search/...
```

## Constraints

- **Never kill processes broadly** — check before killing anything
- **Use test server :3199** (`nanobrain_test` DB), never touch dev DB at :3100
- **No agent footers** in commits (no Co-Authored-By lines)
- **Git user**: kokorolx
- **Always branch from `origin/master`**: `git checkout -b fix/<name> origin/master`
