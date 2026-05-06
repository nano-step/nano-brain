## ADDED Requirements

### Requirement: Documents table has domain_type and last_reinforced_at columns
The `documents` SQLite table SHALL have two new columns added via migration: `domain_type TEXT DEFAULT 'general'` and `last_reinforced_at TEXT`. The migration MUST be additive and safe to run on existing data.

#### Scenario: Migration runs without data loss
- **WHEN** the server starts on an existing database without these columns
- **THEN** the migration adds both columns with defaults
- **THEN** all existing documents retain their data
- **THEN** `domain_type` defaults to `'general'` for all existing rows

#### Scenario: New documents can set domain_type
- **WHEN** a document is inserted with `domain_type: 'tech-stack'`
- **THEN** the document is stored with `domain_type = 'tech-stack'`
