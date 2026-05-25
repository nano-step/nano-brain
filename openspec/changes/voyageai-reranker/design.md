## Context

The search pipeline already has full reranking support: `positionAwareBlend()` with tiered weights, result caching in `llm_cache`, and the `Reranker` interface in `reranker.ts`. The only missing piece is an actual provider. VoyageAI's rerank API is a single HTTP POST that maps directly to the existing interface.

## Goals / Non-Goals

**Goals:**
- Working reranker with zero new npm dependencies (just `fetch`)
- Configurable model via config.yml
- Graceful degradation on API failure (skip reranking, use RRF-only)
- Token usage tracking via existing `onTokenUsage` callback

**Non-Goals:**
- Model fallback chain (all models share org-level rate limits, so fallback between models doesn't help for quota)
- Retry logic (the search pipeline already caches results; next query will retry)

## Decisions

### D1: Config structure

```yaml
reranker:
  model: rerank-2.5-lite    # default
  apiKey: voyage-xxx         # required
```

Add `RerankerConfig` to `CollectionConfig`. The `apiKey` can also be read from `embedding.apiKey` if the user uses VoyageAI for both embedding and reranking.

### D2: API call

```
POST https://api.voyageai.com/v1/rerank
Authorization: Bearer <apiKey>
{
  "query": "...",
  "documents": ["...", "..."],
  "model": "rerank-2.5-lite",
  "top_k": <candidate count>,
  "truncation": true
}
```

Response maps to existing `RerankResult`: `results[].index` → `index`, `results[].relevance_score` → `score`, document id from input → `file`.

### D3: Error handling

On any non-200 response or network error: log warning, return null from `rerank()`. The search pipeline already handles `reranker === null` by falling back to RRF-only results.

### D4: Token tracking

VoyageAI returns `total_tokens` in the response. Pass to `onTokenUsage(model, tokens)` callback for the existing token usage tracking in store.

## Files Changed

| File | Changes |
|------|---------|
| `src/reranker.ts` | Replace stub with `VoyageAIReranker` class implementing `Reranker` interface |
| `src/types.ts` | Add `RerankerConfig` interface, add `reranker?` field to `CollectionConfig` |
| `src/server.ts` | Read reranker config, pass to `createReranker()`, update model status |
| `src/index.ts` | Read reranker config in CLI search path, pass to `createReranker()` |
