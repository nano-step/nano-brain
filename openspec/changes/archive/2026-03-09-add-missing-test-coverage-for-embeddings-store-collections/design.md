# Design: Test Coverage for Embeddings, Store, and Collections

## Architecture Overview

Three targeted test implementations to close identified coverage gaps:

```
test/
├── embeddings.test.ts (NEW, 500-700 lines)
│   ├── Health checks (checkOllamaHealth, checkOpenAIHealth)
│   ├── OllamaEmbeddingProvider class
│   ├── OpenAICompatibleEmbeddingProvider class
│   └── createEmbeddingProvider factory function
│
├── store.test.ts (APPEND, 30-50 lines)
│   └── sanitizeFTS5Query utility function
│
└── collections.test.ts (APPEND, 100-150 lines)
    ├── normalizeCollectionName
    ├── buildCollectionIndex
    └── validateCollectionConfig
```

## Test Implementation Details

### 1. test/embeddings.test.ts (NEW FILE)

#### Section A: Health Check Functions
- **checkOllamaHealth()**
  - ✅ Mock fetch: returns reachable=true with models list
  - ✅ Mock fetch: returns reachable=false with error message
  - ✅ Mock fetch: timeout handling
  - ✅ Test with different URLs

- **checkOpenAIHealth()**
  - ✅ Mock fetch: returns reachable=true with model name
  - ✅ Mock fetch: returns reachable=false with error text
  - ✅ Test API key and baseUrl formatting
  - ✅ Test timeout handling

#### Section B: OllamaEmbeddingProvider Class
- **Constructor and basic properties**
  - ✅ Initializes with URL, model name
  - ✅ URL trailing slash is removed

- **detectModelContext()**
  - ✅ Parses model info from /api/show endpoint
  - ✅ Detects context length and embedding dimensions
  - ✅ Falls back gracefully if endpoint fails
  - ✅ Updates maxChars based on context length

- **embed(text)**
  - ✅ Calls /api/embed endpoint
  - ✅ Handles response and returns EmbeddingResult
  - ✅ Throws error on HTTP failure

- **embedBatch(texts)**
  - ✅ Batch embedding with multiple texts
  - ✅ Returns array of EmbeddingResults
  - ✅ Truncates long texts using maxChars

- **truncate(text)**
  - ✅ Returns text as-is if <= maxChars
  - ✅ Truncates to maxChars if longer

#### Section C: OpenAICompatibleEmbeddingProvider Class
- **Constructor and configuration**
  - ✅ Initializes with baseUrl, model, apiKey
  - ✅ URL trailing slash removed
  - ✅ Custom maxChars and rpmLimit settings
  - ✅ onTokenUsage callback support

- **throttle() rate limiting**
  - ✅ Tracks request timestamps in 60s window
  - ✅ Waits if rpm limit exceeded
  - ✅ Cleans up old timestamps

- **fetchWithRetry()**
  - ✅ Makes POST request to /v1/embeddings
  - ✅ Handles 429 rate limit with Retry-After header
  - ✅ Retries exponential backoff
  - ✅ Throws after max retries
  - ✅ Throws on non-429 errors

- **embed(text)**
  - ✅ Single text embedding with input_type='query'
  - ✅ Sets dimensions on first embedding
  - ✅ Calls onTokenUsage callback if provided
  - ✅ Truncates text before embedding

- **embedBatch(texts)**
  - ✅ Batches texts to stay under token limits
  - ✅ Splits into sub-batches if needed
  - ✅ Uses input_type='document'
  - ✅ Preserves order and handles all embeddings
  - ✅ Calls onTokenUsage for each sub-batch

#### Section D: createEmbeddingProvider() Factory
- **OpenAI provider path**
  - ✅ Selects OpenAI-compatible if configured
  - ✅ Validates url and apiKey required
  - ✅ Tests connection with embed('test')
  - ✅ Returns null if fails

