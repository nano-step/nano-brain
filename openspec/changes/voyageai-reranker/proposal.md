## Why

The reranking pipeline is fully built (score blending, caching, position-aware weights) but has no working provider — `createReranker()` was gutted when `node-llama-cpp` was removed. VoyageAI offers rerank-2.5/2.5-lite models via a simple HTTP API that fits the existing `Reranker` interface perfectly. This restores reranking with zero new dependencies.

## What Changes

- Replace the stub `createReranker()` with a VoyageAI HTTP-based reranker
- Add `reranker` config section to `CollectionConfig` (model, apiKey)
- Default model: `rerank-2.5-lite` (cheapest, fastest, 32K context)
- On API failure (429/5xx): log warning and skip reranking (fall back to RRF-only gracefully)
- Configurable model via `config.yml` so users can switch to `rerank-2.5` for max accuracy

## Capabilities

### New Capabilities
- `voyageai-reranker`: VoyageAI rerank API integration with configurable model and graceful fallback

### Modified Capabilities

## Impact

- `src/reranker.ts`: Replace stub with VoyageAI HTTP implementation
- `src/types.ts`: Add `RerankerConfig` interface to `CollectionConfig`
- `src/server.ts`: Pass config to `createReranker()`, update model status string
- `src/index.ts`: Pass config to `createReranker()` in CLI search path
