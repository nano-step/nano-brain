## ADDED Requirements

### Requirement: reindex marks collection chunks pending
When `POST /api/v1/reindex` is called with a valid collection name, the server SHALL update `embed_status` to `'pending'` for all chunks belonging to documents in that collection and workspace. This ensures the embed queue will re-embed those chunks on its next scan cycle.

#### Scenario: Successful reindex triggers embed reset
- **WHEN** `POST /api/v1/reindex` is called with `{"root": "my-collection"}` and the workspace header is valid
- **THEN** the server SHALL return HTTP 202 with `{"status": "queued", "message": "Reindex queued for collection my-collection in workspace <wsHash>"}` and all chunks for documents in `my-collection` SHALL have `embed_status = 'pending'` in the database

#### Scenario: Missing root field returns 400
- **WHEN** `POST /api/v1/reindex` is called with `{}` (no `root` field) or an empty `root`
- **THEN** the server SHALL return HTTP 400 with an error message and SHALL NOT modify any chunks

#### Scenario: Invalid request body returns 400
- **WHEN** `POST /api/v1/reindex` is called with a malformed JSON body
- **THEN** the server SHALL return HTTP 400 and SHALL NOT modify any chunks

#### Scenario: Collection with no chunks is accepted
- **WHEN** `POST /api/v1/reindex` is called with a valid collection name that has no documents or chunks in the workspace
- **THEN** the server SHALL return HTTP 202 (zero rows updated is not an error)

### Requirement: reindex triggers watcher rescan
When `POST /api/v1/reindex` is called and the named collection is currently watched, the server SHALL trigger the watcher to rescan that collection's directory on its next debounce cycle, so that new or modified files are ingested.

#### Scenario: Watched collection is rescanned
- **WHEN** `POST /api/v1/reindex` is called for a collection that is actively watched by the watcher
- **THEN** the watcher SHALL schedule a rescan of that collection's directory (mark dirty), which will run on the next debounce cycle

#### Scenario: Unwatched collection still returns 202
- **WHEN** `POST /api/v1/reindex` is called for a collection that is not currently in the watcher's watch list (e.g., watcher not running or collection was never watched)
- **THEN** the server SHALL still return HTTP 202, since the embed-status reset succeeded; the watcher rescan is best-effort
