# Epic 1: Foundation & Data Layer — User Stories

**Epic goal:** The nano-brain server starts, connects to PostgreSQL, runs migrations, responds to health checks, and loads configuration. A developer can `docker compose up` and get a working (empty) instance with health monitoring and structured logging.

**Sequence note:** Stories must be implemented in order. Each story builds on the previous one. E1.1 has no dependencies. E1.6 requires all prior stories to be complete.

---

#### Story 1.1: Go Module Init and Project Skeleton

**Description:** Initialize the Go module and create the canonical project directory structure so all subsequent stories have a consistent, compilable foundation to build on. This includes the `cmd/nano-brain/` entrypoint, all `internal/` package stubs, the `migrations/` directory, `Makefile`, `.golangci.yml`, and `sqlc.yaml`. No business logic is implemented here — only the scaffold that compiles clean.

**Covers:** AR-1 (`CGO_ENABLED=0` static binary, `go mod init` from scratch), AR-16 (14 `/internal/` packages, inward dependency flow), AR-17 (naming conventions), AR-18 (`golangci-lint` + `go test -race` as CI gates)

**Applies:** AR-1, AR-16, AR-17, AR-18

**Complexity:** S

**Acceptance Criteria:**

- Given a clean checkout with no existing Go files, when `go mod init github.com/nano-brain/nano-brain` runs, then `go.mod` is created with `go 1.23` or later and module path `github.com/nano-brain/nano-brain`.
- Given the project root, when `ls` runs, then the following directories exist: `cmd/nano-brain/`, `internal/server/`, `internal/mcp/`, `internal/search/`, `internal/harvest/`, `internal/watcher/`, `internal/embed/`, `internal/chunk/`, `internal/collection/`, `internal/storage/`, `internal/migrate/`, `internal/bench/`, `internal/config/`, `internal/telemetry/`, `internal/health/`, `internal/testutil/`, `migrations/`, `docker/`, `bench/testdata/`, `docs/`.
- Given each `internal/` package directory, when `ls` runs, then each contains at least one `.go` file with a valid `package` declaration matching the directory name.
- Given `cmd/nano-brain/main.go`, when `go build ./...` runs, then the binary compiles with zero errors and zero `golangci-lint` warnings at the configured lint rules.
- Given `CGO_ENABLED=0 go build ./...`, when the build completes, then the resulting binary is a static binary with no CGO dependencies (verified by `file` or `ldd`).
- Given `Makefile`, when `make build` runs, then it invokes `CGO_ENABLED=0 go build` and places the binary in `./bin/nano-brain`.
- Given `Makefile`, when `make lint` runs, then it invokes `golangci-lint run ./...`.
- Given `Makefile`, when `make test` runs, then it invokes `go test -race -short ./...` and exits 0 on a clean repo.
- Given `.golangci.yml` at project root, when `golangci-lint run` executes, then it applies at minimum: `errcheck`, `govet`, `staticcheck`, `unused`.
- Given `sqlc.yaml` at project root, when `sqlc generate` runs (after queries exist), then it generates into `internal/storage/sqlc/` without errors.

**Test expectations:**
- No integration tests at this story. Unit test: `TestMainPackageCompiles` — a build-tag-gated test that calls `go build ./cmd/nano-brain/` and asserts exit 0.

---

#### Story 1.2: Configuration System

**Description:** Implement the configuration loading layer in `internal/config/`. nano-brain reads YAML from `~/.nano-brain/config.yml` (overridable via `--config` flag or `NANO_BRAIN_CONFIG_PATH`), generates a default file on first run if none exists, merges environment variable overrides, and exits with a descriptive error on invalid config. All configurable values from PRD §4.13 are defined as struct fields with sane defaults.

**Covers:** FR-89, FR-90, FR-91, FR-92, FR-93

**Applies:** AR-11 (koanf v2 for YAML + env loading), AR-6 (manual constructor injection), AR-17 (naming conventions)

**Complexity:** M

**Acceptance Criteria:**

