## ADDED Requirements

### Requirement: Symbols endpoint returns code symbols with edges and clusters

The system SHALL expose `GET /api/v1/graph/symbols?workspace=<hash>` that returns all code symbols, their edges, and Louvain cluster assignments for the specified workspace.

#### Scenario: Successful symbols retrieval

- **WHEN** client sends GET request to `/api/v1/graph/symbols?workspace=abc123`
- **THEN** server returns 200 with JSON containing `symbols`, `edges`, and `clusters` arrays

#### Scenario: Symbol object structure

- **WHEN** response contains symbols
- **THEN** each symbol object SHALL include: `id`, `name`, `kind`, `filePath`, `startLine`, `endLine`, `exported`, `clusterId`

#### Scenario: Edge object structure

- **WHEN** response contains edges
- **THEN** each edge object SHALL include: `sourceId`, `targetId`, `edgeType`, `confidence`

#### Scenario: Cluster object structure

- **WHEN** response contains clusters
- **THEN** each cluster object SHALL include: `id`, `memberCount`

#### Scenario: Empty workspace

- **WHEN** workspace has no indexed symbols
- **THEN** server returns 200 with empty arrays: `{"symbols": [], "edges": [], "clusters": []}`

#### Scenario: Invalid workspace hash

- **WHEN** workspace parameter is missing or invalid
- **THEN** server returns 400 with error message

#### Scenario: Large dataset performance

- **WHEN** workspace contains 5000+ symbols
- **THEN** response SHALL complete within 2 seconds