- **Ollama provider path**
  - ✅ Selects Ollama if not explicitly configured as 'local'
  - ✅ Uses detectOllamaUrl() by default
  - ✅ Health check before instantiation
  - ✅ Calls detectModelContext()
  - ✅ Tests connection with embed('test')

- **Fallback logic**
  - ✅ Falls back from Ollama to null if not configured as explicit
  - ✅ Returns null if explicitly configured but unavailable
  - ✅ Handles both config and no-config cases

### 2. test/store.test.ts (APPEND ~40 lines)

#### New Section: sanitizeFTS5Query()
```javascript
describe('sanitizeFTS5Query', () => {
  it('wraps query in double quotes', () => {
    expect(sanitizeFTS5Query('test')).toBe('"test"');
  });

  it('escapes double quotes inside query', () => {
    expect(sanitizeFTS5Query('test "quoted"')).toBe('"test ""quoted"""');
  });

  it('handles multiple quotes', () => {
    expect(sanitizeFTS5Query('"a" "b"')).toBe('"""a"" ""b"""');
  });

  it('trims whitespace', () => {
    expect(sanitizeFTS5Query('  test  ')).toBe('"test"');
  });

  it('returns empty string for empty/whitespace-only input', () => {
    expect(sanitizeFTS5Query('   ')).toBe('');
  });

  it('handles special FTS5 characters', () => {
    expect(sanitizeFTS5Query('test AND OR NOT')).toBe('"test AND OR NOT"');
  });

  it('handles newlines and tabs', () => {
    const result = sanitizeFTS5Query('test\nquery\ttab');
    expect(result.startsWith('"')).toBe(true);
    expect(result.endsWith('"')).toBe(true);
  });
});
```

### 3. test/collections.test.ts (APPEND ~120 lines)

#### New Sections
- **normalizeCollectionName()**
  - ✅ Converts to lowercase
  - ✅ Replaces spaces with hyphens
  - ✅ Removes special characters (keep alphanumeric + hyphens)
  - ✅ Removes leading/trailing hyphens

- **buildCollectionIndex()**
  - ✅ Indexes collection files by path
  - ✅ Returns Map<filePath, CollectionIndexEntry>
  - ✅ Handles duplicate paths
  - ✅ Returns empty for no files

- **validateCollectionConfig()**
  - ✅ Validates required fields (name, path)
  - ✅ Validates path exists
  - ✅ Validates name format (kebab-case)
  - ✅ Throws error for invalid config
  - ✅ Returns true for valid config

## Testing Strategy

### Mocking Approach
- **Fetch API**: Use `vi.fn()` to mock fetch responses
- **File System**: Use temp directories (already pattern in existing tests)
- **Date/Time**: Mock Date.now() for rate limiting tests
- **Callbacks**: Track onTokenUsage calls with vitest mocks

### Test Fixtures
- Sample embeddings (4 or 8 dimensional vectors)
- Sample Ollama API responses (from real Ollama)
- Sample OpenAI API responses (from OpenAI docs)
- Mock model info responses

### Coverage Goals
- **embeddings.ts**: 90%+ coverage
  - All functions: 100%
  - All class methods: 100%
  - Error paths: All major errors tested

- **sanitizeFTS5Query()**: 100% coverage
  - All edge cases: quotes, whitespace, special chars

- **Collection helpers**: 85%+ coverage
  - Happy path: 100%
  - Error cases: All validation errors

## Risk Mitigation

- ✅ Mock all external API calls (no real Ollama/OpenAI calls during tests)
- ✅ Use temp directories to avoid file system pollution
- ✅ Test rate limiting with fake timers (lolex or vitest timers)
- ✅ Validate error messages and types match expected behavior
- ✅ Run full test suite after changes to catch regressions

## Integration Points

- No changes to source code — **only test additions**
- No modifications to existing test files beyond append-only sections
- No new dependencies required (vitest already available)
- All tests follow existing patterns from store.test.ts and search.test.ts
