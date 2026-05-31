# workspace-registration-guard Delta — New Capability

## ADDED Requirements

### Requirement: Persister rejects unregistered workspace_hash

The `Persister.Save()` method in `internal/summarize/persist.go` SHALL verify that `meta.WorkspaceHash` corresponds to a row in the `workspaces` table before persisting any document or chunk. If the workspace_hash is not registered, `Save()` SHALL return a non-nil error wrapping the string `workspace_not_registered` and SHALL NOT perform any database writes (no upsert of document, no upsert of chunks, no enqueue of embedding jobs).

#### Scenario: Save rejects unregistered hash

- **WHEN** `Persister.Save()` is called with `meta.WorkspaceHash = "not-a-registered-hash-xyz"` which has no matching row in `workspaces`
- **THEN** Save returns an error whose message contains `workspace_not_registered`
- **AND** the error includes the rejected workspace_hash for diagnostic purposes
- **AND** the `documents` table has no new rows after the call
- **AND** the `chunks` table has no new rows after the call

#### Scenario: Save proceeds for registered hash

- **WHEN** `Persister.Save()` is called with `meta.WorkspaceHash` matching a row in `workspaces`
- **THEN** the existing idempotency and upsert logic runs as before
- **AND** the document and chunks are persisted

#### Scenario: Save propagates DB errors during lookup

- **WHEN** `Persister.Save()` is called and the workspace registration lookup fails due to a database connection error (NOT `sql.ErrNoRows`)
- **THEN** Save returns an error wrapping the underlying DB error
- **AND** no DB writes are performed

### Requirement: HTTP middleware enforces workspace registration on write endpoints

A new middleware `workspaceRegisteredMiddleware(q WorkspaceQuerier)` SHALL be applied to the write endpoints `POST /api/v1/summarize`, `POST /api/v1/write`, `POST /api/v1/embed`, `POST /api/v1/reindex`, and `POST /api/v1/update`. The middleware SHALL:

1. Reject requests where the workspace string is the literal `"all"` with HTTP 400 and error code `workspace_all_not_supported`.
2. Reject requests where the workspace string is not present in the `workspaces` table with HTTP 400 and error code `workspace_not_registered`.
3. Return HTTP 500 with error code `workspace_lookup_failed` on DB lookup errors.
4. Pass through to the next handler on successful registration lookup.

Read endpoints (`/query`, `/search`, `/vsearch`, `/get`, `/multi-get`, `/wake-up`, `/workspaces`, `/tags`, `/collections`) and management endpoints (`/init`, `DELETE /workspaces/:hash`) SHALL NOT have this middleware applied (they continue to use only the existing `workspaceMiddleware`).

#### Scenario: Write endpoint accepts registered workspace

- **WHEN** a client sends `POST /api/v1/summarize` with `{"workspace":"<registered-hash>","source":"opencode","limit":1}`
- **THEN** the middleware looks up the hash in `workspaces` and finds it
- **AND** the request proceeds to the handler

#### Scenario: Write endpoint rejects unregistered workspace

- **WHEN** a client sends `POST /api/v1/summarize` with `{"workspace":"unregistered-xyz","source":"opencode","limit":1}`
- **THEN** the middleware returns HTTP 400
- **AND** the response body is `{"error":"workspace_not_registered","message":"workspace_hash \"unregistered-xyz\" is not registered; use POST /api/v1/init to register it first"}`
- **AND** the handler is NOT invoked

#### Scenario: Write endpoint rejects "all" scope

- **WHEN** a client sends `POST /api/v1/write` with `{"workspace":"all","source_path":"x","content":"y"}`
- **THEN** the middleware returns HTTP 400
- **AND** the response body is `{"error":"workspace_all_not_supported","message":"this endpoint does not support the 'all' workspace scope; provide a specific registered workspace hash"}`

#### Scenario: Read endpoint still accepts "all"