- Given no config file at `~/.nano-brain/config.yml`, when the server starts, then it creates the file from built-in defaults and logs `"generated default config"` at info level.
- Given `--config=/tmp/test.yml` flag, when the server starts, then it reads from `/tmp/test.yml` instead of the default path.
- Given `NANO_BRAIN_CONFIG_PATH=/tmp/test.yml` env var (no `--config` flag), when the server starts, then it reads from `/tmp/test.yml`.
- Given a valid YAML config, when `config.Load()` is called, then it returns a populated `Config` struct with no error.
- Given a YAML config with `embedding.concurrency: 5`, when `config.Load()` is called, then `cfg.Embedding.Concurrency == 5`.
- Given the env var `NANO_BRAIN_PORT=8080`, when `config.Load()` is called, then `cfg.Server.Port == 8080` regardless of the YAML value.
- Given the env var `NANO_BRAIN_HOST=0.0.0.0`, when `config.Load()` is called, then `cfg.Server.Host == "0.0.0.0"`.
- Given the env var `VOYAGE_API_KEY=mykey`, when `config.Load()` is called, then `cfg.Embedding.VoyageAPIKey == "mykey"`.
- Given a YAML file with an unknown top-level key (e.g., `unknown_key: true`), when the server starts, then it prints `"unknown config key: unknown_key"` and exits with a non-zero status code.
- Given a YAML file with `embedding.concurrency: -1` (below valid range), when the server starts, then it prints a descriptive validation error and exits non-zero.
- Given no YAML config and no env vars, when `config.Load()` is called, then the returned `Config` contains working defaults: `Server.Port=3100`, `Server.Host="localhost"`, `Embedding.Provider="ollama"`, `Embedding.Concurrency=3`, `Intervals.SessionPoll=120`, `Watcher.DebounceMs=2000`, `Watcher.ReindexInterval=300`, `Search.RrfK=60`, `Search.RecencyWeight=0.3`, `Search.RecencyHalfLifeDays=180`, `Storage.MaxFileSizeBytes=314572800` (300 MB), `Logging.Level="info"`.
- Given all FR-91 config keys, when reviewing the `Config` struct, then every key listed in §4.13 has a corresponding typed field.

**Test expectations:**
- Unit tests (no PG): `TestLoadDefaults`, `TestEnvVarOverride`, `TestConfigFileOverride`, `TestUnknownKeyRejectsStartup`, `TestInvalidRangeRejectsStartup`, `TestGeneratesDefaultConfigFile`.
- All tests use a temp directory to avoid touching `~/.nano-brain`.

---

#### Story 1.3: Structured Logging

**Description:** Implement the logging layer in `internal/health/` (shared logger bootstrap) using zerolog. Logs write to both stdout and a daily rotating log file at `~/.nano-brain/logs/nano-brain-YYYY-MM-DD.log`. Rotation caps at 50 MB per file, 5 files retained. Log level is configurable via config and env var. A shared logger instance is constructed once in `main()` and injected into all components.

**Covers:** FR-94, FR-95

**Applies:** AR-10 (zerolog for structured JSON logging), AR-6 (constructor injection), AR-17 (logging convention from architecture doc)

**Complexity:** S

**Acceptance Criteria:**

- Given `logging.level: "debug"` in config, when any component logs at debug level, then the message appears in the log file.
- Given `logging.level: "warn"` in config, when a component logs at info level, then the message does not appear in the log file.
- Given `NANO_BRAIN_LOG=error` env var, when the server starts, then only error-level messages appear, regardless of the YAML `logging.level` value.
- Given the server has been running, when any log entry is written, then it is a valid JSON object containing at minimum: `level`, `time`, and `message` fields.
- Given `logging.file` is set to a writable path, when the server writes log entries, then they appear in both stdout and the file at that path.
- Given the default log path `~/.nano-brain/logs/nano-brain-YYYY-MM-DD.log`, when the server starts, then the log directory is created if it does not exist.
- Given a log file that reaches 50 MB, when the next log entry is written, then the logger rotates to a new file without crashing.
- Given 5 rotated files already exist, when a 6th rotation occurs, then the oldest file is deleted so only 5 files are retained.
- Given the zerolog logger is constructed in `main()`, when any `internal/` package logs, then it does so via the injected `zerolog.Logger` instance, never via a global `log` package call.
- Given the logging convention from the architecture doc, when a component logs an operational event, then it uses `log.Info().Str(...).Int(...).Dur(...).Msg(...)` format — no `log.Fatal` outside `main()`, no `panic` for recoverable errors.

