# internal/search

Hybrid search pipeline: BM25 + vector + RRF fusion + recency decay.

## Pipeline

```
Query → BM25 (ts_rank_cd) ──┐
                              ├─→ RRF Fusion (k=60) → Recency Decay → Results
Query → Vector (HNSW cos) ──┘
```

Both legs run concurrently via `errgroup`. One leg failing degrades gracefully;
both failing returns an error.

## Files

| File | Responsibility |
|------|---------------|
| `service.go` | `SearchService` — `Querier` + `Embedder`, config behind `RWMutex`, `HybridSearch` |
| `rrf.go` | `RRFMerge` — `score = Σ 1/(k + rank + 1)`, deduplicates by chunk ID |
| `recency.go` | `ApplyRecencyBoost` — normalizes scores to [0,1], exponential half-life decay |
| `search.go` | `Result` struct — shared return type for all search paths |

## Key Interfaces

- `Querier`: `BM25Search`, `BM25SearchAll`, `VectorSearch`, `VectorSearchAll`
- `Embedder`: `Embed(ctx, text) ([]float32, error)`, `Dimension() int`

`*All` variants ignore workspace scoping (used for cross-workspace queries).

## Config Tunables

| Field | Default | Effect |
|-------|---------|--------|
| `rrf_k` | 60 | Smoothing constant — higher = rank position matters less |
| `recency_weight` | 0.3 | Recency vs relevance blend |
| `recency_half_life_days` | 180 | Days until recency multiplier halves |
| `limit` | 20 | Default max results |

`UpdateConfig` swaps config atomically under a write lock.

## Recency Formula

`final = (1 - weight) * normalized_rrf + weight * exp(-ln2 * age / half_life)`

## Workspace Isolation

All queries require a workspace hash. Pass `"all"` to search across workspaces.
`HybridSearch` fetches `max(maxResults*3, 30)` candidates per leg before fusion.
