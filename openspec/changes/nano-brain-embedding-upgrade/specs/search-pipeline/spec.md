# Search Pipeline Specification

## Purpose

Search pipeline providing FTS, vector, and hybrid search with query sanitization, parallel execution, cached embeddings, and populated snippets.

## Requirements

### Requirement: FTS5 query sanitization

The `searchFTS` function SHALL sanitize user queries before passing them to FTS5 `MATCH`. All user-provided query strings MUST be treated as literal search text, never as FTS5 syntax.

#### Scenario: Query containing hyphenated words
- **WHEN** user searches for `nano-brain`
- **THEN** the search treats the entire hyphenated term as a literal phrase, not as `opencode NOT memory`

#### Scenario: Query containing FTS5 column names
- **WHEN** user searches for `memory architecture`
- **THEN** the search treats `memory` as a search term, not as a column reference
- **THEN** no `no such column` error is thrown

#### Scenario: Query containing FTS5 operators
- **WHEN** user searches for `AND OR NOT NEAR`
- **THEN** the search treats these as literal words, not as FTS5 boolean operators

#### Scenario: Query containing double quotes
- **WHEN** user searches for `he said "hello"`
- **THEN** internal double quotes are escaped and the search completes without SQL error

#### Scenario: Empty or whitespace-only query
- **WHEN** user searches for `   ` or empty string
- **THEN** the search returns an empty result set without error

#### Scenario: Normal multi-word query
- **WHEN** user searches for `sqlite vector search`
- **THEN** the search returns documents containing those terms, ranked by BM25 relevance

## MODIFIED Requirements

### Requirement: Vector search returns populated snippets

The `searchVec()` function SHALL JOIN with the `content` table and return snippet text for each match. For per-chunk embeddings, the snippet MUST be extracted from the specific chunk using the seq offset.

#### Scenario: Vector search returns snippet for whole document
- **WHEN** vector search matches a document with no chunk seq (legacy format)
- **THEN** the result includes `substr(body, 1, 700)` as the snippet
- **THEN** the snippet is extracted from `content.body` via JOIN

#### Scenario: Vector search returns snippet for specific chunk
- **WHEN** vector search matches `hash:2` (third chunk of a document)
- **THEN** the snippet is extracted from `content.body` starting at offset (2 * chunk_size)
- **THEN** the snippet length is approximately 700 characters or the chunk size, whichever is smaller

#### Scenario: Vector search with no content table match
- **WHEN** vector search matches a `hash_seq` with no corresponding `content` row
- **THEN** the result includes an empty snippet or placeholder
- **THEN** no SQL error is thrown

### Requirement: Hybrid search executes in parallel

The `hybridSearch()` function SHALL execute FTS and vector search concurrently for all query variants using `Promise.all`, not sequentially.

#### Scenario: Multiple query variants execute concurrently
- **WHEN** `hybridSearch()` is called with 3 query variants
- **THEN** all 3 variants start FTS and vector search concurrently
- **THEN** the total execution time is approximately equal to the slowest variant, not the sum of all variants

#### Scenario: FTS and vector search run in parallel per variant
- **WHEN** a single query variant is processed
- **THEN** `searchFTS()` and `searchVec()` are called concurrently via `Promise.all`
- **THEN** the variant's total time is the maximum of FTS time and vector time, not the sum

#### Scenario: Results aggregated after all variants complete
- **WHEN** all query variants finish executing in parallel
- **THEN** results from all variants are combined using RRF
- **THEN** the final ranked list is returned

### Requirement: Query embeddings are cached

The `searchVec()` function SHALL use the query embedding cache to avoid redundant Ollama calls for identical queries.

#### Scenario: Cache hit for repeated query
- **WHEN** a query `"vector search"` is embedded and cached
- **THEN** a subsequent vector search for `"vector search"` retrieves the cached embedding
- **THEN** no HTTP request is made to Ollama `/api/embed` endpoint

#### Scenario: Cache miss triggers Ollama call
- **WHEN** a query `"new search term"` has no cached embedding
- **THEN** the system calls Ollama `/api/embed` endpoint
- **THEN** the returned embedding is stored in cache with key `computeHash('qembed:new search term')`
- **THEN** the embedding is used for vector search

#### Scenario: Cache invalidated on model change
- **WHEN** the embedding model changes from `nomic-embed-text` to `mxbai-embed-large`
- **THEN** all `qembed:*` cache entries are cleared
- **THEN** subsequent queries trigger fresh embeddings with the new model
