---
epic: 2
title: "Ingestion Pipeline"
status: ready
depends_on: Epic 1 (Foundation & Data Layer)
created: 2026-05-23
---

# Epic 2: Ingestion Pipeline — User Stories

## Overview

These stories deliver workspace registration, document ingestion via API and CLI, markdown-aware chunking, file watching with debounced reindexing, and collection management. All stories depend on Epic 1 being complete (PostgreSQL running, Echo server up, config loaded, migrations applied).

**FRs covered:** FR-16, FR-17, FR-18, FR-19, FR-22, FR-23, FR-24, FR-25, FR-26, FR-27, FR-28, FR-48, FR-49, FR-50, FR-51, FR-52, FR-53, FR-59, FR-68, FR-69, FR-70, FR-76, FR-77, FR-78, FR-83, FR-84

**ARs applied:** AR-5 (errgroup for watcher goroutine), AR-8 (sqlc for all DB access), AR-12 (fsnotify v1), AR-14 (user-defined collections), AR-17 (naming conventions)

**NFRs enforced:**
- NFR-2: workspace_hash in every SQL WHERE clause on document/chunk tables. Missing workspace returns HTTP 400. No default fallback.
- NFR-4: All inserts use ON CONFLICT upsert semantics. Re-running ingestion is idempotent.

---

## Story Sequence

```
Story 2.1 (Workspace Registration)
  └─ Story 2.2 (API Middleware: workspace validation, version header, content-type)
       └─ Story 2.3 (POST /api/write + document storage via sqlc)
            └─ Story 2.4 (Markdown-Aware Chunker)
                 └─ Story 2.5 (Chunker integration into write path)
                      └─ Story 2.6 (File Watcher with debounced reindex)
                           └─ Story 2.7 (Collection Management CLI + watcher live-attach)
                                └─ Story 2.8 (CLI init, write, query stubs + env/json flags)
```

---

#### Story 2.1: Workspace Registration

**Description:** `nano-brain init --root <dir>` registers a workspace by deriving a SHA-256 hash from the canonical directory path, inserting a row into the `workspaces` table, creating the two default collections (`memory`, `sessions`) in config and database, and printing an AGENTS.md snippet. `GET /api/v1/workspaces` lists all registered workspaces with document counts and timestamps.

**Covers:** FR-16, FR-17, FR-18, FR-19, FR-68, FR-76

**Applies:** AR-8 (sqlc), AR-17 (naming)

**Complexity:** M

**Acceptance Criteria:**

- Given a directory `/path/to/project` that has not been registered, when `nano-brain init --root /path/to/project` runs, then a row is inserted into `workspaces` with `workspace_hash = sha256("/path/to/project")`, `root_path`, `created_at`, and `updated_at`.
- Given `nano-brain init --root /path/to/project` has succeeded, when it is run a second time on the same path, then no duplicate row is created (upsert semantics, NFR-4).
- Given a registered workspace, when `GET /api/v1/workspaces` is called, then the response includes `[{workspace_hash, root_path, document_count, last_updated_at}]` for every registered workspace with accurate counts.
- Given no workspace parameter in the request, when any data endpoint is called (e.g., `GET /api/v1/workspaces` variant that requires workspace), then HTTP 400 is returned with `{"error": "workspace_required", "message": "..."}` (NFR-2).
- Given a registered workspace, when a document is inserted via any write path, then `workspace_hash` is stored as a non-nullable column on every document and chunk row (FR-16).
- Given two workspaces A and B, when any read query runs for workspace A, then the SQL includes `WHERE workspace_hash = $1` bound to A's hash, never returning B's data (FR-17).

**Test expectations:**

- Unit: hash derivation function returns stable SHA-256 for the same path across calls.
- Integration (real PG, `//go:build integration`): `init` inserts workspace row; second `init` on same path is idempotent; `GET /api/v1/workspaces` returns correct count after adding documents.

---

#### Story 2.2: API Middleware Layer

