## ADDED Requirements

### Requirement: List all flows in a workspace
The system SHALL expose `GET /api/v1/documents?collection=flows&workspace=<hash>` (existing endpoint with pagination) that returns a JSON array of all detected flow documents in a workspace.

#### Scenario: List flows
- **WHEN** a client requests `GET /api/v1/documents?collection=flows&workspace=<hash>`
- **THEN** the response is a JSON array of documents with `id`, `title`, `source_path`, `content`, `metadata`, `tags`, `created_at`, `updated_at`
- **AND** each document's `metadata` contains `{"chain": [...]}` with the flow chain data

#### Scenario: Paginated list flows
- **WHEN** a client requests `GET /api/v1/documents?collection=flows&workspace=<hash>&limit=50`
- **THEN** the response contains at most 50 documents
- **AND** includes `next_cursor` field (last document ID) for fetching the next page

#### Scenario: Empty workspace
- **WHEN** the workspace has no flow documents
- **THEN** the response is an empty array `[]`

#### Scenario: Unknown workspace
- **WHEN** the workspace hash is not registered
- **THEN** the response is a 404 with a clear error message
