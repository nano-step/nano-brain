---
stepsCompleted: [1, 2, 3, 4, 5, 6, 7, 8]
lastStep: 8
status: 'complete'
completedAt: '2026-05-23'
inputDocuments:
  - docs/prds/prd-nano-brain-greenfield-2026-05-23/prd.md
  - docs/briefs/brief-nano-brain-greenfield-2026-05-22/brief.md
  - docs/research/technical-nano-brain-v2-stack-selection-research-2026-05-22.md
  - docs/reference-prd.md
  - docs/reference-readme.md
workflowType: 'architecture'
project_name: 'nano-brain-greenfield'
user_name: 'BMad'
date: '2026-05-23'
---

# Architecture Decision Document

_This document builds collaboratively through step-by-step discovery. Sections are appended as we work through each architectural decision together._

---

## Project Context Analysis

### Requirements Overview

**Functional Requirements:**
111 FRs organized across 15 feature groups: Session Harvesting (FR-1–6), Hybrid Search (FR-7–15), Per-Workspace Isolation (FR-16–21), File Watcher (FR-22–28), Embedding Providers (FR-29–36e), Benchmarking Suite (FR-37–41), Corruption Detection & Recovery (FR-42–47), Chunking Strategy (FR-48–53), HTTP API (FR-54–70), MCP-over-HTTP (FR-71–75, FR-72b), CLI (FR-77–84), Data Migration (FR-85–88), Configuration System (FR-89–93b), Logging & Telemetry (FR-94–98), Docker Compose Deployment (FR-99–105).

Core pipeline: Harvest → Chunk → Embed → Store → Search → Return. Each stage maps to a distinct module. The pipeline is write-heavy during ingestion and read-heavy during agent interaction.

**Non-Functional Requirements:**
- Concurrency safety: All shared state in PostgreSQL. CI runs `go test -race`. No application-level mutexes around pool access. (§8.1)
- Workspace isolation invariant: Binary, non-negotiable. WHERE clause enforcement, not post-filter. Missing workspace → HTTP 400. (§8.2)
- Search quality baseline: v2 hybrid must meet v1 metrics (P@5≥0.835, R@10≥0.970, MRR≥1.000). Benchmarking suite is the instrument. (§8.3)
- Data integrity: Atomic transactions, idempotent upserts, forward-only migrations, derivable cache principle. (§8.4)
- Privacy: Ollama default (local, no outbound). VoyageAI opt-in. No telemetry transmitted externally. (§8.5)

**Scale & Complexity:**
- Primary domain: Backend API / data pipeline
- Complexity level: Medium-high
- Estimated architectural components: 12–15 (HTTP server, MCP adapter, search engine, harvester, watcher, embedder, chunker, migration tool, CLI, config loader, benchmark suite, health checker, logging)

### Technical Constraints & Dependencies

| Constraint | Decision |
|---|---|
| Language | Go 1.23+ |
| Database | PostgreSQL 17 + pgvector 0.8.2 (pinned Docker image) |
| MCP Transport | HTTP only (SSE + Streamable HTTP). No stdio. |
| Deployment | Docker Compose. Single-node only. |
| Embedding providers | Ollama (default, local) and VoyageAI (cloud) |
| Vector scale | Max 500,000 vectors per workspace |
| Config format | YAML + env var overrides |
| Platforms | Linux and macOS. No Windows native (WSL2 acceptable). |

### Cross-Cutting Concerns Identified

1. **Workspace hash propagation**: Partition key must flow from HTTP/MCP entry point through service layer to every SQL query. No default fallback — missing workspace is an error.
2. **Content-addressed deduplication**: SHA-256 hashing for documents and chunks. Upsert semantics (ON CONFLICT) ensure idempotency across all write paths.
3. **Transaction boundaries**: Every mutation (document, chunk, embedding, metadata) runs in a single PostgreSQL transaction. Partial writes are never visible to readers.
4. **Background job isolation**: Watcher, harvester, and embedder goroutines communicate only via PostgreSQL. No in-memory shared mutable state between them.
5. **Graceful degradation**: BM25 search works without embeddings. Vector search returns empty (not error) when no embeddings exist. System remains available during provider outages.
6. **Tier 2 module boundaries**: Architecture must define clean interfaces at ingestion, search pipeline, and storage layers so code intelligence, knowledge graph, self-learning, and consolidation features can be added without refactoring core modules.
7. **Configuration cascading**: YAML file → environment variables → explicit reload endpoint. Reloadable vs restart-required settings are separated.

---

## Project Foundation

### Primary Technology Domain

Backend API / data pipeline (Go 1.23+). No starter template applicable — Go projects initialize from `go mod init` and build structure manually.

### Initialization

