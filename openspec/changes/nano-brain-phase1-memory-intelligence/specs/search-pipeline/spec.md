## ADDED Requirements

### Requirement: Usage-aware scoring in hybrid pipeline

The `memory_query` hybrid search pipeline SHALL incorporate usage-based scoring as an additional ranking signal. The scoring pipeline order SHALL be: RRF fusion → top-rank bonus → centrality boost → usage boost → supersede demotion → position-aware blend (if reranking enabled).

#### Scenario: Hybrid search with usage boost enabled

- **WHEN** a hybrid search is performed with usage boost enabled
- **THEN** search results reflect access patterns in their ranking
- **THEN** frequently accessed documents rank higher than identical documents with lower access counts

#### Scenario: Hybrid search with usage boost disabled

- **WHEN** a hybrid search is performed with `usage_boost_weight` set to 0
- **THEN** the search results are identical to the current behavior without usage boosting
- **THEN** no usage-based score adjustments are applied

#### Scenario: BM25-only search does not apply usage boost

- **WHEN** a BM25-only search is performed using `memory_search`
- **THEN** no usage boost is applied to the results
- **THEN** only the hybrid pipeline (`memory_query`) incorporates usage-based scoring
