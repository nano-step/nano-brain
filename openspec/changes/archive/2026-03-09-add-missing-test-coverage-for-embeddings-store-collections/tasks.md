# Implementation Tasks

## Phase 1: Create test/embeddings.test.ts (HIGH PRIORITY)

### Task 1.1: Setup and imports
- [ ] Create new file `test/embeddings.test.ts`
- [ ] Import vitest utilities (describe, it, expect, vi, beforeEach)
- [ ] Import functions from `src/embeddings.ts`:
  - `checkOllamaHealth`, `checkOpenAIHealth`
  - `createEmbeddingProvider`, `detectOllamaUrl`
  - `EmbeddingProvider` interface
- [ ] Import types: `EmbeddingConfig`, `EmbeddingResult`
- [ ] Setup mocks for global `fetch` API

### Task 1.2: Health check function tests
- [ ] Write tests for `checkOllamaHealth()`
  - Mock fetch returning 200 OK with model list
  - Mock fetch returning error response
  - Mock fetch timeout
  - Test URL formatting
- [ ] Write tests for `checkOpenAIHealth()`
  - Mock fetch returning 200 OK with model
  - Mock fetch returning error response
  - Test baseUrl formatting and trailing slash handling
  - Test authorization header

### Task 1.3: OllamaEmbeddingProvider tests
- [ ] Test constructor (URL normalization, model setting)
- [ ] Test `detectModelContext()` method
  - Mock /api/show endpoint
  - Parse model info and context length
  - Calculate maxChars from context
  - Graceful fallback if endpoint fails
- [ ] Test `embed()` method
  - Mock /api/embed endpoint
  - Return EmbeddingResult with correct structure
  - Error handling for failed requests
- [ ] Test `embedBatch()` method
  - Multiple texts in single request
  - Truncation of long texts
  - Return array of results in order
- [ ] Test utility methods: `getDimensions()`, `getModel()`, `getMaxChars()`

### Task 1.4: OpenAICompatibleEmbeddingProvider tests
- [ ] Test constructor (baseUrl, model, apiKey normalization)
- [ ] Test `throttle()` rate limiting
  - Track timestamps in 60s window
  - Wait when rpm limit exceeded
  - Cleanup old timestamps
- [ ] Test `fetchWithRetry()` method
  - Mock successful 200 response
  - Mock 429 rate limit with Retry-After header
  - Exponential backoff retry logic (attempt 1, 2, 3)
  - Max retries exceeded error
  - Non-429 errors thrown immediately
- [ ] Test `embed()` method
  - Single text with input_type='query'
  - Dimensions set on first embedding
  - onTokenUsage callback invoked with tokens
  - Text truncation
- [ ] Test `embedBatch()` method
  - Multiple texts batched
  - Sub-batching for large requests (maxCharsPerBatch)
  - input_type='document' for batches
  - Preserve order of results
  - Handle missing embeddings error
  - onTokenUsage called for each sub-batch
- [ ] Test utility methods: `getDimensions()`, `getModel()`, `getMaxChars()`, `getRpmLimit()`

### Task 1.5: Factory function tests
- [ ] Test `createEmbeddingProvider()` with OpenAI config
  - Returns OpenAICompatibleEmbeddingProvider when configured
  - Validates apiKey and url required
  - Tests connection with embed('test')
  - Returns null on connection failure
- [ ] Test `createEmbeddingProvider()` with Ollama
  - Returns OllamaEmbeddingProvider when available
  - Uses detectOllamaUrl() by default
  - Calls detectModelContext()
  - Tests connection with embed('test')
  - Returns null if explicitly configured but unavailable
- [ ] Test `createEmbeddingProvider()` with no config
  - Tries Ollama first
  - Falls back to null if unavailable
- [ ] Test `createEmbeddingProvider()` with local provider
  - Returns null when configured as 'local' and nothing available

---

## Phase 2: Add tests to test/store.test.ts (MEDIUM PRIORITY)

### Task 2.1: Add sanitizeFTS5Query tests
- [ ] Add new describe block: `describe('sanitizeFTS5Query', ...)`
- [ ] Test basic wrapping in double quotes
- [ ] Test escaping of internal quotes ('"' → '""')
- [ ] Test multiple quotes in same string
- [ ] Test whitespace trimming
- [ ] Test empty/whitespace-only input returns ''
- [ ] Test special FTS5 operators not parsed (AND, OR, NOT)
- [ ] Test newlines and tabs preserved
- [ ] Test security: prevent quote injection

---

## Phase 3: Add tests to test/collections.test.ts (LOW PRIORITY)

### Task 3.1: Add helper function tests
- [ ] Add describe block for `normalizeCollectionName()`
  - Lowercase conversion
  - Space → hyphen replacement
  - Remove special characters (keep alphanumeric + hyphens)
  - Remove leading/trailing hyphens
  - Edge cases (empty, single char, all special)
- [ ] Add describe block for `validateCollectionConfig()`
  - Required fields validation (name, path)
  - Name format validation (kebab-case)
  - Path existence check
  - Return true for valid config
  - Throw descriptive errors
- [ ] Add describe block for `buildCollectionIndex()`
  - Index files by path
  - Return Map structure
  - Handle empty input
  - Handle duplicates

---

## Phase 4: Testing and Validation

### Task 4.1: Run tests locally
- [ ] `npm run test -- test/embeddings.test.ts` — all tests pass
- [ ] `npm run test -- test/store.test.ts` — all tests pass (including new ones)
- [ ] `npm run test -- test/collections.test.ts` — all tests pass (including new ones)
- [ ] No test regressions: `npm run test` (full suite)

### Task 4.2: Check coverage
- [ ] embeddings.ts coverage ≥ 90%
- [ ] store.ts sanitizeFTS5Query coverage = 100%
- [ ] collections.ts helper coverage ≥ 85%

### Task 4.3: Code review and finalization
- [ ] All tests follow existing patterns
- [ ] No console warnings or errors
- [ ] All mocks properly cleaned up
- [ ] Comments added for complex test scenarios

---

## Completion Checklist

- [ ] test/embeddings.test.ts created (500-700 lines)
- [ ] test/store.test.ts updated with sanitizeFTS5Query tests
- [ ] test/collections.test.ts updated with helper function tests
- [ ] All tests passing locally
- [ ] Coverage targets met
- [ ] No regressions in existing tests
- [ ] Code follows project conventions
- [ ] Ready for merge
