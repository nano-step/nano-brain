## ADDED Requirements

### Requirement: Result cache integration in search pipeline
The search pipeline SHALL integrate with the result cache when compact mode is requested. After executing a search, the pipeline SHALL store the full results in the cache and return both the results and the cache key.

#### Scenario: Hybrid search stores results in cache
- **WHEN** `hybridSearch` is called and the caller requests compact output
- **THEN** the full `SearchResult[]` is stored in the in-memory cache
- **THEN** the cache key is returned alongside the results

#### Scenario: Search pipeline without compact mode skips caching
- **WHEN** `hybridSearch` is called and the caller does not request compact output
- **THEN** no cache entry is created
- **THEN** results are returned as before (no cache key)

### Requirement: Compact formatting function
The search pipeline SHALL export a `formatCompactResults` function that renders results in the compact single-line format. This function SHALL be separate from the existing `formatSearchResults` function.

#### Scenario: formatCompactResults produces single-line output
- **WHEN** `formatCompactResults` is called with a `SearchResult[]` and a `cacheKey`
- **THEN** the output starts with the cache key header line
- **THEN** each result is rendered as a single line with index, score, title, docid, path:line, optional symbols, and truncated first line of snippet

#### Scenario: formatSearchResults remains unchanged
- **WHEN** `formatSearchResults` is called with a `SearchResult[]`
- **THEN** the output is identical to the current verbose format (full snippets, markdown headers)
