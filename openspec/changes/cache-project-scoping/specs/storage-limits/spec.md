## MODIFIED Requirements

### Requirement: Cache table schema supports project scoping and type discrimination
The `llm_cache` table SHALL have columns `hash TEXT`, `project_hash TEXT NOT NULL DEFAULT 'global'`, `type TEXT NOT NULL DEFAULT 'general'`, `result TEXT NOT NULL`, and `created_at TEXT`. The primary key SHALL be `(hash, project_hash)`.

#### Scenario: New database creation
- **WHEN** a new database is created
- **THEN** the `llm_cache` table is created with columns: `hash`, `project_hash`, `type`, `result`, `created_at`
- **THEN** the primary key is `(hash, project_hash)`

#### Scenario: Migration from old schema
- **WHEN** the store opens a database with the old `llm_cache` schema (only `hash TEXT PRIMARY KEY`)
- **THEN** `project_hash TEXT NOT NULL DEFAULT 'global'` column is added
- **THEN** `type TEXT NOT NULL DEFAULT 'general'` column is added
- **THEN** existing rows get `project_hash = 'global'` and `type = 'general'`
- **THEN** the table is rebuilt with composite primary key `(hash, project_hash)`

#### Scenario: Subsequent startup after migration
- **WHEN** the store opens a database that already has `project_hash` and `type` columns
- **THEN** no migration runs
