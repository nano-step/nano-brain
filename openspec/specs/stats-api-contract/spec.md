# stats-api-contract Specification

## Purpose
TBD - created by archiving change fix-stats-api-contract. Update Purpose after archive.
## Requirements
### Requirement: Stats response shape matches frontend StatsResponse
The `GET /api/v1/stats?workspace=<hash>` endpoint SHALL return a JSON object whose top-level keys exactly match the `StatsResponse` interface in `web/src/api/types.ts`.

#### Scenario: All required fields present
- **WHEN** `GET /api/v1/stats?workspace=<hash>` is called against a server with a registered workspace
- **THEN** the response is HTTP 200 with `Content-Type: application/json`
- **AND** the response body has these top-level keys: `server_version`, `uptime_sec`, `embedding`, `migration_version`, `docs_total`, `chunks_total`, `chunks_by_embed_status`, `embeddings_total`, `graph_edges_by_type`, `collections`, `tags_top_20`, `harvest`, `watcher`, `recent_docs`
- **AND** no legacy field names (`chunks`, `graph_edges`, `top_tags`, `recent_queries`) are present

#### Scenario: chunks_by_embed_status is object not array
- **WHEN** the response is parsed
- **THEN** `chunks_by_embed_status` is a JSON object (not array)
- **AND** it contains keys `pending`, `embedded`, `embed_failed` with integer values
- **AND** missing statuses default to 0

#### Scenario: graph_edges_by_type is object not array
- **WHEN** the response is parsed
- **THEN** `graph_edges_by_type` is a JSON object (not array)
- **AND** it contains keys like `contains`, `imports`, `calls`, `references` with integer values
- **AND** unknown edge types are included as additional keys

#### Scenario: tags_top_20 uses count field
- **WHEN** the response is parsed
- **THEN** `tags_top_20` is an array of objects with keys `tag` (string) and `count` (integer)
- **AND** there is no `doc_count` field

### Requirement: Server context fields populated
The response SHALL include server-level metadata sourced from the server's runtime context, not from per-workspace queries.

#### Scenario: Version and uptime
- **WHEN** the server has been running for N seconds with version "v1.2.3"
- **THEN** the response includes `server_version: "v1.2.3"` and `uptime_sec` is a non-negative integer

#### Scenario: Embedding info
- **WHEN** the server is configured with provider "ollama", model "nomic-embed-text", dimension 768
- **THEN** the response includes `embedding: {provider: "ollama", model: "nomic-embed-text", dim: 768}`

#### Scenario: Migration version
- **WHEN** the database is at goose migration version 12
- **THEN** the response includes `migration_version: 12`

### Requirement: Aggregate totals populated
The response SHALL include `docs_total`, `chunks_total`, `embeddings_total` as integer counts of all documents, chunks, and embeddings respectively for the given workspace.

#### Scenario: Empty workspace
- **WHEN** the workspace has no documents
- **THEN** `docs_total`, `chunks_total`, `embeddings_total` are all 0

#### Scenario: Populated workspace
- **WHEN** the workspace has 100 documents, 500 chunks, 480 embeddings
- **THEN** `docs_total` is 100, `chunks_total` is 500, `embeddings_total` is 480

### Requirement: Regression test for response shape
A handler test SHALL assert the response body has all 14 expected top-level keys and that nested objects have correct types. The test SHALL fail if any field is renamed, omitted, or returned as an unexpected type (e.g., array instead of object).

#### Scenario: Test catches field rename
- **WHEN** the handler is modified to use JSON tag `top_tags` instead of `tags_top_20`
- **THEN** `TestStats_ResponseShape` fails with assertion indicating missing `tags_top_20`

#### Scenario: Test catches shape change
- **WHEN** the handler returns `chunks_by_embed_status` as array instead of object
- **THEN** `TestStats_ResponseShape` fails with assertion that field is not an object

