## NEW Requirements

### Requirement: VoyageAI reranker provider
`createReranker()` SHALL return a working `Reranker` when `reranker.apiKey` is configured.

#### Scenario: Reranker with valid config
- **WHEN** `reranker.apiKey` is set in config.yml
- **THEN** `createReranker()` returns a `VoyageAIReranker` instance
- **AND** the model defaults to `rerank-2.5-lite` if not specified

#### Scenario: Reranker without apiKey
- **WHEN** `reranker.apiKey` is not set and `embedding.apiKey` is not set
- **THEN** `createReranker()` returns null
- **AND** search falls back to RRF-only (existing behavior)

#### Scenario: Reranker falls back to embedding apiKey
- **WHEN** `reranker.apiKey` is not set but `embedding.apiKey` is set
- **THEN** `createReranker()` uses `embedding.apiKey`

#### Scenario: API failure during rerank
- **WHEN** VoyageAI returns 429 or 5xx during a rerank call
- **THEN** `rerank()` returns a result with empty results array
- **AND** a warning is logged
- **AND** search continues with RRF-only scores

#### Scenario: Token usage tracking
- **WHEN** a rerank call succeeds
- **THEN** `total_tokens` from the response is reported via `onTokenUsage` callback
