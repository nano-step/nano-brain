## ADDED Requirements

### Requirement: Connections endpoint returns document relationships

The system SHALL expose `GET /api/v1/graph/connections?workspace=<hash>` that returns all document connections with relationship metadata for the specified workspace.

#### Scenario: Successful connections retrieval

- **WHEN** client sends GET request to `/api/v1/graph/connections?workspace=abc123`
- **THEN** server returns 200 with JSON containing `connections` array

#### Scenario: Connection object structure

- **WHEN** response contains connections
- **THEN** each connection object SHALL include: `id`, `fromDoc`, `toDoc`, `relationshipType`, `strength`, `description`, `createdAt`

#### Scenario: Document reference structure

- **WHEN** connection contains fromDoc or toDoc
- **THEN** each document reference SHALL include: `id`, `title`, `path`

#### Scenario: Empty workspace

- **WHEN** workspace has no document connections
- **THEN** server returns 200 with empty array: `{"connections": []}`

#### Scenario: Invalid workspace hash

- **WHEN** workspace parameter is missing or invalid
- **THEN** server returns 400 with error message

#### Scenario: Relationship type values

- **WHEN** response contains connections
- **THEN** relationshipType SHALL be one of: `supports`, `contradicts`, `extends`, `supersedes`, `related`, `caused_by`, `refines`, `implements`

#### Scenario: Strength value range

- **WHEN** response contains connections
- **THEN** strength SHALL be a number between 0.0 and 1.0