```bash
go mod init github.com/nano-brain/nano-brain
```

### Project Layout

Standard Go Project Layout (`/cmd`, `/internal`, `/pkg`):

- `/cmd/nano-brain/` — single entrypoint for both CLI and server modes
- `/internal/` — private packages (one per architectural component)
- `/pkg/` — public packages (if any emerge; empty initially)
- `/migrations/` — goose SQL migration files
- `/docker/` — Dockerfile, docker-compose.yml
- `/bench/` — benchmarking suite data and scripts
- `/docs/` — architecture, PRD, and API reference

**Rationale:** 12–15 components map cleanly to `/internal/` sub-packages. Standard layout is the most recognized convention in the Go ecosystem. `/internal/` enforces compile-time import boundaries — external consumers cannot import private packages.

### Key Libraries (Locked via Technical Research)

| Component | Library | Purpose |
|---|---|---|
| DB driver | pgx v5 | PostgreSQL connection pooling, COPY protocol |
| Vector ops | pgvector-go | pgvector type support for Go |
| SQL generation | sqlc | Type-safe SQL → Go code generation |
| File watching | fsnotify v1 | OS-level file change notifications |
| Config | koanf v2 | YAML + env var config with merge semantics |
| Logging | zerolog | Structured JSON logging, zero-allocation |
| Migrations | goose v3 | SQL schema versioning |
| Testing | stdlib `testing` + real PG | Integration tests connect real PostgreSQL via `host.docker.internal` |
| MCP server | modelcontextprotocol/go-sdk v1.5+ | MCP-over-HTTP (SSE + Streamable HTTP) |

### Build Tooling

- `Makefile` for build, test, lint, migrate commands
- `golangci-lint` for static analysis (CI gate)
- `go test -race` for race detection (CI gate)
- Multi-stage Dockerfile: build stage (golang:1.23) → runtime stage (distroless/static)

---

## Core Architectural Decisions

### Decision Priority Analysis

**Critical Decisions (Block Implementation):**
D1 (schema strategy), D2 (vector index), D3 (HTTP framework), D5 (MCP SDK), D6 (goroutine coordination), D7 (embedding queue), D8 (DI pattern)

**Important Decisions (Shape Architecture):**
D4 (error handling), D9 (interface design), D10 (Docker strategy)

**Deferred Decisions (Post-MVP):**
D11 (CI pipeline — document shape now, implement when code exists)

### Data Architecture

**D1: Single schema + workspace_hash column partitioning.**
All workspaces share the same tables. Every query includes `WHERE workspace_hash = $1`. No per-workspace PostgreSQL schemas. Rationale: PRD §8.2 assumes column-level partitioning. Simpler for single-node deployment. Workspace isolation enforced at query layer, not schema layer.

**D2: HNSW vector index (pgvector).**
HNSW over IVFFlat. Better recall quality without periodic reindex maintenance. 500K vectors/workspace is within HNSW sweet spot. Aligns with North Star (correctness/reliability). Trade-off: higher memory usage and slower index build time — acceptable for single-node, write-infrequent workload.

### API & Communication Patterns

**D3: Echo v4 HTTP framework.**
17+ HTTP endpoints, MCP transport routes, and middleware requirements (workspace extraction, version header, content-type validation, structured error responses) justify a lightweight framework over stdlib `net/http`. Echo adds one dependency. Middleware chain and centralized `HTTPErrorHandler` directly serve FR-68 (workspace validation), FR-69 (version header), FR-70 (content-type check).

**D4: Stdlib errors + custom domain types + Echo error handler.**
Go stdlib `errors.New()`, `errors.Is()`, `errors.As()` with custom domain error types: `ErrWorkspaceRequired`, `ErrDocumentNotFound`, `ErrEmbeddingQueueFull`, etc. Echo's `HTTPErrorHandler` maps domain errors to HTTP status codes centrally. No external error library. Stack traces via zerolog context, not error wrapping.

**D5: Official MCP Go SDK (`github.com/modelcontextprotocol/go-sdk`).**
Verified: the official SDK (v1.5.0+) supports both SSE server (`mcp.SSEHandler`) and Streamable HTTP server (`mcp.StreamableHTTPHandler`). Both types implement Go's standard `http.Handler` interface — zero integration friction with Echo v4. The community `mark3labs/mcp-go` (v0.54.0) also supports both transports but uses encapsulated wrappers rather than `http.Handler`. Official SDK wins on Go idioms, Anthropic backing, and spec compliance guarantee.

### Background Job Architecture

**D6: errgroup + context cancellation.**
Three background goroutines (file watcher, session harvester, embedding queue processor) managed by `golang.org/x/sync/errgroup`. Cancellation propagates via `context.Context`. Graceful shutdown sequence: cancel root context → goroutines drain current work → `errgroup.Wait()` blocks until all complete → server exits. Aligns with FR-46 (graceful shutdown).

