## Why

Tracking: #600

Embedding work runs asynchronously from document lifecycle operations. A chunk can be deleted after either embedding path has read it but before it persists the vector, causing PostgreSQL to reject `embeddings.chunk_id` with SQLSTATE 23503 and emit a database error.

## What Changes

- Make the shared embedding insert conditional on the source chunk still existing in the same workspace.
- Treat an absent source chunk as a benign stale-job result in both the background queue and the direct embedding endpoint.
- Add regression coverage for the SQL race and both production write paths.

## Capabilities

### New Capabilities

- `embedding-write-integrity`: Vector writes remain safe when document lifecycle operations delete a chunk during embedding.

### Modified Capabilities

- None.

## Impact

- `internal/storage/queries/embeddings.sql` and regenerated sqlc output.
- `internal/embed/queue.go` and `internal/server/handlers/embed.go` stale-result handling.
- Unit and isolated PostgreSQL integration tests; no migration, provider, or public response-shape change.
