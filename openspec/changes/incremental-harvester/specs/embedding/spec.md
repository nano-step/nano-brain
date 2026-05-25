## MODIFIED Requirements

### Requirement: Embedding queries exclude session documents
All embedding SQL queries SHALL filter out documents from the `sessions` collection. Session documents are indexed for FTS (full-text search) only and SHALL NOT be sent to external embedding providers.

#### Scenario: getHashesNeedingEmbedding returns only non-session documents
- **WHEN** `getHashesNeedingEmbedding()` is called with or without a projectHash
- **THEN** the result excludes all documents where `collection = 'sessions'`

#### Scenario: getNextHashNeedingEmbedding returns only non-session documents
- **WHEN** `getNextHashNeedingEmbedding()` is called with or without a projectHash
- **THEN** the result excludes all documents where `collection = 'sessions'`

#### Scenario: Session documents remain searchable via FTS
- **WHEN** a session document is indexed in the `sessions` collection
- **THEN** it is searchable via `searchFTS()` but never appears in embedding pending queues
