# Vector Store Interface Specification

## Purpose

Provider-agnostic interface for vector storage operations, enabling nano-brain to swap between sqlite-vec, Qdrant, and future vector databases without changing search or indexing logic.

## ADDED Requirements

### Requirement: VectorStore interface contract

All vector store providers SHALL implement the VectorStore interface with search, upsert, batchUpsert, delete, deleteByHash, health, and close methods.

#### Scenario: Search returns ranked results
- **WHEN** search is called with a 1024-dim embedding and limit=10
- **THEN** up to 10 VectorSearchResult objects are returned
- **THEN** results are sorted by descending cosine similarity score (0-1 normalized)

#### Scenario: Upsert inserts or replaces a vector
- **WHEN** upsert is called with id "abc123:0" and a 1024-dim embedding
- **THEN** the vector is stored with its metadata (hash, seq, pos, model, collection, projectHash)
- **THEN** calling upsert again with the same id replaces the previous vector

#### Scenario: BatchUpsert handles chunked uploads
- **WHEN** batchUpsert is called with 1200 points
- **THEN** points are uploaded in chunks (max 500 per batch for Qdrant, unbounded for SQLite)
- **THEN** all 1200 points are stored after completion

#### Scenario: DeleteByHash removes all chunks for a document
- **WHEN** deleteByHash is called with hash "abc123"
- **THEN** all vectors with that hash are removed (abc123:0, abc123:1, abc123:2, etc.)

#### Scenario: Health returns provider status
- **WHEN** health is called
- **THEN** returns ok=true/false, provider name, vector count, and dimensions

### Requirement: Provider factory creates correct implementation

The createVectorStore factory SHALL return the correct provider based on config.vector.provider value.

#### Scenario: Default provider is sqlite-vec
- **WHEN** config has no vector section or vector.provider is "sqlite-vec"
- **THEN** SqliteVecStore is returned
- **THEN** existing behavior is preserved with zero changes

#### Scenario: Qdrant provider selected
- **WHEN** config has vector.provider set to "qdrant"
- **THEN** QdrantVecStore is returned with url and apiKey from config
- **THEN** collection is created if it does not exist