**D7: Buffered channel (10K capacity) for embedding queue.**
`chan ChunkID` with capacity 10,000. Producer (chunker) sends chunk IDs after insert. Consumer (embedder) receives and processes. Reject threshold: `len(ch) >= 50,000` is not possible with 10K channel — instead, a separate atomic counter tracks total pending (including database-persisted backlog). Queue state is ephemeral by design: embeddings are derivable from chunks (PRD §8.4). On restart, embedder scans for chunks without embeddings.

### Configuration & Dependency Injection

**D8: Manual constructor injection.**
Each component takes dependencies as constructor parameters: `NewSearchService(pool *pgxpool.Pool, embedder EmbeddingProvider, cfg SearchConfig)`. No DI framework (Wire, fx). 12–15 components is manageable with manual wiring in `main()`. Dependencies are explicit in function signatures. Tests pass mock implementations.

**D9: Consumer-side interfaces (Go convention).**
Interfaces defined where they are consumed, not where they are implemented. Example: `internal/search/service.go` defines `type EmbeddingProvider interface { Embed(ctx context.Context, text string) ([]float32, error) }`. The search package does not import the embed package. This keeps packages decoupled and follows the Go proverb: "Accept interfaces, return structs."

### Infrastructure & Deployment

**D10: Multi-stage Dockerfile.**
- Build stage: `golang:1.23-bookworm` — compile static binary with CGO_ENABLED=0.
- Production runtime: `gcr.io/distroless/static-debian12` — ~15MB image, no shell, minimal attack surface.
- Development runtime: `alpine:3.19` — shell available for debugging.
- Dockerfile uses build arg to switch: `ARG RUNTIME=distroless`. Default = distroless.

**D11: CI pipeline shape (deferred to implementation).**
Planned stages: `go build` → `golangci-lint` → `go test -race -short ./...` → `go test -race -tags=integration ./...` (real PG) → Docker build → release gates (G1–G7 from PRD §7.1). Implementation deferred until code exists.

### Data Migration & Collections

**D12: Hybrid migration strategy (v1 SQLite → v2 PostgreSQL).**
Migrate user-created data only: `documents`, `content`, tags, supersede relationships (~3-4 tables). Regenerate everything else (code index, embeddings, knowledge graph, learning data, cache). Rationale: user-written memories/notes are irreplaceable; code index and embeddings are derivable from source code via v2 pipeline (better quality than importing v1 artifacts). Migration tool: `nano-brain migrate --from ~/.nano-brain/data/` scans all `*.sqlite` files, extracts user data, transforms workspace hash, inserts into PostgreSQL, triggers reindex. Requires `go-sqlite3` (CGO) in migration binary only — not in core v2.

**D13: User-defined collections (Tier 1).**
Retain v1's collection system: users can add arbitrary folders (Obsidian vault, notes directory, custom docs) for nano-brain to watch + index. CLI commands: `collection add/remove/list/rename`. Each collection has: name, path, glob pattern, update mode (auto/manual), exclude folders. Default collections: `memory` (`~/.nano-brain/memory/**/*.md`), `sessions` (`~/.nano-brain/sessions/**/*.md`). Custom collections stored in config YAML + database metadata. File watcher (fsnotify) monitors all configured collection paths.

### Dev Environment

**D14: Dev-in-container with host services.**
Development runs entirely inside OpenCode container. Go toolchain, build, lint, and all tests execute in-container. PostgreSQL and Ollama run on the host (Docker or native) and are accessed via `host.docker.internal`. No Docker-in-Docker. No testcontainers-go. Integration and E2E tests connect real PostgreSQL — test environment matches production topology. Config: `DATABASE_URL=postgres://user:pass@host.docker.internal:5432/nanobrain_dev`. AI agent operates autonomously (code → build → migrate → test → fix → verify) without human intervention.

| Test type | Runs in | PostgreSQL | Build tag |
|---|---|---|---|
| Unit | Container | Not needed (mock interfaces) | (default) |
| Integration | Container | `host.docker.internal:5432` | `//go:build integration` |
| E2E | Container | Same PG connection | `//go:build e2e` |
| Benchmark | Container | Same PG connection | `//go:build bench` |

### Decision Impact Analysis

**Implementation Sequence:**
1. D1 (schema) + D2 (HNSW) → database schema design and migration files
2. D3 (Echo) + D4 (errors) → HTTP server skeleton and middleware
3. D8 (DI) + D9 (interfaces) → package structure and component wiring
4. D5 (MCP SDK) → MCP transport layer (after verification)
5. D6 (errgroup) + D7 (channel queue) → background job infrastructure
6. D10 (Docker) → Dockerfile and docker-compose.yml

