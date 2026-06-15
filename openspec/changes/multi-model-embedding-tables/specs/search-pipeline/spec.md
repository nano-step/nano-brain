# search-pipeline Specification — Delta

## MODIFIED Requirements

### Requirement: Vector search queries config-driven embedding table

The vector search queries (`VectorSearch`, `VectorSearchAll`, `VectorSearchWithTags`, `VectorSearchAllWithTags`) SHALL target the PostgreSQL table specified by `config.Embedding.TableName`. The search pipeline's `Querier` dependency SHALL be a table-routing wrapper that delegates to the correct sqlc query set based on the active config.

All existing search pipeline behavior — BM25 search, RRF fusion, recency decay, tag filtering, time-range filtering — is unchanged.

#### Scenario: Vector search queries embed_1024 when configured

- **GIVEN** the search pipeline is constructed with `cfg.Embedding.TableName = "embed_1024"`
- **AND** the `embed_1024` table contains embeddings for chunks matching a query
- **WHEN** `VectorSearch` or any related vector search method is called
- **THEN** the SQL query SHALL target `FROM embed_1024`
- **AND** results SHALL be returned with scores computed using the 1024d embeddings

#### Scenario: Vector search queries embeddings table by default

- **GIVEN** the search pipeline is constructed with `cfg.Embedding.TableName = "embeddings"` (default)
- **WHEN** any vector search method is called
- **THEN** the SQL query SHALL target `FROM embeddings`
- **AND** behavior SHALL be identical to the pre-change implementation
