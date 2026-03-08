## ADDED Requirements

### Requirement: Access tracking on search results

The system SHALL increment `access_count` and update `last_accessed_at` for every document returned in search results to the user. Internal pipeline queries (expansion, reranking) SHALL NOT trigger access tracking.

#### Scenario: Search returns multiple results

- **WHEN** a search returns 5 results to the user
- **THEN** all 5 documents have their `access_count` incremented by 1
- **THEN** all 5 documents have their `last_accessed_at` updated to the current timestamp

#### Scenario: Same document returned in separate searches

- **WHEN** the same document is returned in two separate user searches
- **THEN** the document's `access_count` is 2
- **THEN** the document's `last_accessed_at` reflects the most recent search timestamp

#### Scenario: Internal pipeline query does not trigger tracking

- **WHEN** a vector-only internal search occurs during the hybrid pipeline
- **THEN** no `access_count` increments occur for those intermediate results
- **THEN** only the final results returned to the user trigger access tracking

### Requirement: Decay score computation

The system SHALL compute a decay score using the formula `1 / (1 + daysSinceAccess / halfLife)` where `daysSinceAccess` is the number of days since `last_accessed_at` and `halfLife` is configurable. Documents with NULL `last_accessed_at` SHALL use `created_at` as fallback.

#### Scenario: Document accessed today

- **WHEN** a document was accessed today (daysSinceAccess = 0)
- **THEN** the decay score is approximately 1.0

#### Scenario: Document not accessed for 30 days with 30-day half-life

- **WHEN** a document has not been accessed for 30 days and `halfLife` is 30 days
- **THEN** the decay score is approximately 0.5

#### Scenario: Document never accessed

- **WHEN** a document has NULL `last_accessed_at`
- **THEN** the system uses `created_at` for the `daysSinceAccess` calculation
- **THEN** the decay score is computed based on the document's age

### Requirement: Schema migration

The system SHALL add `access_count INTEGER DEFAULT 0` and `last_accessed_at TEXT DEFAULT NULL` columns to the `documents` table. Existing documents SHALL retain default values. Migration SHALL be backward compatible (no data loss).

#### Scenario: Fresh database initialization

- **WHEN** a new database is created
- **THEN** the `documents` table includes `access_count` and `last_accessed_at` columns from creation

#### Scenario: Existing database without decay columns

- **WHEN** an existing database does not have `access_count` or `last_accessed_at` columns
- **THEN** an ALTER TABLE migration adds both columns with default values
- **THEN** no existing data is lost

#### Scenario: Existing documents after migration

- **WHEN** the migration completes on a database with existing documents
- **THEN** all existing documents have `access_count` set to 0
- **THEN** all existing documents have `last_accessed_at` set to NULL

### Requirement: Decay configuration

The system SHALL support a `decay` section in config.yml with `enabled` (boolean, default false), `halfLife` (duration string, default "30d"), and `boostWeight` (number 0-1, default 0.15). Invalid values SHALL log a warning and use defaults.

#### Scenario: Decay not configured

- **WHEN** config.yml has no `decay` section
- **THEN** decay is disabled by default
- **THEN** no decay scoring is applied to search results

#### Scenario: Decay enabled with custom half-life

- **WHEN** config.yml contains `decay: { enabled: true, halfLife: "7d" }`
- **THEN** the system uses a 7-day half-life for decay calculations

#### Scenario: Invalid half-life value

- **WHEN** config.yml contains `decay: { halfLife: "banana" }`
- **THEN** a warning is logged indicating the invalid value
- **THEN** the default half-life of 30 days is used
