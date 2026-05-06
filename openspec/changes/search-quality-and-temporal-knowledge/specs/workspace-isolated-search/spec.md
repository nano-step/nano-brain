## ADDED Requirements

### Requirement: Qdrant vector search filters by workspace server-side
Every Qdrant vector search call SHALL include a payload filter on `project_hash` matching the active workspace. The system MUST NOT rely on post-filtering to enforce workspace isolation.

#### Scenario: Vector search returns only current workspace results
- **WHEN** a query is issued for workspace `aeedd3af6f8d`
- **THEN** all returned vector results have `project_hash = 'aeedd3af6f8d'`
- **THEN** no results from other workspaces appear in the response

#### Scenario: Qdrant backfill runs on startup when payload is missing
- **WHEN** the server starts and detects Qdrant points without `project_hash` payload
- **THEN** a background backfill job runs non-blocking
- **THEN** all existing points are updated with their `project_hash` from SQLite in batches of 100
- **THEN** normal search operates correctly during backfill

### Requirement: FTS search does not leak global project_hash
FTS SQL queries SHALL filter documents using `project_hash = ?` only. The implicit `'global'` leakage via `IN (?, 'global')` MUST be removed. Memory notes with `project_hash = 'global'` SHALL only appear when `scope = 'all'` is explicitly requested.

#### Scenario: Default search excludes global documents
- **WHEN** a query is issued without `scope: 'all'`
- **THEN** documents with `project_hash = 'global'` do not appear in results

#### Scenario: scope=all search includes all workspaces
- **WHEN** a query is issued with `scope: 'all'`
- **THEN** documents from all workspaces including global are returned
- **THEN** workspace filter is not applied
