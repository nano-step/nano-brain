## 1. Config types

- [x] 1.1 In `src/types.ts`, add `RerankerConfig` interface: `{ model?: string; apiKey?: string }`.
- [x] 1.2 In `src/types.ts`, add `reranker?: RerankerConfig` field to `CollectionConfig`.

## 2. Reranker implementation

- [x] 2.1 In `src/reranker.ts`, add `VoyageAIReranker` class implementing `Reranker` interface. Constructor takes `apiKey`, `model` (default `rerank-2.5-lite`), optional `onTokenUsage` callback.
- [x] 2.2 Implement `rerank(query, documents)`: POST to `https://api.voyageai.com/v1/rerank` with `{ query, documents: docs.map(d => d.text), model, top_k: docs.length, truncation: true }`. Map response `results[].relevance_score` → `score`, use input `docs[result.index].file` → `file`. Return `{ results, model }`.
- [x] 2.3 On non-200 response or network error: log warning with status/message, return `{ results: [], model }`.
- [x] 2.4 On success: call `onTokenUsage(model, response.total_tokens)` if callback provided.
- [x] 2.5 Implement `dispose()` as no-op.
- [x] 2.6 Update `createReranker()`: accept `options?: { apiKey?: string; model?: string; onTokenUsage?: (model: string, tokens: number) => void }`. If no apiKey, return null. Otherwise return new `VoyageAIReranker`.

## 3. Wire into server

- [x] 3.1 In `src/server.ts`, read `rerankerConfig` from loaded config. Resolve apiKey: `rerankerConfig?.apiKey || embeddingConfig?.apiKey`.
- [x] 3.2 Pass `{ apiKey, model: rerankerConfig?.model, onTokenUsage }` to `createReranker()`.
- [x] 3.3 Update model status string: use actual model name from config instead of hardcoded `'bge-reranker-v2-m3'`.

## 4. Wire into CLI

- [x] 4.1 In `src/index.ts`, in the search/query command handler, read reranker config and pass to `createReranker()` the same way as server.

## 5. Verification

- [x] 5.1 Run `lsp_diagnostics` on all changed files — zero errors.
- [x] 5.2 Run existing tests — no regressions.
