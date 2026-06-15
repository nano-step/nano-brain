# embed-queue Specification — Delta

## MODIFIED Requirements

### Requirement: Embed queue writes to config-driven table

The `embed.Queue` SHALL write new embeddings to the PostgreSQL table specified by `config.Embedding.TableName` instead of the hardcoded `embeddings` table. The queue SHALL receive a table-aware `EmbeddingQuerier` at construction time that delegates `InsertEmbedding` to the correct sqlc query set based on the config.

The queue's chunk scanning, in-flight dedup, retry logic, and backpressure behavior are unchanged.

#### Scenario: Queue writes to configured table

- **GIVEN** a Queue constructed with a querier that routes to `embed_1024`
- **AND** a chunk is successfully embedded
- **WHEN** the queue calls the insert method
- **THEN** the SQL INSERT SHALL target the `embed_1024` table
- **AND** the insert SHALL use the correct `vector(1024)` type for the embedding

#### Scenario: Queue writes to default table when config unchanged

- **GIVEN** a Queue constructed with a querier that routes to `embeddings` (the default)
- **AND** a chunk is successfully embedded
- **WHEN** the queue calls the insert method
- **THEN** the SQL INSERT SHALL target the `embeddings` table
- **AND** the insert SHALL use the existing `vector(768)` type