**Test expectations:**
- Unit tests (no PG): `TestLogLevelFilter`, `TestLogOutputIsJSON`, `TestLogRotationTriggersOnSize` (uses a temp dir and a mock writer), `TestEnvVarOverridesLogLevel`.
- No integration tests for this story.

---

#### Story 1.4: Database Layer — Pool, Migrations, and Schema

**Description:** Implement the PostgreSQL connection pool in `internal/storage/pool.go` using pgx v5, write the initial goose migration that creates the core schema tables, configure sqlc, and generate type-safe query stubs. This story establishes the database foundation all subsequent data operations build on. No query logic beyond pool initialization and migration runner is implemented here.

**Covers:** FR-42 (transactional mutations), FR-43 (upsert semantics), FR-44 (auto-migration on startup), FR-46 (pool health checks)

**Applies:** AR-2 (PG 17 + pgvector 0.8.2, HNSW vector index), AR-8 (sqlc for type-safe SQL), AR-9 (goose v3 migrations in `/migrations/`), AR-6 (constructor injection), NFR-1 (pgxpool goroutine-safe, context.WithTimeout on every tx), NFR-4 (atomic transactions, idempotent upserts)

**Complexity:** L

**Acceptance Criteria:**

- Given a running PostgreSQL 17 instance with pgvector installed, when `storage.NewPool(ctx, cfg)` is called, then it returns a connected `*pgxpool.Pool` with no error.
- Given the pool is created, when `pool.Ping(ctx)` is called with a 5-second timeout, then it returns nil.
- Given `DATABASE_URL` env var is unset and config has a valid DSN, when `storage.NewPool(ctx, cfg)` is called, then it uses the config DSN.
- Given `DATABASE_URL` env var is set, when `storage.NewPool(ctx, cfg)` is called, then `DATABASE_URL` takes precedence.
- Given `migrate.Up(ctx, pool)` is called on a fresh database, when goose runs the initial migration, then all tables exist: `workspaces`, `documents`, `chunks`, `embeddings`, `collections`, `telemetry_logs`.
- Given `migrate.Up(ctx, pool)` is called on a database already at the latest version, when it runs, then it completes with no error and no schema changes (idempotent).
- Given the `documents` table, when examining its columns, then `workspace_hash TEXT NOT NULL` is present with an index `idx_documents_workspace_hash`.
- Given the `chunks` table, when examining its columns, then `content_hash TEXT NOT NULL` is present with a unique constraint `uq_chunks_content_hash_workspace_hash`.
- Given the `embeddings` table, when examining its columns, then a `vector(1536)` column exists and an HNSW index using cosine distance is created.
- Given `pgx.BeginTxFunc(ctx, pool, ...)` is used for a write operation, when the inner function returns an error, then the transaction is rolled back and no partial data is written.
- Given a document insert query, when the same content hash is inserted twice, then the second insert uses `ON CONFLICT DO NOTHING` and returns no error (idempotent).
- Given `sqlc.yaml` is configured, when `sqlc generate` runs after query files are placed in `internal/storage/queries/`, then it generates `internal/storage/sqlc/` with no errors and Go types matching the schema.
- Given pgxpool's `HealthCheckPeriod`, when an idle connection becomes unhealthy, then the pool removes and replaces it without requiring a server restart.

