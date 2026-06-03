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

### Requirement: Stable result ordering on tied scores

The search pipeline (`internal/search/rrf.RRFMerge` and `internal/search/recency.ApplyRecencyBoost`) SHALL produce a deterministic result order when multiple results have mathematically identical scores. When two or more results have equal RRF scores or equal recency-boosted scores, ordering between them SHALL be determined by ascending result `id` (UUID string comparison).

Without this guarantee, cursor-based pagination cannot return a stable slice of a result set across paginated calls — equal-score results would reorder between page 1 and page 2 due to Go map iteration order or PostgreSQL `LIMIT` without `ORDER BY` indeterminism.

#### Scenario: RRF fusion breaks ties by id ASC

- **GIVEN** two BM25 candidates A and B that produce identical RRF scores after fusion
- **WHEN** `RRFMerge` is called on a result set containing A and B
- **THEN** the result whose `id` sorts smaller (lexicographic UUID string comparison) appears earlier in the output slice
- **THEN** running `RRFMerge` again on the same input SHALL produce the same output order

#### Scenario: Recency boost preserves stable order on tied boosted scores

- **GIVEN** two results X and Y with identical pre-boost scores AND identical `created_at` timestamps
- **WHEN** `ApplyRecencyBoost` is called on the result set
- **THEN** the result whose `id` sorts smaller appears earlier in the output slice
- **THEN** running `ApplyRecencyBoost` again on the same input SHALL produce the same output order

#### Scenario: Paginated calls return non-overlapping, non-skipping slices

- **GIVEN** a quiescent index with 20 documents matching a query
- **WHEN** the first call returns results 1–10 with stable ordering
- **AND THEN** the second call (with cursor) returns results 11–20
- **THEN** no result appears in both pages
- **THEN** no result from 1–20 is absent from the union of both pages
