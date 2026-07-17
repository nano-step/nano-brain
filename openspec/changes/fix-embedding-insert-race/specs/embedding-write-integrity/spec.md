## ADDED Requirements

### Requirement: Embedding inserts are conditional on a live source chunk

The storage layer SHALL insert or update an embedding only when its source chunk with the requested workspace still exists. The existence check and embedding write SHALL be protected from concurrent chunk deletion within one database statement. When the chunk was already deleted, the storage call SHALL return `sql.ErrNoRows` rather than a foreign-key violation.

#### Scenario: Chunk was deleted before vector persistence

- **GIVEN** an embedding worker or direct endpoint has already generated a vector for chunk `c1`
- **AND** another lifecycle operation deletes `c1` before vector persistence
- **WHEN** the writer persists the vector
- **THEN** no embedding row is written
- **AND** the storage call returns `sql.ErrNoRows`
- **AND** PostgreSQL does not raise a foreign-key violation for `embeddings.chunk_id`

#### Scenario: Chunk deletion starts during vector persistence

- **GIVEN** chunk `c1` exists when the embedding insert begins
- **WHEN** a concurrent lifecycle operation deletes `c1`
- **THEN** the embedding insert completes against the live chunk
- **AND** the deletion completes after the insert and cascades the embedding removal
- **AND** PostgreSQL does not raise a foreign-key violation

### Requirement: Stale embedding results are benign in all production writers

The background queue and direct embedding endpoint SHALL treat `sql.ErrNoRows` from embedding persistence as a stale result, not a database failure. The queue SHALL finish the job without retrying it. The direct endpoint SHALL skip only the stale chunk and continue processing remaining pending chunks.

#### Scenario: Queue persists a vector after its chunk is deleted

- **GIVEN** a queue worker has generated a vector for chunk `c1`
- **WHEN** embedding persistence returns `sql.ErrNoRows`
- **THEN** the queue decrements its pending count and clears retry state for `c1`
- **AND** it does not log an error or retry `c1`

#### Scenario: Direct endpoint encounters a stale chunk in a batch

- **GIVEN** the direct embedding endpoint has pending chunks `c1` and `c2`
- **AND** persistence for `c1` returns `sql.ErrNoRows`
- **WHEN** the endpoint processes the batch
- **THEN** it skips `c1` without an error-level log
- **AND** it continues to embed and persist `c2`
