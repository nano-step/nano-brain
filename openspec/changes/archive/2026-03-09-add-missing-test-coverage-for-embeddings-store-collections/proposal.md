# Proposal: Add Missing Test Coverage for Embeddings, Store, and Collections

## Problem

The nano-brain test suite is comprehensive (95%+ coverage), but has 10 identified gaps:

1. **embeddings.ts** — NO dedicated test file. 6 critical functions untested:
   - `createEmbeddingProvider()` (provider selection logic)
   - `checkOllamaHealth()` and `checkOpenAIHealth()` (health checks)
   - `OllamaEmbeddingProvider` class (truncation, batch embedding, context detection)
   - `OpenAICompatibleEmbeddingProvider` class (rate limiting, retry logic, batching, token tracking)

2. **store.ts** — 1 utility function only indirectly tested:
   - `sanitizeFTS5Query()` (FTS5 query escaping — security-critical)

3. **collections.ts** — 3 helper functions missing unit tests:
   - `normalizeCollectionName()`
   - `buildCollectionIndex()`
   - `validateCollectionConfig()`

## Impact

- **embeddings.ts gaps**: HIGH — Embedding providers are core to the search pipeline. Currently tested only via mocks/integration tests. Missing unit tests for provider classes, error handling, rate limiting, and fallback logic.
- **store.ts gaps**: MEDIUM — `sanitizeFTS5Query()` is security-critical (FTS5 injection prevention). Only indirectly tested.
- **collections.ts gaps**: LOW — These helpers are utility functions. Not critical path but good to have.

## Solution

Create three new test files with focused unit tests:

1. **`test/embeddings.test.ts`** (500-700 lines)
   - Test provider detection and selection in `createEmbeddingProvider()`
   - Test Ollama provider (truncation, embedding, batching, model context detection)
   - Test OpenAI-compatible provider (rate limiting, retries, token tracking, batching)
   - Test health check functions with mock fetch

2. **Add to `test/store.test.ts`** (30-50 lines)
   - Unit test `sanitizeFTS5Query()` with edge cases (quotes, empty, special chars)

3. **Add to `test/collections.test.ts`** (100-150 lines)
   - Unit tests for `normalizeCollectionName()`, `buildCollectionIndex()`, `validateCollectionConfig()`

## Effort Estimate

- **embeddings.test.ts**: 4-6 hours (500-700 lines)
- **store.test.ts additions**: 30-45 minutes (30-50 lines)
- **collections.test.ts additions**: 1-1.5 hours (100-150 lines)
- **Total**: 6-8 hours

## Success Criteria

✅ All 10 identified test gaps are filled with unit tests
✅ New tests follow existing patterns (mocking, fixtures, async handling)
✅ Code coverage for embeddings.ts increases from 0% to 90%+
✅ All new tests pass (`npm run test`)
✅ No regressions in existing tests