**Cross-Component Dependencies:**
- D1 (schema) affects every component that touches the database
- D3 (Echo) + D5 (MCP SDK) must coexist on the same HTTP server — Echo mounts MCP routes
- D6 (errgroup) manages D7 (embedding queue consumer) lifecycle
- D8 (DI) + D9 (interfaces) define how all components connect in main()
- D10 (Docker) depends on D1 (PostgreSQL) and wraps the final binary

---

## Implementation Patterns & Consistency Rules

### Naming Patterns

**Database (PostgreSQL):**
- Tables: `snake_case`, plural — `documents`, `chunks`, `embeddings`, `workspaces`, `telemetry_logs`
- Columns: `snake_case` — `workspace_hash`, `content_hash`, `created_at`, `updated_at`
- Primary keys: `id` (UUID, generated by PostgreSQL `gen_random_uuid()`)
- Foreign keys: `{referenced_table_singular}_id` — `document_id`, `chunk_id`
- Indexes: `idx_{table}_{columns}` — `idx_documents_workspace_hash`, `idx_chunks_content_hash`
- Constraints: `uq_{table}_{columns}` (unique), `fk_{table}_{ref}` (foreign key)

**API (HTTP + MCP):**
- Endpoints: `/api/{resource}` — lowercase, no trailing slash
- JSON fields: `snake_case` — `workspace_hash`, `query_ms`, `created_at`
- Query params: `snake_case` — `?workspace=abc&limit=10`
- Headers: `X-Nano-Brain-Version` (PRD FR-69)

**Go Code:**
- Packages: single lowercase word — `search`, `embed`, `harvest`, `chunk`, `config`
- Files: `snake_case.go` — `search_service.go`, `embedding_provider.go`
- Types: `PascalCase` — `SearchService`, `EmbeddingProvider`, `ChunkResult`
- Functions: `PascalCase` (exported), `camelCase` (unexported)
- Variables: `camelCase` — `workspaceHash`, `queryMs`
- Constants: `PascalCase` for exported, `camelCase` for unexported
- Interfaces: action-oriented — `Searcher`, `Embedder`, `Chunker` (not `ISearcher`, `SearchInterface`)
- Test files: `{file}_test.go` co-located with source

### Structure Patterns

**Package Organization (`/internal/`):**

```
internal/
├── server/          # HTTP server, Echo setup, middleware
├── mcp/             # MCP transport adapters (SSE, Streamable HTTP)
├── search/          # Hybrid, BM25, vector search services
├── harvest/         # Session harvester (OpenCode, Claude Code)
├── watcher/         # File watcher (fsnotify)
├── embed/           # Embedding providers (Ollama, VoyageAI), queue
├── chunk/           # Document chunking algorithm
├── storage/         # PostgreSQL repository layer (sqlc generated + helpers)
├── migrate/         # v1→v2 migration tool, goose runner
├── bench/           # Benchmarking suite
├── config/          # YAML + env config loading (koanf)
├── telemetry/       # Search telemetry recording
└── health/          # Health check logic
```

**Test Organization:**
- Unit tests: co-located `{file}_test.go`
- Integration tests: co-located with `//go:build integration` build tag
- Benchmark test data: `/bench/testdata/`
- Test helpers: `internal/testutil/` shared package

**SQL Files:**
- Migrations: `/migrations/YYYYMMDDHHMMSS_{description}.sql`
- sqlc queries: `/internal/storage/queries/{domain}.sql` — `documents.sql`, `chunks.sql`, `embeddings.sql`, `workspaces.sql`
- sqlc config: `/sqlc.yaml` at project root

### Format Patterns

**API Response (success):**

```json
{
  "results": [...],
  "total": 42,
  "query_ms": 123
}
```

**API Response (error):**

```json
{
  "error": "workspace_required",
  "message": "A workspace identifier is required."
}
```

**HTTP Status Codes:**
- `200` — success
- `400` — bad input (missing workspace, invalid params)
- `404` — resource not found
- `415` — wrong content type
- `500` — internal error
- `503` — queue full (with `Retry-After` header)

**Date/Time:** ISO 8601 UTC — `"2026-05-23T10:30:00Z"`. Database: `TIMESTAMPTZ`. Go: `time.Time` serialized as RFC 3339.

**Null Handling:** JSON fields are omitted when null (`json:",omitempty"`), not sent as `null`. Exception: explicit `null` for error fields in error responses.

### Process Patterns

**Error Handling Flow:**