**Description:** Echo middleware chain handles the three cross-cutting API concerns for all data endpoints: workspace extraction and validation (HTTP 400 on missing), `X-Nano-Brain-Version` header injection on every response, and content-type enforcement (HTTP 415 on non-JSON body to JSON endpoints).

**Covers:** FR-68, FR-69, FR-70

**Applies:** AR-3 (Echo v4), AR-17

**Complexity:** S

**Acceptance Criteria:**

- Given a `POST /api/write` request with a JSON body but no `workspace` field, when the request arrives at the handler, then the middleware returns HTTP 400 with `{"error": "workspace_required", "message": "A workspace identifier is required. Pass workspace in request body (POST) or query string (GET). Use 'all' for cross-workspace queries."}` before any handler logic runs.
- Given any successful API response, when the client inspects headers, then `X-Nano-Brain-Version: <semver>` is present (FR-69).
- Given a `POST /api/write` request with `Content-Type: text/plain`, when the request is processed, then HTTP 415 is returned with a descriptive error (FR-70).
- Given a `GET /api/v1/workspaces` request (no body), when it is processed, then the content-type middleware does not reject it (GET requests without body are exempt from 415 check).
- Given middleware is mounted in Echo's middleware chain, when `golangci-lint` runs, then no lint errors are reported on `internal/server/middleware.go`.

**Test expectations:**

- Unit: middleware functions tested in isolation using Echo's `httptest` utilities.
- Verify each middleware returns the correct status code and does not call `c.Next()` on rejection.

---

#### Story 2.3: Document Write Endpoint (POST /api/write)

**Description:** `POST /api/write` accepts `{content, tags?, collection?, workspace}` and inserts a document into the `documents` table inside a single PostgreSQL transaction. The document is content-addressed by SHA-256 of its content. The response returns `{id, hash, collection, workspace_hash}`. Storage limits are enforced before insert.

**Covers:** FR-28, FR-42, FR-43, FR-59, FR-68

**Applies:** AR-8 (sqlc), AR-17

**Complexity:** M

**Acceptance Criteria:**

- Given a valid `POST /api/write` request with `content`, `workspace`, and optional `tags`, when it is processed, then a row is inserted into `documents` with `id` (UUID), `content_hash` (SHA-256 of content), `workspace_hash`, `collection` (defaults to `memory` if omitted), `tags`, `created_at`, `updated_at`.
- Given the same content posted twice to the same workspace, when the second request arrives, then the upsert returns the existing `id` without error and without a duplicate row (NFR-4, FR-43).
- Given content exceeding `storage.max_file_size` (300 KB), when `POST /api/write` is called, then HTTP 400 is returned with a descriptive message before any DB insert (FR-28).
- Given a document insert that fails mid-transaction (simulated), when the failure is injected, then no partial document or chunk rows are visible to subsequent reads (FR-42).
- Given a successful write, when `POST /api/search` is called immediately after with the same workspace and a matching keyword, then the new document appears in BM25 results (BM25 does not require embeddings).
- Given a write request without a `workspace` field, when the middleware chain processes it, then HTTP 400 is returned before the handler executes (FR-68).

**Test expectations:**

- Unit: storage layer `InsertDocument` function with mock pgxpool.
- Integration (real PG): write + immediate BM25 search round-trip; duplicate write idempotency; transaction rollback on simulated failure.

---

#### Story 2.4: Markdown-Aware Chunker

**Description:** The `internal/chunk` package implements the heading-aware chunking algorithm: 3600-char target, 200-char overlap, scored break points (H1 > H2 > H3/code-fence > H4-H6 > hr > blank > list > newline), code-fence awareness, short-chunk merging, sequence/line position tracking, and determinism.

**Covers:** FR-48, FR-49, FR-50, FR-51, FR-52, FR-53

**Applies:** AR-17 (naming)

**Complexity:** L

**Acceptance Criteria:**

