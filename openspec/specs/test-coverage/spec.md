# test-coverage Specification

## Purpose
TBD - created by archiving change add-missing-test-coverage-for-embeddings-store-collections. Update Purpose after archive.
## Requirements
### Requirement: Embeddings Module Test Coverage

The system SHALL create comprehensive unit tests for `src/embeddings.ts` module that MUST cover all exported functions and classes.

#### Scenario: Health check functions
- **Given:** checkOllamaHealth() is called with valid URL
- **When:** Ollama responds with 200 OK and model list
- **Then:** Function returns { reachable: true, models: [...] }

- **Given:** checkOpenAIHealth() is called with valid baseUrl and apiKey
- **When:** OpenAI-compatible endpoint responds with 200 OK
- **Then:** Function returns { reachable: true, model: "..." }

#### Scenario: OllamaEmbeddingProvider class
- **Given:** OllamaEmbeddingProvider is instantiated
- **When:** embed() is called with text
- **Then:** Provider calls /api/embed endpoint and returns EmbeddingResult

- **Given:** embedBatch() is called with array of texts
- **When:** Provider sends request to /api/embed
- **Then:** Returns array of EmbeddingResult objects in order

#### Scenario: OpenAICompatibleEmbeddingProvider class
- **Given:** OpenAICompatibleEmbeddingProvider is instantiated with rate limit
- **When:** embed() is called multiple times in rapid succession
- **Then:** Provider throttles requests to respect RPM limit

- **Given:** fetchWithRetry() receives 429 rate limit response
- **When:** Retry-After header is present
- **Then:** Provider waits and retries (max 3 attempts)

#### Scenario: Factory function
- **Given:** createEmbeddingProvider() is called with OpenAI config
- **When:** Config has provider='openai', url, and apiKey
- **Then:** Returns OpenAICompatibleEmbeddingProvider instance

- **Given:** createEmbeddingProvider() is called with no config
- **When:** Ollama is available at default localhost:11434
- **Then:** Returns OllamaEmbeddingProvider instance

- **Given:** createEmbeddingProvider() is called and no provider is available
- **When:** Neither Ollama nor OpenAI is reachable
- **Then:** Returns null

---

### Requirement: Store Utility Function Test Coverage

The system SHALL add unit tests for `sanitizeFTS5Query()` utility function that MUST validate FTS5 query escaping and security.

#### Scenario: Query escaping
- **Given:** sanitizeFTS5Query() is called with simple text
- **When:** Text is "test"
- **Then:** Returns '"test"' (wrapped in quotes)

- **Given:** sanitizeFTS5Query() is called with quotes inside
- **When:** Text contains internal double quotes
- **Then:** Double quotes are escaped ('"' becomes '""')

#### Scenario: Edge cases
- **Given:** sanitizeFTS5Query() is called with whitespace
- **When:** Text is "  query  "
- **Then:** Trims whitespace and returns '"query"'

- **Given:** sanitizeFTS5Query() is called with empty/whitespace-only
- **When:** Text is empty or all spaces
- **Then:** Returns empty string ''

---

### Requirement: Collections Helper Function Test Coverage

The system SHALL add unit tests for helper functions in collections module that MUST validate collection name normalization and config validation.

#### Scenario: Collection name normalization
- **Given:** normalizeCollectionName() is called
- **When:** Name is "My Documents"
- **Then:** Returns "my-documents" (lowercase, space→hyphen)

- **Given:** normalizeCollectionName() is called with special characters
- **When:** Name contains non-alphanumeric chars
- **Then:** Removes special chars, keeps alphanumeric and hyphens

#### Scenario: Collection config validation
- **Given:** validateCollectionConfig() is called
- **When:** Config has required fields (name, path) and path exists
- **Then:** Returns true

- **Given:** validateCollectionConfig() is called with missing fields
- **When:** Config missing required name or path
- **Then:** Throws descriptive error

---

