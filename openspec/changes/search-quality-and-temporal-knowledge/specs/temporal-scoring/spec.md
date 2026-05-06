## ADDED Requirements

### Requirement: SearchResult exposes creation date
The `SearchResult` interface SHALL include `createdAt?: string` (ISO 8601) populated from the document's `created_at` column. All three search paths (FTS, vector, hybrid) MUST populate this field.

#### Scenario: Search result includes date
- **WHEN** a hybrid search returns results from the `sessions` collection
- **THEN** each result has a non-null `createdAt` field in ISO 8601 format

### Requirement: Recency boost applied to sessions and memory collections
After RRF fusion, results in `collection IN ('sessions', 'memory')` SHALL receive a recency boost computed as:
`finalScore = score * (1 + recencyWeight * (1 / (1 + daysSince / halfLifeDays)))`
Default config: `recency_weight: 0.3`, `recency_half_life_days: 180`. Codebase collection results MUST NOT receive recency boost.

#### Scenario: Recent session ranks above older session on same topic
- **WHEN** two sessions cover the same topic, one from 30 days ago and one from 400 days ago
- **THEN** the 30-day-old session ranks higher in search results

#### Scenario: Codebase files are not affected by recency
- **WHEN** a search returns both a codebase file and a session file
- **THEN** the codebase file score is not modified by recency boost

### Requirement: Superseded documents are effectively invisible
Documents marked as superseded SHALL have their score multiplied by `0.05` (down from `0.3`). The `supersedeDocument` function MUST store the new document's ID (not `0`) as the superseding reference.

#### Scenario: Superseded doc does not appear in top results
- **WHEN** document A is superseded by document B
- **THEN** document A's score is reduced to ≤5% of its original score
- **THEN** document B appears in results when its topic is queried

#### Scenario: Superseding chain is correctly linked
- **WHEN** `supersedeDocument(oldId, newId)` is called
- **THEN** the old document's `superseded_by` field stores `newId` (not `0`)
