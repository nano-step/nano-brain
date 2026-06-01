# documents-list-endpoint Specification

## Purpose
TBD - created by archiving change add-documents-list-endpoint. Update Purpose after archive.
## Requirements
### Requirement: List documents endpoint
The `GET /api/v1/documents` endpoint SHALL return a JSON object with a single top-level key `documents` whose value is an array of document records for the given workspace.

#### Scenario: List all documents
- **WHEN** `GET /api/v1/documents?workspace=<hash>` is called for a workspace with N documents
- **THEN** the response is HTTP 200 with `Content-Type: application/json`
- **AND** the body parses as `{"documents": [...]}` array of length N
- **AND** each item has these keys: `id`, `title`, `collection`, `tags`, `created_at`, `updated_at`, `supersedes_id`, `superseded_by_id`

#### Scenario: Empty workspace
- **WHEN** the workspace has no documents
- **THEN** the response is `{"documents": []}` (not null, not raw `[]`)

#### Scenario: Filter by collection
- **WHEN** the query includes `&collection=session-summary`
- **THEN** only documents with `collection = "session-summary"` are returned

#### Scenario: Filter by tags
- **WHEN** the query includes `&tags=foo,bar`
- **THEN** only documents with at least one matching tag are returned

#### Scenario: Filter by text
- **WHEN** the query includes `&text=hello`
- **THEN** only documents whose title contains "hello" (case-insensitive) are returned

#### Scenario: Sorted by updated_at DESC
- **WHEN** the response contains multiple documents
- **THEN** they are ordered by `updated_at` descending (most recent first)

### Requirement: Delete document endpoint
The `DELETE /api/v1/documents/:id` endpoint SHALL permanently remove a single document and its associated chunks + embeddings within the workspace.

#### Scenario: Successful delete
- **WHEN** `DELETE /api/v1/documents/<id>` is called with a valid id matching a document in the workspace
- **THEN** the response is HTTP 200 with `{"deleted_id": "<id>"}`
- **AND** the document is removed from `documents` table
- **AND** related chunks (via FK CASCADE) are removed
- **AND** related embeddings (via FK CASCADE) are removed

#### Scenario: Not found
- **WHEN** the id does not match any document in the workspace
- **THEN** the response is HTTP 404

