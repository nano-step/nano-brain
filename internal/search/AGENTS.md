# internal/search

Hybrid search pipeline: BM25 + vector + RRF fusion + recency decay + reranking.

## Pipeline

```
Query → Preprocessor ──→ BM25 ──┐
                                 ├─→ Dynamic RRF → Recency → Entity/PageRank → Reranker → Results
Query ─→ HyDE ─→ Embed ─→ Vector ─┘
```

Both search legs run concurrently via `errgroup`. One leg failing degrades gracefully;
both failing returns an error.

### Phase 2 (Search Quality)

- **HyDE** (Hypothetical Document Embedding): Before the vector leg, an LLM rewrites the query as an "ideal answer" passage to improve semantic retrieval. Gated by `search.hyde.enabled`. Falls back to raw query on timeout/error.
- **Dynamic RRF**: Adjusts the RRF `k` parameter based on BM25/vector overlap — higher overlap → lower k (more aggressive rank weighting). Uses `ComputeRRFK` + `DynamicRRFMerge`.
- **Reranking**: A cross-encoder model reorders top-N results after all boosts. Currently supports Cohere (`rerank-v4.0-pro`). Gracefully degrades to boosted results on failure.

## Files

| File | Responsibility |
|------|---------------|
| `service.go` | `SearchService` — `Querier` + `Embedder`, config behind `RWMutex`, `HybridSearch` |
| `rrf.go` | `RRFMerge`, `DynamicRRFMerge`, `ComputeRRFK` — RRF fusion with dynamic k |
| `recency.go` | `ApplyRecencyBoost` — normalizes scores to [0,1], exponential half-life decay |
| `search.go` | `Result` struct, `Reranker` interface — shared return type for all search paths |
| `hyde/` | HyDE generator — LLM-based query-to-hypothetical-document |
| `reranking/` | Cross-encoder reranker (Cohere adapter) |

## Key Interfaces

- `Querier`: `BM25Search`, `BM25SearchAll`, `VectorSearch`, `VectorSearchAll`
- `Embedder`: `Embed(ctx, text) ([]float32, error)`, `Dimension() int`
- `Reranker`: `Rerank(ctx, query, docs, topK) ([]Result, error)`

`*All` variants ignore workspace scoping (used for cross-workspace queries).

## Config Tunables

| Field | Default | Effect |
|-------|---------|--------|
| `rrf_k` | 60 | Base smoothing constant — dynamic when HyDE/reranking enabled |
| `recency_weight` | 0.3 | Recency vs relevance blend |
| `recency_half_life_days` | 180 | Days until recency multiplier halves |
| `limit` | 20 | Default max results |
| `hyde.enabled` | false | Enable HyDE hypothetical document generation |
| `hyge.provider_url` | "" | OpenAI-compatible LLM endpoint for HyDE |
| `hyde.api_key` | "" | API key for HyDE LLM |
| `hyde.model` | "" | Model name for HyDE |
| `hyde.max_latency_ms` | 500 | Timeout for HyDE LLM call |
| `reranking.enabled` | false | Enable cross-encoder reranking |
| `reranking.provider` | "" | Reranker provider (cohere) |
| `reranking.api_key` | "" | Cohere API key |
| `reranking.top_k` | 20 | Number of results to rerank |

`UpdateConfig` swaps config atomically under a write lock.

## Recency Formula

`final = (1 - weight) * normalized_rrf + weight * exp(-ln2 * age / half_life)`

## Dynamic RRF

`ComputeRRFK` computes overlap ratio between BM25 and vector result sets, then adjusts k:
- High overlap (both legs agree) → lower k (aggressive rank-based separation)
- Low overlap (legs diverge) → higher k (conservative, let both legs contribute)
- Clamped to `[0.5 * baseK, 2.0 * baseK]`

## Workspace Isolation

All queries require a workspace hash. Pass `"all"` to search across workspaces.
`HybridSearch` fetches `max(maxResults*3, 30)` candidates per leg before fusion.
