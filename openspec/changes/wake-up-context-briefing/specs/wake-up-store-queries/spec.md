# Spec: wake-up-store-queries

Two new Store interface methods for targeted document retrieval.

## ADDED Requirements

### Requirement: getTopAccessedDocuments method
The Store SHALL provide a method to retrieve documents ordered by access_count descending.

#### Scenario: Retrieve top-accessed documents
- **WHEN** `store.getTopAccessedDocuments(limit, projectHash)` is called
- **THEN** it returns an array of documents ordered by `access_count DESC`
- **AND** each result includes: id, path, collection, title, access_count, last_accessed_at
- **AND** results are limited to `limit` entries

#### Scenario: Project-scoped top documents
- **WHEN** `projectHash` is provided
- **THEN** only documents matching that project_hash or 'global' are returned

#### Scenario: Inactive and superseded exclusion
- **WHEN** called on any workspace
- **THEN** documents with `active = 0` are excluded
- **AND** documents with `superseded_by IS NOT NULL` are excluded

#### Scenario: No matching documents
- **WHEN** no documents match the criteria
- **THEN** an empty array is returned

### Requirement: getRecentDocumentsByTags method
The Store SHALL provide a method to retrieve recent documents filtered by tag names.

#### Scenario: Retrieve recent documents by tags
- **WHEN** `store.getRecentDocumentsByTags(tags, limit, projectHash)` is called
- **THEN** it returns documents that have at least one of the specified tags
- **AND** results are ordered by `modified_at DESC`
- **AND** each result includes: id, path, collection, title, modified_at, tags
- **AND** results are limited to `limit` entries

#### Scenario: Project-scoped tag query
- **WHEN** `projectHash` is provided
- **THEN** only documents matching that project_hash or 'global' are returned

#### Scenario: Inactive and superseded exclusion for tags
- **WHEN** called on any workspace
- **THEN** documents with `active = 0` are excluded
- **AND** documents with `superseded_by IS NOT NULL` are excluded

#### Scenario: No documents with matching tags
- **WHEN** no documents match the criteria
- **THEN** an empty array is returned

### Requirement: Both methods use prepared statements
Both query methods SHALL use SQLite prepared statements with indexed column filters.

#### Scenario: Prepared statement usage
- **WHEN** either method is called
- **THEN** the underlying SQL uses prepared statements (not string interpolation)
- **AND** the queries use indexed columns for filtering (active, superseded_by, project_hash)

### Requirement: Interface contract
Both methods SHALL be declared in the Store interface in types.ts with matching signatures.

#### Scenario: Interface declaration
- **WHEN** the Store interface in types.ts is examined
- **THEN** both `getTopAccessedDocuments` and `getRecentDocumentsByTags` are declared
- **AND** their signatures match the types specified in design.md