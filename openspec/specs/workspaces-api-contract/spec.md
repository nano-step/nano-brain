# workspaces-api-contract Specification

## Purpose
TBD - created by archiving change fix-workspaces-api-contract. Update Purpose after archive.
## Requirements
### Requirement: Workspaces list response shape
The `GET /api/v1/workspaces` endpoint SHALL return a JSON object with a single top-level key `workspaces` whose value is an array of workspace records. Each workspace record SHALL include the fields specified below with the exact JSON tag names.

#### Scenario: Successful list with workspaces
- **WHEN** `GET /api/v1/workspaces` is called against a server with at least one registered workspace
- **THEN** the response is HTTP 200 with `Content-Type: application/json`
- **AND** the response body parses as a JSON object with key `workspaces` and value of type array
- **AND** each item in the array has the following keys (all required unless noted): `hash` (string), `name` (string), `root_path` (string), `doc_count` (integer), `chunk_count` (integer), `created_at` (RFC3339 string), `updated_at` (RFC3339 string), and optional `last_document_updated` (RFC3339 string or absent)
- **AND** the `hash` field contains the SHA-256-derived workspace identifier (non-empty)

#### Scenario: Empty workspaces
- **WHEN** the server has no registered workspaces
- **THEN** the response is `{"workspaces": []}`
- **AND** is NOT `null`, NOT `[]`, NOT an empty object

#### Scenario: Field naming consistency with frontend
- **WHEN** the response is compared field-by-field with `web/src/api/types.ts:Workspace` interface
- **THEN** every field defined in the TypeScript interface has a matching key with the same name in the JSON response
- **AND** no field is renamed between backend Go struct JSON tags and frontend TypeScript fields

### Requirement: Workspaces chunk count aggregation
Each workspace record SHALL include the total number of chunks across all documents in that workspace, queried at list time.

#### Scenario: Workspace with embedded chunks
- **WHEN** a workspace has 5 documents totaling 42 chunks (regardless of `embed_status`)
- **THEN** the workspace record SHALL include `"chunk_count": 42`

#### Scenario: Empty workspace
- **WHEN** a workspace has no documents (and therefore no chunks)
- **THEN** the workspace record SHALL include `"chunk_count": 0`

### Requirement: Regression test for response shape drift
The handler test SHALL assert the response body structure matches the contract specified in this spec. The test SHALL fail if any field is renamed, omitted, or if the top-level wrapper is changed.

#### Scenario: Test catches wrapping change
- **WHEN** the handler is modified to return raw array instead of wrapped object
- **THEN** `TestListWorkspaces_ResponseShape` fails with assertion error indicating expected `{"workspaces":...}` got `[...]`

#### Scenario: Test catches field rename
- **WHEN** the handler is modified to use JSON tag `workspace_hash` instead of `hash`
- **THEN** `TestListWorkspaces_ResponseShape` fails with assertion indicating missing expected field `hash`

### Requirement: CLI compatibility with new response shape
The CLI commands `nano-brain workspaces list` and `nano-brain workspaces remove` SHALL correctly parse the wrapped response shape and continue to function with the new field names.

#### Scenario: CLI workspaces list with new shape
- **WHEN** `nano-brain workspaces list` is invoked against a server returning the new wrapped shape
- **THEN** the command outputs a table with columns HASH, NAME, PATH, DOCS, LAST UPDATE
- **AND** exit code is 0
- **AND** the HASH column shows values from the `hash` field

#### Scenario: CLI workspaces remove still works
- **WHEN** `nano-brain workspaces remove --workspace=<hash>` is invoked
- **THEN** the command successfully identifies the workspace by hash and proceeds with removal flow