1. Storage layer returns domain errors (`ErrNotFound`, `ErrDuplicate`)
2. Service layer wraps with context (`fmt.Errorf("search workspace %s: %w", hash, err)`)
3. HTTP handler maps domain error → HTTP status via Echo `HTTPErrorHandler`
4. MCP adapter maps same domain error → MCP error response
5. zerolog logs at the handler layer only (no double-logging)

**Transaction Pattern:**

```go
err := pgx.BeginTxFunc(ctx, pool, pgx.TxOptions{}, func(tx pgx.Tx) error {
    qtx := queries.WithTx(tx)
    // all writes within tx
    return nil
})
```

**Graceful Shutdown Sequence:**

```
SIGTERM/SIGINT → cancel root context → errgroup goroutines finish current work →
HTTP server drains (5s timeout) → pool.Close() → os.Exit(0)
```

**Workspace Validation (middleware):**
Every handler that touches data MUST go through workspace middleware. The middleware extracts workspace from body (POST) or query string (GET), validates it exists, and stores `workspaceHash` in Echo context. Missing workspace → HTTP 400 immediately.

**Logging Convention:**

```go
log.Info().
    Str("workspace", hash).
    Str("collection", col).
    Int("count", n).
    Dur("elapsed", dur).
    Msg("documents indexed")
```

Levels: `Error` (failures affecting correctness), `Warn` (degraded but functional), `Info` (normal operations), `Debug` (internal state). No `log.Fatal` except in `main()`.

### Enforcement Guidelines

**All AI agents MUST:**

1. Run `golangci-lint` before considering any file complete
2. Include `workspace_hash` in every SQL query that touches document/chunk/embedding tables
3. Use `sqlc`-generated code for database access — no raw `pool.Query()` except in migrations
4. Write `_test.go` for every exported function
5. Use constructor injection — no global variables for dependencies
6. Use `context.Context` as first parameter in all functions that touch I/O

**Anti-Patterns (forbidden):**

- `interface{}` / `any` for typed data — use concrete types or generics
- `panic()` for recoverable errors — return `error`
- `init()` functions — use explicit initialization in `main()`
- Unexported types in public API responses — all JSON-serialized types are exported
- `time.Sleep()` for coordination — use channels, timers, or tickers

---

## Project Structure & Module Boundaries

### Full Directory Tree

```
nano-brain/
├── cmd/
│   └── nanobrain/
│       └── main.go              # Entrypoint: CLI dispatch + server start
├── internal/
│   ├── server/                  # Echo HTTP server, middleware, routes
│   │   ├── server.go            # Server lifecycle (start, shutdown)
│   │   ├── routes.go            # Route registration
│   │   ├── middleware.go         # Workspace extraction, versioning, content-type
│   │   └── handler/             # HTTP handlers grouped by domain
│   │       ├── search.go        # /api/query, /api/search, /api/vsearch
│   │       ├── document.go      # /api/write, /api/read, /api/delete
│   │       ├── admin.go         # /api/status, /api/reload-config, /api/reindex
│   │       └── collection.go    # /api/collections/*
│   ├── mcp/                     # MCP-over-HTTP transport
│   │   ├── adapter.go           # Maps MCP tools → service layer calls
│   │   ├── sse.go               # SSE handler registration
│   │   └── streamable.go        # Streamable HTTP handler registration
│   ├── search/                  # Search engine (hybrid, BM25, vector)
│   │   ├── service.go           # SearchService — orchestrates hybrid pipeline
│   │   ├── bm25.go              # BM25 full-text search via PG tsvector
│   │   ├── vector.go            # Vector similarity search via pgvector
│   │   ├── rrf.go               # Reciprocal Rank Fusion merger
│   │   └── interfaces.go        # Consumer-side interfaces (EmbeddingProvider, etc.)
│   ├── harvest/                 # Session harvester
│   │   ├── harvester.go         # Scan + convert sessions → documents
│   │   ├── opencode.go          # OpenCode JSON session parser
│   │   └── claudecode.go        # Claude Code session parser
│   ├── watcher/                 # File watcher (fsnotify)
│   │   ├── watcher.go           # Watch collection directories for changes
│   │   └── debounce.go          # Debounce logic for rapid file changes
│   ├── embed/                   # Embedding providers + queue
│   │   ├── service.go           # Embedding queue consumer (goroutine)
│   │   ├── ollama.go            # Ollama provider implementation
│   │   └── voyageai.go          # VoyageAI provider implementation
│   ├── chunk/                   # Document chunking
│   │   └── chunker.go           # Token-based chunking (900 tokens, 15% overlap)
│   ├── collection/              # Collection management
│   │   └── service.go           # Add/remove/list/rename collections
│   ├── storage/                 # PostgreSQL repository layer
│   │   ├── pool.go              # pgxpool setup + health check
│   │   ├── queries/             # sqlc query files
│   │   │   ├── documents.sql    # Document CRUD + workspace filtering
│   │   │   ├── chunks.sql       # Chunk CRUD + content-addressed dedup
│   │   │   ├── embeddings.sql   # Vector storage + similarity search
│   │   │   ├── collections.sql  # Collection metadata
│   │   │   └── telemetry.sql    # Search telemetry recording
│   │   └── sqlc/                # sqlc-generated Go code (DO NOT EDIT)
│   ├── migrate/                 # v1→v2 migration tool
│   │   ├── migrate.go           # Migration orchestrator
│   │   └── sqlite_reader.go     # Read v1 SQLite (requires CGO for go-sqlite3)
│   ├── bench/                   # Benchmarking suite
│   │   └── runner.go            # CLI benchmark runner
│   ├── config/                  # Configuration loading
│   │   ├── config.go            # Struct definitions + defaults
│   │   └── loader.go            # YAML + env var loading (koanf)
│   ├── telemetry/               # Search telemetry
│   │   └── recorder.go          # Record query metrics for learning
│   ├── health/                  # Health check
│   │   └── checker.go           # PG ping + embedding provider status
│   └── testutil/                # Shared test helpers
│       └── testdb.go            # Test database setup + cleanup
├── migrations/                  # goose SQL migrations
│   ├── 00001_init_schema.sql
│   └── ...
├── docker/
│   ├── Dockerfile               # Multi-stage (build + distroless/alpine)
│   └── docker-compose.yml       # nano-brain + PostgreSQL 17 + pgvector
├── bench/
│   └── testdata/                # Benchmark fixture data
├── docs/                        # Architecture, PRD, API reference
├── sqlc.yaml                    # sqlc configuration
├── Makefile                     # Build, test, lint, migrate commands
├── .golangci.yml                # Linter configuration
├── go.mod
└── go.sum
```

