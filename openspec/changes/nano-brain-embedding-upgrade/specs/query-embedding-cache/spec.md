# Query Embedding Cache Specification

## Purpose

Cache query embeddings in the `llm_cache` table to eliminate redundant Ollama HTTP calls for identical queries.

## ADDED Requirements

### Requirement: Query embedding cache key format

The query embedding cache SHALL use the `llm_cache` table with a `qembed:` prefix. Cache keys MUST be computed as `computeHash('qembed:' + query)`.

#### Scenario: Cache key for simple query
- **WHEN** a query `"test search"` is embedded
- **THEN** the cache key is computed as `computeHash('qembed:test search')`
- **THEN** the key is stored in `llm_cache.hash` column

#### Scenario: Cache key for query with special characters
- **WHEN** a query `"nano-brain: memory"` is embedded
- **THEN** the cache key includes the exact query string with special characters
- **THEN** the cache key is `computeHash('qembed:nano-brain: memory')`

### Requirement: Query embedding cache value format

Cached query embeddings SHALL be stored as JSON-stringified arrays in the `llm_cache.result` column.

#### Scenario: Store embedding vector
- **WHEN** Ollama returns an embedding `[0.123, -0.456, 0.789]` for a query
- **THEN** the value stored in `llm_cache.result` is `JSON.stringify([0.123, -0.456, 0.789])`
- **THEN** the `created_at` column is set to the current Unix timestamp

#### Scenario: Retrieve cached embedding
- **WHEN** a cached embedding is retrieved from `llm_cache`
- **THEN** the `result` column is parsed as `JSON.parse(result)`
- **THEN** the returned value is a numeric array matching the original embedding dimensions

### Requirement: Query embedding cache hit behavior

When a query embedding exists in cache, the system SHALL return the cached embedding without calling Ollama.

#### Scenario: Cache hit for identical query
- **WHEN** a query `"vector search"` is embedded and cached
- **THEN** a subsequent embedding request for `"vector search"` retrieves the cached value
- **THEN** no HTTP request is made to Ollama `/api/embed` endpoint

#### Scenario: Cache miss for new query
- **WHEN** a query `"new search term"` has no cached embedding
- **THEN** the system calls Ollama `/api/embed` endpoint
- **THEN** the returned embedding is stored in cache before returning

### Requirement: Query embedding cache invalidation

The system SHALL clear all `qembed:*` cache entries when the embedding model or dimensions change.

#### Scenario: Model change invalidates cache
- **WHEN** the embedding model changes from `nomic-embed-text` to `mxbai-embed-large`
- **THEN** all rows in `llm_cache` where `hash` starts with the `qembed:` prefix hash pattern are deleted
- **THEN** subsequent queries trigger fresh embeddings with the new model

#### Scenario: Dimension change invalidates cache
- **WHEN** the embedding dimensions change from 768 to 1024
- **THEN** all query embedding cache entries are cleared
- **THEN** cached embeddings from the old dimension count are not returned

### Requirement: Query embedding cache determinism

Query embedding cache entries SHALL have no TTL. Cached embeddings are deterministic and valid until model/dimension changes.

#### Scenario: Cache persists across server restarts
- **WHEN** a query embedding is cached and the server restarts
- **THEN** the cached embedding is still valid and returned on next request
- **THEN** no expiration check is performed on `created_at`

#### Scenario: Identical queries always hit cache
- **WHEN** the same query string is embedded multiple times over days
- **THEN** all requests after the first return the cached embedding
- **THEN** the cache entry is never evicted by time-based TTL