- Given a 10,000-character markdown document, when `Chunk(content)` is called, then it returns multiple chunks, each between 200 and ~4000 characters (overlap-adjusted upper bound), and no chunk is shorter than 200 characters (FR-48, FR-51).
- Given a document containing a fenced code block (` ``` `...` ``` `), when chunked, then no chunk boundary falls inside an open code fence; cuts prefer the fence delimiter line (FR-50).
- Given a document with H1, H2, and blank-line boundaries near a 3600-char window boundary, when chunked, then the break is placed at the H1 or H2 boundary (highest score in the 800-char search window), not at the blank line (FR-49).
- Given any document, when `Chunk` is called twice on the same input, then it returns identical chunks in identical order (FR-53).
- Given a chunk's metadata, when inspected, then it includes `Sequence` (0-indexed position), `StartLine`, and `EndLine` corresponding to the document line range (FR-52).
- Given consecutive chunks, when their content is inspected, then they share a ~200-character overlap region at the boundary (FR-48).
- Given a document with only 150 characters of content, when chunked, then a single chunk is returned containing all content (short-chunk merge to adjacent, FR-51).

**Test expectations:**

- Unit (no PG needed): table-driven tests in `internal/chunk/chunker_test.go` covering:
  - Break-point priority ordering (H1 wins over blank line)
  - Code fence non-cutting invariant
  - Determinism: same input, two calls, byte-identical output
  - Minimum chunk length: no chunk shorter than 200 chars in output
  - Position tracking: `StartLine`/`EndLine` correct for multiline documents
- Fuzz test: random UTF-8 input never panics and always returns at least one chunk.

---

#### Story 2.5: Chunker Integration into Write Path

**Description:** Wire the `internal/chunk` package into `POST /api/write`. After the document row is inserted, the document content is chunked and each chunk is inserted into the `chunks` table within the same transaction. Chunk rows carry `workspace_hash`, `document_id`, `content_hash`, `content`, `sequence`, `start_line`, `end_line`. Content-addressed upsert prevents duplicate chunk storage.

**Covers:** FR-16, FR-17, FR-43, FR-48 through FR-53 (applied via chunker), FR-59

**Applies:** AR-8 (sqlc), AR-17

**Complexity:** M

**Acceptance Criteria:**

- Given a `POST /api/write` with a 5000-character markdown document, when the request succeeds, then multiple rows exist in the `chunks` table, all with the same `document_id` and `workspace_hash` as the parent document (NFR-2, FR-16).
- Given a chunk insert transaction, when the transaction commits, then chunk rows are visible immediately (FR-42).
- Given the same document written twice (idempotent upsert), when the second write completes, then the `chunks` table has the same number of chunk rows as after the first write, not double (FR-43, NFR-4).
- Given a document belonging to workspace A, when `SELECT * FROM chunks WHERE workspace_hash = $b_hash` is executed for workspace B, then the result set is empty (NFR-2, FR-17).
- Given a chunk row, when its fields are inspected, then `sequence`, `start_line`, and `end_line` match the values produced by `Chunk()` for that document (FR-52).
- Given a transaction failure after document insert but before all chunks are committed, when the failure is simulated, then no partial chunk rows are visible (FR-42).

**Test expectations:**

- Integration (real PG): write a multi-section markdown document, assert correct chunk count and workspace_hash on all chunk rows; write same document twice, assert no duplicate chunks.

---

#### Story 2.6: File Watcher with Debounced Reindex

**Description:** `internal/watcher` uses fsnotify to monitor all collection directories for a workspace. On a file event, a dirty flag is set for the affected collection. A debounce timer (default 2000 ms) coalesces rapid changes into a single reindex. A periodic full-scan poll (default 300 s) catches missed events. The watcher runs as a goroutine managed by errgroup and never blocks the HTTP API.

**Covers:** FR-22, FR-23, FR-24, FR-25, FR-26, FR-28

**Applies:** AR-5 (errgroup + context), AR-12 (fsnotify v1), AR-17

**Complexity:** L

**Acceptance Criteria:**

