---
epic: 8
title: "V1 Migration & Operations"
status: draft
created: 2026-05-23
inputDocuments:
  - docs/epics.md
  - docs/architecture.md
  - docs/prds/prd-nano-brain-greenfield-2026-05-23/prd.md
---

# Epic 8: V1 Migration & Operations

**Epic goal:** Existing v1 users can migrate their SQLite data to PostgreSQL in one command. Operations teams have full CLI and API tooling for monitoring, config hot-reload, log inspection, collection management, and telemetry — all without transmitting any data externally.

**Depends on:** Epic 2 (ingestion pipeline, chunking, storage layer), Epic 3 (embedding pipeline — migration triggers re-embed).

**Packages:** `internal/migrate`, `internal/ops`, `internal/telemetry`, `internal/config`, `internal/collection`

---

## Stories

---

#### Story 8.1: V1 SQLite to PostgreSQL Migration Command

**Description:** A v1 user runs `nano-brain db:migrate --from-v1 <path>` to import their existing documents, chunks, and tags from a v1 SQLite database into the v2 PostgreSQL instance. The command uses `modernc.org/sqlite` (pure Go, no CGO) to read the v1 schema, so the entire project stays `CGO_ENABLED=0`. Embeddings are not migrated — they are regenerated afterward via `nano-brain embed`. The migration reports progress as it runs and completes with a clear "run embed next" instruction.

**Covers:** FR-85, FR-87, FR-88

**Applies:** AR-13 (`modernc.org/sqlite`, pure Go, `CGO_ENABLED=0`)

**Complexity:** L

**Acceptance Criteria:**

- Given a valid v1 SQLite file at the path provided, when `nano-brain db:migrate --from-v1 <path>` is run, then the command reads documents, chunks, and tags from the v1 schema and inserts them into the active v2 PostgreSQL instance via content-addressed upsert.
- Given the migration is running, when it processes records, then it prints `Migrated N / M documents` (updating N as records are inserted) so the user can see progress.
- Given the migration completes without error, when the final record is inserted, then it prints `Migration complete. Run 'nano-brain embed' to regenerate embeddings.`
- Given the v1 SQLite file contains a corrupt or unparseable record, when the migration encounters that record, then it logs the document ID with an error message and continues to the next record without aborting the entire migration.
- Given `CGO_ENABLED=0` is set at build time, when the migration binary is compiled, then it builds successfully (no CGO dependency — `modernc.org/sqlite` used, not `go-sqlite3`).
- Given the command runs on a v2 instance that already has the schema in place, when executed, then `nano-brain status` afterward shows the correct document count from the v1 database (post-migration count equals v1 document count minus corrupt records).
- Given the migration runs, when complete, then `POST /api/search` (BM25) returns migrated documents before `nano-brain embed` is run (embeddings are not required for BM25).

---

#### Story 8.2: Idempotent Migration via Content-Addressed Upsert

**Description:** Running the migration command twice against the same v1 database produces the same document count as running it once — no duplicates are created. This reuses the same content-addressed upsert semantics (`ON CONFLICT DO NOTHING / DO UPDATE`) from the ingestion pipeline, enforcing NFR-4 at the migration layer.

**Covers:** FR-86

**Applies:** AR-13, AR-8 (sqlc upsert queries)

**Complexity:** S

**Acceptance Criteria:**

- Given a v1 SQLite file has been successfully migrated, when `nano-brain db:migrate --from-v1 <path>` is run a second time on the same file, then the final document count in PostgreSQL is unchanged (no duplicates created).
- Given the upsert logic runs, when a document with the same content hash already exists in the v2 database, then the existing record is retained and no new row is inserted.
- Given partial migration was interrupted (e.g., killed mid-run), when the migration is re-run from the beginning, then it completes cleanly and the final state is equivalent to a clean single run.
- Given the idempotency guarantee holds, when an operator runs migration in a CI/CD script that may run multiple times, then the data state is always deterministic with no manual cleanup required.

---

#### Story 8.3: Config Hot-Reload Endpoint

**Description:** Without restarting the server, an operator calls `POST /api/reload-config` to pick up changes from the YAML config file. The endpoint reloads safe settings (collection globs, embedding provider, embedding concurrency, log level, search parameters) and returns a structured response identifying which settings were reloaded, unchanged, or require a full restart. Settings that cannot be changed at runtime (database URL, listen address, workspace root paths) are listed in `requires_restart` rather than silently ignored.

**Covers:** FR-93b

**Applies:** AR-11 (koanf v2 config loading)

**Complexity:** M

**Acceptance Criteria:**

- Given the server is running and the YAML config file has been edited, when `POST /api/reload-config` is called, then the server re-reads the config file and applies changes to reloadable settings without restarting.
- Given reloadable settings were changed in the YAML file, when the reload succeeds, then the response contains `{"reloaded": ["embedding.concurrency", ...], "unchanged": [...], "requires_restart": [...]}` with each changed setting placed in the correct list.
- Given a setting is reloadable (e.g., `embedding.concurrency`), when it changes in the config file and reload is called, then the new value takes effect immediately for subsequent operations (e.g., a new embedding cycle uses the updated concurrency).
- Given a setting requires restart (e.g., `server.port`), when reload is called, then the setting appears in `requires_restart` and the server does not attempt to apply it without a restart.
- Given no changes were made to the config file, when reload is called, then all settings appear in `unchanged` and the response returns HTTP 200 with an empty `reloaded` list.
- Given the config file contains invalid YAML at reload time, when the endpoint is called, then it returns HTTP 400 with a descriptive error and the running configuration is unchanged.

---

