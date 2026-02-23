## ADDED Requirements

### Requirement: Integration test infrastructure
The project SHALL have an integration test file that exercises MCP tool handlers against a real SQLite database with real FTS5 indexes and real sqlite-vec tables.

#### Scenario: Test setup creates real database with indexed documents
- **WHEN** the integration test suite starts
- **THEN** a temporary SQLite database is created with sqlite-vec loaded
- **THEN** at least 2 test documents are indexed with FTS5 entries
- **THEN** the MCP server's tool handlers are initialized with the real store

#### Scenario: Test teardown cleans up
- **WHEN** the integration test suite completes
- **THEN** the temporary database file is deleted
- **THEN** no test artifacts remain on disk

### Requirement: Search integration tests
Integration tests SHALL verify that `memory_search` works end-to-end with real FTS5 queries.

#### Scenario: Search finds indexed document
- **WHEN** `memory_search` handler is called with a query matching an indexed document
- **THEN** the response contains the matching document with title, path, and snippet

#### Scenario: Search with hyphenated query
- **WHEN** `memory_search` handler is called with query `opencode-memory`
- **THEN** the response completes without error
- **THEN** results include documents containing the term

#### Scenario: Search with collection filter
- **WHEN** `memory_search` handler is called with a collection filter
- **THEN** only documents from that collection are returned

#### Scenario: Search with empty query
- **WHEN** `memory_search` handler is called with an empty string query
- **THEN** the response returns empty results without error

### Requirement: Update integration tests
Integration tests SHALL verify that `memory_update` works end-to-end.

#### Scenario: Update indexes new files
- **WHEN** a new markdown file is added to a collection directory
- **THEN** calling the `memory_update` handler indexes the new file
- **THEN** the file is searchable via `memory_search`

### Requirement: Status integration tests
Integration tests SHALL verify that `memory_status` returns accurate information.

#### Scenario: Status reflects indexed documents
- **WHEN** documents have been indexed
- **THEN** `memory_status` handler returns correct document count and collection info
