# embedding-storage-routing Specification

## ADDED Requirements

### Requirement: Config-driven embedding table selection

The `EmbeddingConfig` struct SHALL include a `table_name` field that determines which PostgreSQL table is used for embedding read and write operations. The field SHALL default to `"embeddings"` when not specified in the config file.

The `table_name` field SHALL be validated at config load time:
- MUST be non-empty (after trimming whitespace)
- MUST match `^[a-z][a-z0-9_]*$` (valid PostgreSQL unquoted identifier)

#### Scenario: Default value with empty config

- **GIVEN** a config file without `embedding.table_name`
- **WHEN** the application loads the config
- **THEN** `cfg.Embedding.TableName` SHALL equal `"embeddings"`

#### Scenario: Custom table name

- **GIVEN** a config file specifying `embedding.table_name: embed_1024`
- **WHEN** the application loads the config
- **THEN** `cfg.Embedding.TableName` SHALL equal `"embed_1024"`

#### Scenario: Invalid table name rejected

- **GIVEN** a config file specifying `embedding.table_name: "Embeddings!!"` (contains uppercase and special characters)
- **WHEN** the application loads the config
- **THEN** validation SHALL return an error

### Requirement: Table-routing Querier wrapper

A `TableRoutingQuerier` struct SHALL implement the existing `Querier` interface (used by the search pipeline, MCP tools, and HTTP handlers) by holding references to two concrete query sets — one for the `embeddings` table (768d) and one for the `embed_1024` table (1024d). Every `VectorSearch*` method call SHALL delegate to the query set matching the configured `table_name`.

The routing wrapper SHALL be constructed once at application startup and injected into all callers. No caller should need to know which table is active.

#### Scenario: Table name "embeddings" delegates to 768d queries

- **GIVEN** a `TableRoutingQuerier` constructed with `cfg.Embedding.TableName = "embeddings"`, a `*Queries` (768d), and a `*Queries1024` (1024d)
- **WHEN** any `VectorSearch*` method is called
- **THEN** the call SHALL delegate to the corresponding `*Queries` method
- **AND** the `*Queries1024` methods SHALL NOT be called

#### Scenario: Table name "embed_1024" delegates to 1024d queries

- **GIVEN** a `TableRoutingQuerier` constructed with `cfg.Embedding.TableName = "embed_1024"`
- **WHEN** any `VectorSearch*` method is called
- **THEN** the call SHALL delegate to the corresponding `*Queries1024` method

#### Scenario: Table-routing wrapper is used by all vector search callers

- **GIVEN** the search pipeline, MCP tools, and HTTP handlers are constructed
- **WHEN** each service receives its `Querier` dependency
- **THEN** the concrete type injected SHALL be `*TableRoutingQuerier` (or equivalent), not `*sqlc.Queries` directly

### Requirement: Embed queue writes to configured table

The embed queue SHALL write embeddings to the table specified by `config.Embedding.TableName`. The queue's `InsertEmbedding` call SHALL delegate to the correct sqlc query set.

#### Scenario: New embedding goes to configured table

- **GIVEN** the embed queue is constructed with `cfg.Embedding.TableName = "embed_1024"`
- **AND** a chunk is processed by the queue
- **WHEN** the queue calls InsertEmbedding
- **THEN** the SQL INSERT SHALL target the `embed_1024` table
- **AND** the `embeddings` table SHALL remain unchanged