- **WHEN** a client sends `POST /api/v1/query` with `{"workspace":"all","query":"test"}`
- **THEN** the request proceeds (the read endpoint's middleware does NOT include `workspaceRegisteredMiddleware`)
- **AND** the query runs across all workspaces as documented

### Requirement: MCP tool handlers enforce workspace registration

The MCP tool handlers `memory_write` and `memory_update` in `internal/mcp/tools.go` SHALL verify that the `workspace` argument corresponds to a row in the `workspaces` table before invoking `UpsertDocument`, `UpsertChunk`, or any reindex queuing logic. The MCP transport (`/mcp`, `/sse` endpoints) does NOT pass through Echo HTTP middleware, so the `workspaceRegisteredMiddleware` does not protect MCP writes. This requirement closes that gap.

The handlers SHALL:
1. Reject `workspace == ""` with `mcp.NewToolResultError` containing `workspace_required`.
2. Reject `workspace == "all"` with `mcp.NewToolResultError` containing `workspace_all_not_supported`.
3. Reject workspaces not in the `workspaces` table with `mcp.NewToolResultError` containing `workspace_not_registered`.
4. Return `workspace_lookup_failed` on DB lookup errors.

#### Scenario: memory_write accepts registered workspace

- **WHEN** an MCP client invokes `memory_write` with `{"workspace": "<registered-hash>", "source_path": "x", "content": "y"}`
- **THEN** the handler proceeds to call `UpsertDocument`
- **AND** the document is persisted

#### Scenario: memory_write rejects unregistered workspace

- **WHEN** an MCP client invokes `memory_write` with `{"workspace": "unregistered-xyz", "source_path": "x", "content": "y"}`
- **THEN** the handler returns a tool result error containing the string `workspace_not_registered`
- **AND** `UpsertDocument` is NOT called
- **AND** no document row is created

#### Scenario: memory_write rejects "all" workspace

- **WHEN** an MCP client invokes `memory_write` with `{"workspace": "all", ...}`
- **THEN** the handler returns a tool result error containing the string `workspace_all_not_supported`
- **AND** `UpsertDocument` is NOT called

#### Scenario: memory_update rejects unregistered workspace

- **WHEN** an MCP client invokes `memory_update` with `{"workspace": "unregistered-xyz", ...}`
- **THEN** the handler returns a tool result error containing the string `workspace_not_registered`
- **AND** no reindex job is queued

### Requirement: DB enforces FK constraint on documents and chunks workspace_hash

The `documents` and `chunks` tables SHALL have foreign key constraints on `workspace_hash` referencing `workspaces(hash)` with `ON DELETE CASCADE`. The constraint SHALL be added via migration `00011_add_fk_documents_workspace.sql`. Attempts to insert a row into `documents` or `chunks` with a `workspace_hash` not present in `workspaces` SHALL be rejected by PostgreSQL with a foreign-key-violation error (SQLSTATE 23503).

#### Scenario: FK constraint rejects orphan document insert

- **GIVEN** migration 00011 has been applied
- **WHEN** any client (application code OR direct SQL) attempts `INSERT INTO documents (workspace_hash, ...) VALUES ('not-registered-xyz', ...)`
- **THEN** the insert fails with PostgreSQL error 23503 (foreign_key_violation)
- **AND** no row is created

#### Scenario: FK constraint cascades on workspace deletion

- **GIVEN** migration 00011 has been applied
- **AND** workspace `W` has documents and chunks
- **WHEN** the row for `W` is deleted from the `workspaces` table
- **THEN** all documents with `workspace_hash = W` are deleted automatically by PostgreSQL
- **AND** all chunks with `workspace_hash = W` are deleted automatically by PostgreSQL

#### Scenario: Migration 00011 fails cleanly when orphans exist

- **GIVEN** the database has at least one row in `documents` or `chunks` whose `workspace_hash` is not in `workspaces`
- **WHEN** the operator runs `nano-brain db:migrate` without first running cleanup
- **THEN** migration 00011 fails with a PostgreSQL foreign-key-violation error (SQLSTATE 23503)
- **AND** the error identifies the violating constraint name (`fk_documents_workspace` or `fk_chunks_workspace`) and at least one violating key
- **AND** the migration is rolled back; no constraint is partially applied (both ALTER TABLE statements run in a single transaction)

#### Scenario: FK constraint rejects orphan UPDATE

- **GIVEN** migration 00011 has been applied
- **AND** a document row exists for registered workspace W1
- **WHEN** application code OR direct SQL attempts `UPDATE documents SET workspace_hash = 'not-registered-xyz' WHERE id = '<existing-id>'`
- **THEN** the UPDATE fails with PostgreSQL error 23503 (foreign_key_violation)
- **AND** the document row remains unchanged with workspace_hash = W1

### Requirement: Cleanup command removes orphan documents and chunks

The CLI SHALL expose a `cleanup-orphan-workspaces` command that identifies and removes documents and chunks whose `workspace_hash` is not present in the `workspaces` table. The command SHALL support a `--dry-run` flag that lists what would be deleted without making changes.

#### Scenario: Dry-run reports orphans without changes

- **GIVEN** the DB has 42 documents under unregistered workspace `W1` and 17 under `W2`
- **WHEN** the operator runs `nano-brain cleanup-orphan-workspaces --dry-run`
- **THEN** the command prints `Found 59 documents under 2 unregistered workspace_hash values:` followed by per-hash counts
- **AND** the command prints `Run without --dry-run to apply.`
- **AND** no rows are deleted

#### Scenario: Apply removes orphans

- **GIVEN** the DB has 59 orphan documents and 142 orphan chunks
- **WHEN** the operator runs `nano-brain cleanup-orphan-workspaces` (no flag)
- **THEN** the command prints `Deleted 59 documents + 142 chunks across 2 unregistered workspaces.`
- **AND** the orphan rows are removed
- **AND** registered workspaces and their documents are unchanged

#### Scenario: No orphans → no-op

- **WHEN** the operator runs `nano-brain cleanup-orphan-workspaces` against a DB with no orphans
- **THEN** the command prints `No orphan documents found. DB is clean.`
- **AND** the exit code is 0
- **AND** no rows are modified
