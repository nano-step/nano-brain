## ADDED Requirements

### Requirement: Init registers code collection at workspace root
When a workspace is initialized via `POST /api/v1/init`, the system SHALL create a collection named `code` pointing to the absolute workspace root path (`absPath`), with glob pattern `**/*` and update_mode `auto`, as part of the same atomic transaction as the workspace upsert.

#### Scenario: Code collection created on first init
- **WHEN** `POST /api/v1/init` is called with a valid `root_path`
- **THEN** a collection row with `name = "code"`, `path = absPath`, `glob_pattern = "**/*"`, `update_mode = "auto"` SHALL exist in the database for the workspace

#### Scenario: Code collection upserted on second init (idempotent)
- **WHEN** `POST /api/v1/init` is called twice with the same `root_path`
- **THEN** exactly one collection named `code` SHALL exist for that workspace (no duplicate rows)

#### Scenario: Init failure rolls back all collections
- **WHEN** the `code` collection upsert fails (e.g., DB error)
- **THEN** the entire transaction (workspace + memory + sessions + code upserts) SHALL be rolled back and the HTTP response SHALL be 500

#### Scenario: Watcher indexes project files after server restart
- **WHEN** `POST /api/v1/init` has been called for a workspace and the server is restarted
- **THEN** the watcher seeding loop SHALL register the `code` collection's path for file watching so project files are indexed