#### Story 8.4: CLI Operations Commands (logs, docker, status, db:migrate)

**Description:** The operations CLI surface is complete: `nano-brain logs` tails the rotating log file, `nano-brain docker start/stop/status` wraps docker compose, `nano-brain status` reports full system health, and `nano-brain db:migrate` runs pending goose migrations. All commands honor `NANO_BRAIN_HOST`/`NANO_BRAIN_PORT` and support `--json` output.

**Covers:** FR-79, FR-81, FR-82, FR-96

**Applies:** AR-10 (zerolog log format), AR-9 (goose migrations)

**Complexity:** M

**Acceptance Criteria:**

- Given the server is running and has received queries, when `nano-brain logs -n 50` is run, then it prints the last 50 lines of the current log file to stdout.
- Given `nano-brain logs -f` is run, when new log entries are written, then they appear in stdout in real time (tail -f behavior).
- Given `nano-brain docker start` is run from any directory, when executed, then it runs `docker compose up -d` from the nano-brain installation directory and reports success or failure.
- Given `nano-brain docker stop` is run, when executed, then it runs `docker compose down` and reports the result.
- Given `nano-brain docker status` is run, when executed, then it reports the running status of the `nano-brain` and `postgres` containers (running / stopped / unhealthy).
- Given the server is running, when `nano-brain status` is run, then it prints PostgreSQL connection pool health, active connection count, embedding queue depth, workspace count, and collection stats.
- Given pending goose migrations exist, when `nano-brain db:migrate` is run, then it applies all pending migrations and prints each migration filename as it runs.
- Given `NANO_BRAIN_HOST=myserver` and `NANO_BRAIN_PORT=8080` are set, when any CLI command is run, then it connects to `myserver:8080` instead of `localhost:3100`.
- Given any of these commands is run with `--json`, when output is produced, then it is valid JSON and parseable by a script.

---

#### Story 8.5: Tags Endpoint, Reindex API, and Workspace Listing

**Description:** The remaining operations API endpoints are wired up: `GET /api/v1/tags` returns all tags in the current workspace with document counts, `POST /api/reindex` triggers async reindex for a collection or workspace, `POST /api/update` triggers async reindex of all collections, and `GET /api/v1/workspaces` lists all registered workspaces. Every response includes the `X-Nano-Brain-Version` header. All workspace-data endpoints reject requests without a workspace identifier with HTTP 400.

**Covers:** FR-20, FR-21, FR-61, FR-62, FR-64, FR-65, FR-69

**Applies:** AR-3 (Echo v4 middleware), AR-8 (sqlc queries)

**Complexity:** M

**Acceptance Criteria:**

- Given documents with tags exist in a workspace, when `GET /api/v1/tags?workspace=<hash>` is called, then it returns `[{"tag": "...", "count": N}, ...]` listing all tags used in that workspace with their document counts.
- Given `POST /api/reindex` is called with `{"workspace": "<hash>", "root": "<collection>"}`, when the request is valid, then it returns `{"job_id": "...", "status": "queued"}` and triggers async reindex for that collection.
- Given `POST /api/update` is called with `{"workspace": "<hash>"}`, when executed, then it queues reindex for all collections in that workspace and returns the same `job_id/status` shape.
- Given `GET /api/v1/workspaces` is called, when the server has registered workspaces, then it returns all workspaces with their hashes, document counts, and last-updated timestamps.
- Given `nano-brain collection add/remove/list/rename` is run, when executed, then it manages collections within the current workspace scope only and does not affect other workspaces.
- Given `scope=all` is passed to a read endpoint, when the request is processed, then it returns results merged across all workspaces (read-only; write operations with `scope=all` are rejected with HTTP 400).
- Given any of these endpoints returns a response (success or error), when inspected, then the `X-Nano-Brain-Version: <semver>` header is present.
- Given any of these workspace-data endpoints is called without a `workspace` parameter, when the request is received, then it returns HTTP 400 with `{"error": "workspace_required", "message": "..."}`.

---

#### Story 8.6: Search Telemetry with 90-Day Retention and No External Transmission

**Description:** Every search query (query text, result count, latency, collection, workspace hash) is recorded to the `telemetry_logs` table in PostgreSQL. Records older than 90 days (configurable via `telemetry.retention_days`) are evicted. No telemetry or log data is ever transmitted outside the local system — no analytics endpoints, no outbound calls except to the configured embedding provider. NFR-5 is tested explicitly.

**Covers:** FR-97, FR-98

**Applies:** AR-8 (sqlc `telemetry.sql` queries), AR-10 (zerolog local-only logging)

**Complexity:** S

**Acceptance Criteria:**

- Given a search query is executed via any of the three search endpoints (`/api/query`, `/api/search`, `/api/vsearch`), when the response is returned, then a row is inserted into `telemetry_logs` containing: query text, result count, latency in ms, collection name, and workspace hash.
- Given `telemetry.retention_days` is set to 90 (default), when the cleanup job runs, then records with `created_at` older than 90 days are deleted from `telemetry_logs`.
- Given `telemetry.retention_days` is reconfigured (e.g., to 30), when the config is reloaded and cleanup runs, then records older than 30 days are evicted.
- Given the server is running in an offline environment (no external network access), when multiple queries are executed, then no outbound network connections are made to any host other than the configured embedding provider (verifiable via network monitoring or by running with no embedding provider configured).
- Given the server has been running and processing queries, when `nano-brain logs` is inspected, then log entries show query activity locally but contain no external endpoint URLs or API calls to analytics services.
- Given workspace isolation applies to telemetry, when telemetry is queried internally for a specific workspace, then only that workspace's records are returned (consistent with NFR-2).