### Module Boundary Rules

Each `/internal/` package is a self-contained module. Dependencies flow **inward** — outer layers depend on inner, never the reverse.

```
                    ┌─────────────┐
                    │  cmd/main   │  ← wires everything
                    └──────┬──────┘
            ┌──────────────┼──────────────┐
            ▼              ▼              ▼
      ┌──────────┐  ┌──────────┐  ┌──────────┐
      │  server   │  │   mcp    │  │   CLI    │  ← entry points
      └────┬─────┘  └────┬─────┘  └──────────┘
           │              │
           ▼              ▼
      ┌──────────────────────────┐
      │     service layer        │  ← search, harvest, watcher, embed,
      │  (business logic)        │    chunk, collection, bench, telemetry
      └────────────┬─────────────┘
                   ▼
      ┌──────────────────────────┐
      │     storage layer        │  ← storage/ (sqlc), config/, health/
      │  (PostgreSQL + config)   │
      └──────────────────────────┘
```

**Import rules:**
- `server/` → imports `search/`, `harvest/`, `collection/`, etc. (via interfaces)
- `mcp/` → imports same services as `server/` (different transport, same logic)
- `search/` → imports `storage/` (for queries) and defines `EmbeddingProvider` interface
- `embed/` → implements `search.EmbeddingProvider` but does NOT import `search/`
- `storage/` → imports nothing from `/internal/` except `config/`
- `migrate/` → standalone, imports `storage/` for write path + `go-sqlite3` for read path

### Requirement-to-Module Mapping

| Feature Group | FRs | Primary Module | Supporting Modules |
|---|---|---|---|
| Session Harvesting | FR-1–6 | `harvest/` | `chunk/`, `storage/`, `embed/` |
| Hybrid Search | FR-7–15 | `search/` | `storage/`, `embed/`, `telemetry/` |
| Workspace Isolation | FR-16–21 | `server/middleware` | `storage/` (WHERE clause) |
| File Watcher | FR-22–28 | `watcher/` | `chunk/`, `storage/`, `collection/` |
| Embedding Providers | FR-29–36e | `embed/` | `storage/`, `config/` |
| Benchmarking | FR-37–41 | `bench/` | `search/`, `storage/` |
| Corruption Detection | FR-42–47 | `health/` | `storage/` |
| Chunking | FR-48–53 | `chunk/` | (standalone) |
| HTTP API | FR-54–70 | `server/` | all service modules |
| MCP-over-HTTP | FR-71–75 | `mcp/` | all service modules |
| CLI | FR-77–84 | `cmd/nanobrain/` | all service modules |
| Data Migration | FR-85–88 | `migrate/` | `storage/` |
| Configuration | FR-89–93b | `config/` | (standalone) |
| Logging & Telemetry | FR-94–98 | `telemetry/` | `storage/` |
| Docker Deployment | FR-99–105 | `docker/` | (infrastructure) |
| User Collections | (new) | `collection/` | `watcher/`, `storage/`, `config/` |