- Given the watcher goroutine is started via `errgroup.Go`, when the server starts, then the HTTP API is responsive immediately (the watcher does not block startup, FR-26).
- Given a watched collection directory, when a new `.md` file is created in it, then the file is chunked and its chunks are inserted into the database within `debounce_ms + reindex_processing_time` (FR-22, FR-23).
- Given 10 rapid writes to files in the same collection within 500 ms, when the debounce timer fires, then reindex runs once, not 10 times (FR-23).
- Given a file already indexed with hash H, when the same file is written with identical content (hash still H), then no new chunks are inserted and no existing chunks are modified (FR-24, NFR-4).
- Given a file with hash H is modified (hash becomes H'), when reindex runs, then old chunks for that file are replaced by new chunks derived from H' (FR-24).
- Given `watcher.reindex_interval` seconds have elapsed since the last event-based reindex, when the poll tick fires, then all collection directories are scanned for files whose hash does not match the stored hash (FR-25).
- Given a file exceeding `storage.max_file_size` (300 KB) in a watched directory, when reindex runs, then the file is skipped and a structured log warning is emitted (FR-28).
- Given `context.WithCancel` cancel is called on the root context, when the cancel propagates, then the watcher goroutine drains its current work and exits cleanly (AR-5).

**Test expectations:**

- Unit: debounce logic in isolation (timer reset on rapid events, single fire after quiet period).
- Integration (real PG): create a temp directory as a collection, write a file, assert chunk rows appear; overwrite same file, assert chunk rows updated; write identical content again, assert no duplicate chunks.
- Race test: `go test -race` must pass with concurrent file writes and watcher running.

---

#### Story 2.7: Collection Management (CLI + Live Watcher Attach)

**Description:** `nano-brain collection add/remove/list/rename` manages collections for the current workspace. Collections are stored in both the database (metadata) and config YAML. When a collection is added, the watcher begins monitoring its directory immediately without restart. Removing a collection stops the watch. Renaming changes the name without moving data.

**Covers:** FR-20 (deferred to Epic 8 for full CLI), FR-22, FR-27

**Applies:** AR-12 (fsnotify), AR-14 (user-defined collections), AR-17

**Complexity:** M

**Acceptance Criteria:**

- Given the server is running, when `nano-brain collection add --path /notes --name docs --workspace <hash>` is called, then a row is inserted into the `collections` table with `name=docs`, `path=/notes`, `workspace_hash=<hash>`, and the watcher begins monitoring `/notes` without restart (FR-27).
- Given a collection was just added, when a new file is created in its directory, then the file is indexed within `debounce_ms + processing_time` (FR-22 applied to new collection).
- Given a collection is added with a path that does not exist, when the command runs, then an error is returned and no collection row is inserted.
- Given an existing collection `docs`, when `nano-brain collection rename docs documentation` is called, then the `name` column is updated in the database and the watcher continues monitoring the same path (FR-27).
- Given an existing collection `docs`, when `nano-brain collection remove docs` is called, then the database row is removed, the fsnotify watch for that path is unregistered, and subsequent file changes in `/notes` are not reindexed.
- Given `nano-brain collection list --workspace <hash>`, when called, then all collections for that workspace are printed with their name, path, document count, and last-updated timestamp (FR-27).
- Given all collection commands, when called without `--workspace`, then HTTP 400 is returned (NFR-2).

**Test expectations:**

- Integration (real PG): add collection, write file to its path, assert chunk rows; rename collection, assert name updated, file watch still active; remove collection, assert watch unregistered.

---

#### Story 2.8: CLI Commands (init, write, query stubs) and Env/JSON Flags

**Description:** Implement the CLI commands for Epic 2: `init --root`, `write --tags`, `query/search/vsearch` (as HTTP-forwarding stubs with formatted output), plus the cross-cutting `--json` flag and `NANO_BRAIN_HOST`/`NANO_BRAIN_PORT` env var support.

**Covers:** FR-76, FR-77, FR-78, FR-83, FR-84

**Applies:** AR-17

**Complexity:** S

**Acceptance Criteria:**

- Given `NANO_BRAIN_HOST=myhost` and `NANO_BRAIN_PORT=9090`, when any CLI command runs, then HTTP requests are sent to `myhost:9090` instead of the default `localhost:3100` (FR-83).
- Given `nano-brain init --root /path/to/project`, when it runs against a live server, then it calls `POST /api/init` (or equivalent), the workspace is registered, default collections are created, and an AGENTS.md snippet is printed to stdout (FR-76).
- Given `nano-brain write "some note" --tags decision,arch`, when it runs, then `POST /api/write` is called with `{content: "some note", tags: ["decision", "arch"], workspace: <from config or env>}` and the assigned document ID is printed (FR-78).
- Given `nano-brain query "how does chunking work"`, when it runs, then `POST /api/query` is called and results are printed in a human-readable format (FR-77).
- Given `nano-brain query "test" --json`, when it runs, then the raw JSON response from the API is printed without transformation (FR-84).
- Given `nano-brain write "test" --json`, when it runs, then `{"id": "...", "hash": "...", "collection": "...", "workspace_hash": "..."}` is printed (FR-84).
- Given the server is not running, when any CLI command that requires the server is executed, then a clear error message is printed to stderr with the expected `host:port` and a non-zero exit code.

**Test expectations:**

- Unit: argument parsing and env var overrides tested without a real server (mock HTTP transport).
- Integration: CLI commands run against a live test server, assert correct HTTP requests are made and formatted output matches expectations.

---

## FR Coverage Summary

| FR | Story | Description |
|---|---|---|
| FR-16 | 2.1, 2.3, 2.5 | workspace_hash non-nullable on every write |
| FR-17 | 2.1, 2.3, 2.5 | WHERE workspace_hash in all reads |
| FR-18 | 2.1 | init --root registers workspace |
| FR-19 | 2.1 | GET /api/v1/workspaces |
| FR-22 | 2.6, 2.7 | File watcher monitors collection dirs |
| FR-23 | 2.6 | Debounced reindex (2000ms) |
| FR-24 | 2.6 | Hash-based reindex (skip unchanged) |
| FR-25 | 2.6 | Periodic full-scan poll (300s) |
| FR-26 | 2.6 | Watcher as non-blocking goroutine |
| FR-27 | 2.7 | Collection add/remove/list/rename + live watch |
| FR-28 | 2.3, 2.6 | Storage limits (300KB file, 10GB workspace) |
| FR-48 | 2.4, 2.5 | Chunker: 3600 char target, 200 overlap |
| FR-49 | 2.4 | Chunker: scored break points |
| FR-50 | 2.4 | Chunker: no cut inside code blocks |
| FR-51 | 2.4 | Chunker: merge short chunks |
| FR-52 | 2.4, 2.5 | Chunk position tracking |
| FR-53 | 2.4 | Chunker deterministic |
| FR-59 | 2.3, 2.5 | POST /api/write |
| FR-68 | 2.2, 2.3, 2.7 | Workspace required on data endpoints |
| FR-69 | 2.2 | X-Nano-Brain-Version header |
| FR-70 | 2.2 | JSON only (415 on non-JSON) |
| FR-76 | 2.8 | CLI init --root |
| FR-77 | 2.8 | CLI query/search/vsearch stubs |
| FR-78 | 2.8 | CLI write --tags |
| FR-83 | 2.8 | CLI honors NANO_BRAIN_HOST/PORT |
| FR-84 | 2.8 | CLI --json output |

## Definition of Done (Epic 2)

All 8 stories complete means:

- `nano-brain init --root <dir>` registers a workspace and prints AGENTS.md snippet.
- `nano-brain write "note" --tags tag1` stores a chunked document accessible via BM25 search.
- Creating a markdown file in a watched collection causes it to appear in search within debounce + processing time.
- `nano-brain collection add` attaches a new directory to the watcher without restart.
- All data endpoints return HTTP 400 when workspace is missing.
- `X-Nano-Brain-Version` header appears on every response.
- `go test -race ./...` passes with no race conditions.
- `golangci-lint` reports no errors on all Epic 2 packages.