**Test expectations:**
- Unit tests (mock PG): `TestNewPoolReturnsMockPool` (using a mock that implements the pool interface), `TestMigrateUpIdempotent` (using a test helper that sets up a real or mock schema).
- Integration tests (`//go:build integration`, real PG via `host.docker.internal:5432`): `TestPoolConnects`, `TestMigrateCreatesTables`, `TestUpsertIsIdempotent`, `TestTransactionRollbackOnError`, `TestHNSWIndexExists`.
- Test helpers live in `internal/testutil/testdb.go`: `SetupTestDB(t) *pgxpool.Pool` creates a fresh schema per test and defers cleanup.

---

#### Story 1.5: HTTP Server and Health Endpoints

**Description:** Implement the Echo v4 HTTP server in `internal/server/` with startup/shutdown lifecycle, route registration, centralized error handling middleware, and the two health endpoints: `GET /health` and `GET /api/status`. The server wires config and the PG pool via constructor injection. Graceful shutdown triggers on `SIGTERM`/`SIGINT` using errgroup and context cancellation.

**Covers:** FR-45 (`GET /health` returns ready only when PG healthy and migrations current), FR-46 (pool health checks), FR-47 (`nano-brain status` reports PG pool health, active connections, pending migration count), FR-54 (`GET /health`, no auth), FR-55 (`GET /api/status`, full index health)

**Applies:** AR-3 (Echo v4 with centralized `HTTPErrorHandler`), AR-5 (errgroup + context for background goroutines), AR-6 (constructor injection), D4 (stdlib errors + custom domain types + Echo error handler), D8 (manual constructor injection), D9 (consumer-side interfaces)

**Complexity:** M

**Acceptance Criteria:**

- Given the server is started, when `GET /health` is called and PG pool is reachable and migrations are current, then it returns HTTP 200 with body `{"status":"ok","ready":true,"version":"<semver>","uptime_s":<N>,"workspace_count":<N>}`.
- Given the server is started but PG is unreachable, when `GET /health` is called, then it returns HTTP 200 with body `{"status":"degraded","ready":false,"reason":"database unreachable"}` (note: health endpoints return 200 always so load balancers can read the `ready` field).
- Given the server is started but migrations are pending, when `GET /health` is called, then it returns HTTP 200 with `"ready":false` and a reason describing the pending migration count.
- Given the server is started with a healthy PG, when `GET /api/status` is called, then it returns HTTP 200 with a body including: `pg_status`, `migration_version`, `embedding_queue_depth`, `active_provider`, `workspace_count`.
- Given any request to any endpoint, when the response is sent, then the `X-Nano-Brain-Version` header is set to the current semver (from `version.go` or linker flag).
- Given a request body that is not valid JSON to a JSON endpoint, when the handler processes it, then it returns HTTP 415 with `{"error":"unsupported_media_type","message":"Content-Type must be application/json"}`.
- Given an internal error (e.g., PG query fails) in any handler, when Echo's `HTTPErrorHandler` processes it, then it returns HTTP 500 with `{"error":"internal_error","message":"..."}` and logs the error at error level via zerolog, once only (no double-logging).
- Given `SIGTERM` signal while the server is handling a request, when the signal is received, then: (1) no new requests are accepted, (2) in-flight requests complete (up to 5-second drain timeout), (3) the pool is closed, (4) the process exits 0.
- Given Echo's router, when `make test` runs, then registered routes match: `GET /health`, `GET /api/status`.
- Given the server's `HTTPErrorHandler`, when a `domain.ErrWorkspaceRequired` error reaches it, then it maps to HTTP 400 (not 500).

**Test expectations:**
- Unit tests (mock PG): `TestHealthEndpointHealthyDB`, `TestHealthEndpointUnreachableDB`, `TestStatusEndpointShape`, `TestVersionHeader`, `TestHTTPErrorHandlerMapsErrors`, `TestNonJSONBodyReturns415`.
- Integration tests (`//go:build integration`, real PG): `TestHealthEndpointE2E` — starts a real server, hits `/health`, asserts `ready:true`.
- No mocking framework; use consumer-side interfaces defined in `internal/server/`.