### Data Flow (Ingestion Pipeline)

```
Session JSON / Markdown file / Collection file
       │
       ▼
  ┌──────────┐    ┌──────────┐    ┌──────────┐    ┌──────────┐
  │ harvest/  │───▶│  chunk/  │───▶│ storage/ │───▶│  embed/  │
  │ watcher/  │    │          │    │ (PG tx)  │    │ (async)  │
  └──────────┘    └──────────┘    └──────────┘    └──────────┘
                                       │                │
                                       ▼                ▼
                                  documents,       embeddings
                                  chunks tables    table (pgvector)
```

1. **harvest/** or **watcher/** detects new/changed file
2. **chunk/** splits into 900-token chunks with 15% overlap
3. **storage/** inserts document + chunks in single PG transaction (content-addressed dedup via SHA-256)
4. **embed/** receives chunk IDs via buffered channel, embeds asynchronously, writes vectors to `embeddings` table

### Data Flow (Query Pipeline)

```
HTTP/MCP request (with workspace_hash)
       │
       ▼
  ┌──────────┐    ┌──────────┐    ┌──────────┐
  │ server/  │───▶│ search/  │───▶│ storage/ │
  │ mcp/     │    │          │    │ (PG)     │
  └──────────┘    └──────────┘    └──────────┘
                       │
                  ┌────┴────┐
                  ▼         ▼
              BM25       Vector
            (tsvector)  (pgvector)
                  │         │
                  └────┬────┘
                       ▼
                   RRF merge
                       │
                       ▼
                  telemetry/
                  (record query)
```

1. **server/** or **mcp/** extracts workspace_hash, validates, calls search service
2. **search/** runs BM25 + vector in parallel, merges via RRF
3. **storage/** executes queries with `WHERE workspace_hash = $1`
4. **telemetry/** records query metrics for learning (async)

---

## Architecture Validation Results

### Coherence Validation ✅

**Decision Compatibility:**
All 14 decisions (D1–D14) are mutually compatible. Key validations:
- D1 (PG schema) + D2 (HNSW pgvector) — both PostgreSQL-native, no conflict.
- D3 (Echo v4) + D5 (MCP SDK `http.Handler`) — Echo mounts standard `http.Handler`, zero friction.
- D6 (errgroup) + D7 (buffered channel) — standard Go concurrency composition.
- D10 (`CGO_ENABLED=0`) + D12 (migration needs SQLite reader) — resolved: use `modernc.org/sqlite` (pure Go) instead of `go-sqlite3` (CGO). Entire project stays `CGO_ENABLED=0`.
- D14 (dev-in-container) replaces testcontainers-go — library table updated accordingly.

**Pattern Consistency:**
- Database naming (snake_case), Go naming (PascalCase/camelCase), API naming (snake_case) — consistent throughout.
- All database access via sqlc-generated code — no exceptions.
- All I/O functions take `context.Context` as first parameter — enforced.
- Error handling follows single path: domain error → handler mapping → structured JSON response.

**Structure Alignment:**
- 14 `/internal/` packages map to 16 feature groups (some packages serve multiple groups).
- Module boundary rules (inward dependency flow) are compatible with all decisions.
- `collection/` package added for D13 (user collections). `migrate/` exists for D12 (hybrid migration).

### Requirements Coverage ✅

**Functional Requirements (111 FRs):**
All 15 PRD feature groups + 1 new group (User Collections, D13) have explicit module assignments. No FR is orphaned.

| Feature Group | FRs | Module | Covered |
|---|---|---|---|
| Session Harvesting | FR-1–6 | `harvest/` | ✅ |
| Hybrid Search | FR-7–15 | `search/` | ✅ |
| Workspace Isolation | FR-16–21 | `server/middleware` | ✅ |
| File Watcher | FR-22–28 | `watcher/` | ✅ |
| Embedding Providers | FR-29–36e | `embed/` | ✅ |
| Benchmarking | FR-37–41 | `bench/` | ✅ |
| Corruption Detection | FR-42–47 | `health/` | ✅ |
| Chunking | FR-48–53 | `chunk/` | ✅ |
| HTTP API | FR-54–70 | `server/` | ✅ |
| MCP-over-HTTP | FR-71–75 | `mcp/` | ✅ |
| CLI | FR-77–84 | `cmd/nanobrain/` | ✅ |
| Data Migration | FR-85–88 | `migrate/` | ✅ |
| Configuration | FR-89–93b | `config/` | ✅ |
| Logging & Telemetry | FR-94–98 | `telemetry/` | ✅ |
| Docker Deployment | FR-99–105 | `docker/` | ✅ |
| User Collections | D13 | `collection/` | ✅ |

**Non-Functional Requirements (5 sections):**
- Concurrency safety (§8.1) → PG MVCC (D1), errgroup (D6), `go test -race` (D11) ✅
- Workspace isolation (§8.2) → middleware + WHERE clause + HTTP 400 on missing ✅
- Search quality (§8.3) → bench/ module with P@5, R@10, MRR metrics ✅
- Data integrity (§8.4) → atomic transactions, idempotent upserts, goose migrations ✅
- Privacy (§8.5) → Ollama default, no external telemetry ✅

### Implementation Readiness ✅

**Decision Completeness:**
- 14 decisions documented with rationale, alternatives considered, and trade-offs.
- Library versions locked (pgx v5, Echo v4, sqlc, fsnotify v1, koanf v2, zerolog, goose v3, MCP SDK v1.5+).
- Code examples provided for: transaction pattern, error handling flow, graceful shutdown, logging convention, workspace validation middleware.

**Structure Completeness:**
- Full directory tree with every file and its purpose.
- Module boundary diagram with import rules.
- Data flow diagrams for both ingestion and query pipelines.

**Pattern Completeness:**
- Naming conventions for database, API, and Go code — comprehensive.
- Process patterns: error handling, transactions, shutdown, workspace validation, logging.
- Enforcement guidelines: 6 MUST rules + 5 forbidden anti-patterns.

### Gap Analysis

**Critical Gaps: NONE**

**Minor Gaps (non-blocking):**

1. **Pure Go SQLite for migration** — Use `modernc.org/sqlite` instead of `go-sqlite3` to avoid CGO dependency. Keeps entire project `CGO_ENABLED=0`. Documented in D12 resolution above.

2. **Collection FR numbers** — D13 (User Collections) is a new Tier 1 feature without explicit FR numbers in the PRD. Collection management is referenced in PRD CLI table (§4.11) but lacks dedicated FR section. Non-blocking: architecture covers it; PRD can be updated during epic derivation.

3. **`nano-brain init` module mapping** — FR-76 touches multiple modules (`cmd/nanobrain/` + `collection/` + `storage/`). This is normal for cross-cutting CLI commands and is handled by the wiring in `main()`.

### Architecture Completeness Checklist

**Requirements Analysis**
- [x] Project context thoroughly analyzed
- [x] Scale and complexity assessed
- [x] Technical constraints identified
- [x] Cross-cutting concerns mapped

**Architectural Decisions**
- [x] Critical decisions documented with versions
- [x] Technology stack fully specified
- [x] Integration patterns defined
- [x] Performance considerations addressed

**Implementation Patterns**
- [x] Naming conventions established
- [x] Structure patterns defined
- [x] Communication patterns specified
- [x] Process patterns documented

**Project Structure**
- [x] Complete directory structure defined
- [x] Component boundaries established
- [x] Integration points mapped
- [x] Requirements to structure mapping complete

### Architecture Readiness Assessment

**Overall Status:** READY FOR IMPLEMENTATION

**Confidence Level:** High — all 16 checklist items pass, no critical gaps, all 111 FRs mapped to modules.

**Key Strengths:**
- PostgreSQL MVCC eliminates the entire class of v1 SQLite corruption failures.
- Workspace isolation enforced at architecture level (middleware + WHERE clause), not application logic.
- Dev-in-container topology matches production deployment — test what you ship.
- Consumer-side interfaces enable independent module development and testing.
- Hybrid migration preserves irreplaceable user data while regenerating derivable artifacts.

**Areas for Future Enhancement:**
- Tier 2 features (code intelligence, knowledge graph, self-learning, consolidation) have clean interface boundaries reserved but no detailed design yet.
- PgBouncer for connection pooling in multi-container deployments — evaluate during load testing.
- Log rotation library selection (lumberjack or similar) — decide during implementation.

### Implementation Handoff

**AI Agent Guidelines:**
- Follow all 14 architectural decisions exactly as documented.
- Use implementation patterns consistently across all components.
- Respect module boundaries and import rules — no circular dependencies.
- Every SQL query touching document/chunk/embedding tables MUST include `WHERE workspace_hash = $1`.
- Run `golangci-lint` and `go test -race` before marking any file complete.
- Refer to this document for all architectural questions.

**First Implementation Priority:**
1. `go mod init` + project skeleton (cmd/, internal/ directories)
2. D1 → database schema + goose migration files
3. D3 + D4 → Echo server skeleton with workspace middleware
4. D8 + D9 → component interfaces and constructor wiring in main()
5. Storage layer (sqlc) → search service → HTTP handlers → MCP adapter
6. Background jobs (harvester, watcher, embedder) → Docker → benchmarking