---

#### Story 1.6: Docker — Multi-Stage Dockerfile and Compose Integration

**Description:** Write the multi-stage Dockerfile (build stage: `golang:1.23-bookworm`, runtime stage: `gcr.io/distroless/static-debian12`) and `docker/docker-compose.yml` declaring the `nano-brain` and `postgres` services. The compose file wires the PG service, a named volume for data persistence, environment variable plumbing, and a health check on the nano-brain container that calls `GET /health`. The CLI stub for `nano-brain docker start/stop` wraps `docker compose up -d` and `docker compose down`.

**Covers:** FR-99 (Docker Compose: nano-brain + PG 17 + pgvector 0.8.2), FR-100 (`docker compose up` ready ≤30s), FR-101 (PG data on named volume), FR-102 (container auto-migrates on startup), FR-103 (container health check every 30s, unhealthy after 3 failures), FR-104 (Docker env detection for PG host), FR-105 (CLI `docker start/stop` wraps compose)

**Applies:** AR-7 (multi-stage Dockerfile: golang:1.23 build → distroless runtime ~15MB), AR-1 (`CGO_ENABLED=0` static binary), AR-15 (dev-in-container: PG on host via `host.docker.internal`)

**Complexity:** M

**Acceptance Criteria:**

- Given the `docker/Dockerfile`, when `docker build -f docker/Dockerfile .` runs, then it produces an image with no build errors.
- Given the built image, when `docker run --rm <image> /nano-brain --help` runs, then it exits 0 (binary is present and executes).
- Given `CGO_ENABLED=0` is set in the build stage, when the resulting binary is examined with `ldd` (or `file`), then it is a static binary with no dynamic library dependencies.
- Given `docker/docker-compose.yml`, when `docker compose config` runs, then it validates with no errors.
- Given `docker compose up`, when both services start, then `GET /health` on the nano-brain container returns `ready:true` within 30 seconds.
- Given the compose file's `postgres` service, when examining its image, then it is `pgvector/pgvector:0.8.2-pg17` (pinned version).
- Given the compose file's `postgres` service, when examining its volumes, then a named volume (e.g., `nanobrain_pgdata`) is mounted at `/var/lib/postgresql/data`.
- Given `docker compose down && docker compose up`, when `/health` returns `ready:true` again, then documents written before the restart are still present (data persisted via named volume).
- Given the nano-brain container's health check config, when `/health` returns a non-200 status 3 consecutive times, then `docker inspect` reports the container as `unhealthy`.
- Given the nano-brain container is running inside Docker, when the server reads the database DSN, then `DATABASE_URL` (set in compose env) uses the internal postgres service name (e.g., `postgres://...@postgres:5432/nanobrain`) rather than `localhost`.
- Given `FR-104` Docker env detection, when the binary detects it is inside a Docker container (via `/.dockerenv` presence or `DOCKER_ENV=true` env var), then it substitutes `localhost` with `postgres` in the default PG host if no `DATABASE_URL` is set.
- Given `nano-brain docker start` CLI command, when it runs, then it executes `docker compose up -d` from the nano-brain installation directory (resolved via the binary's own path or `NANO_BRAIN_INSTALL_DIR` env var) and prints the compose output.
- Given `nano-brain docker stop` CLI command, when it runs, then it executes `docker compose down` and prints the compose output.
- Given the distroless runtime stage, when the final image size is measured, then it is under 20 MB (15 MB target; distroless base adds ~5 MB to the static binary).

**Test expectations:**
- No unit tests for Docker artifacts.
- Integration test (`//go:build integration`): `TestDockerHealthCheckPassesWithinThirtySeconds` — runs `docker compose up -d` in the test setup, polls `/health` until `ready:true` or 35-second timeout, asserts success. Runs in CI only.
- Manual smoke test: `make smoke` target that runs `./nano-brain status` against a live Docker stack and exits 0.
