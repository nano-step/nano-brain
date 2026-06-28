# STRUCTURE.md — nano-brain

**Last updated:** 2026-06-28

---

## Top-Level Layout

```
nano-brain/
├── cmd/nano-brain/          # CLI dispatcher + server entry point
├── internal/                # 24 packages — all application logic
├── migrations/              # Goose SQL migrations (embedded)
├── benchmarks/              # Capability benchmarks per workspace
├── sqlc.yaml                # sqlc code generation config
├── go.mod / go.sum          # Go 1.23 module
├── Dockerfile               # Multi-stage build
├── docker-compose.yml       # Postgres + nano-brain
├── config.test.yml          # Test config (port 3199)
├── Makefile                 # Build/test targets
├── npm/                     # npm package wrapper
├── scripts/                 # Helper scripts
├── skills/                  # OpenCode skill definitions
├── openspec/                # OpenSpec change proposals
├── docs/                    # User-facing documentation
├── testdata/                # Test fixtures
└── test/                    # Additional test resources
```

---

## `cmd/nano-brain/` — CLI Layer

Custom dispatcher (no cobra). `main.go` switches on `os.Args[1]`.

| File | Purpose |
|------|---------|
| `main.go` | Entry point: flag parsing, command dispatch, `startServer()` |
| `commands.go` | `runWriteCmd`, `runQueryCmd`, `runSearchCmd`, `runVSearchCmd`, `runReindexCmd`, `runHarvestCmd`, `runInitCmd` |
| `ops.go` | Server-side ops: `runLogsCmd`, `runStatusCmd`, `runVersionCmd`, `runDockerCmd`, `runBenchCmd`, `runDBMigrateCmd` |
| `daemon.go` | `runServeCmd`, `runServeDaemon`, `runStopCmd`, `runRestartCmd`; PID-file lifecycle |
| `guard.go` | Single-instance guard, container detection |
| `client.go` | HTTP client: `doRequest`, `sendRequest`, `getBaseURL` |
| `collection.go` | `runCollectionCmd` — add/remove/list collections |
| `workspaces.go` | `runWorkspacesCmd` — list workspaces |
| `init.go` | `runInteractiveInit` — guided workspace registration wizard |
| `config_cmd.go` | `runConfigCmd` — show/validate config |
| `doctor.go` | `runDoctorCmd` — prerequisite checks (PG, pgvector, Ollama) |
| `detect.go` | Auto-detect OpenCode/Claude Code storage directories |
| `migrate.go` | `runDBMigrateCmd` — goose migrations + V1 SQLite import |
| `bench.go` | `runBenchCmd` — generate/run/compare benchmarks |

---

## `internal/` — Application Packages

### Core Infrastructure

| Package | Purpose |
|---------|---------|
| `config/` | YAML + env configuration (koanf). `Config` struct with 14 sub-configs. Hot-reload via `POST /api/reload-config`. |
| `storage/` | PostgreSQL pool (pgxpool), goose migrations, sqlc query generation |
| `storage/sqlc/` | **Generated** type-safe Go from SQL. Never edit directly. |
| `storage/queries/` | Raw SQL files (input to sqlc). 17 query files. |
| `eventbus/` | Async pub/sub bus. Fan-out notifications between components. |
| `health/` | Health checks, doctor prerequisite validation, logger setup |
| `telemetry/` | Query logging, latency tracking, retention cleanup |
| `testutil/` | `SetupTestDB(t)` — isolated PG schema per test |

### Business Logic

| Package | Purpose |
|---------|---------|
| `search/` | Hybrid search pipeline: BM25 + vector + RRF + recency + PageRank + entity boost. Optional HyDE, reranking, query preprocessing. |
| `graph/` | Code intelligence: 15+ language/framework extractors (Go, TS, JS, Python, Ruby, Echo, Express, Gin, Rails, NestJS, Nuxt). PageRank. CFG extraction. |
| `symbol/` | Code symbol extraction: functions, types, interfaces, constants. 5 language extractors. |
| `harvest/` | Session harvesting from OpenCode (SQLite/JSON) and Claude Code (JSONL). Dedup, tagging, auto-memory. |
| `embed/` | Embedding queue + provider adapters (Ollama, VoyageAI). Bounded worker pool. |
| `flow/` | Execution flow materializer. Builds call chains, Mermaid diagrams, sequence diagrams. |
| `summarize/` | LLM session summarization. Map-reduce pipeline, disk persistence, harvest adapter. |
| `codesummarize/` | Batched code symbol summarization. Budget tracking, retry, cascade, graph context. |
| `intelligence/` | Memory consolidation and LLM-based categorization. |
| `chunker/` | Document chunking strategies: symbol-aware, heading-based, fixed-size. Dispatcher selects strategy. |
| `chunk/` | Chunk data model. |
| `links/` | Document link resolution and extraction (backlinks, forward links). |
| `watcher/` | File system watcher (fsnotify). Debounced, filtered, triggers re-indexing. |
| `timefilter/` | Time range parsing for query filtering. |

### Transport

| Package | Purpose |
|---------|---------|
| `mcp/` | MCP protocol server. 16 tools. Streamable HTTP + SSE transport. |
| `server/` | Echo v4 HTTP server. Routes, middleware, handler registration. |
| `server/handlers/` | 76 handler files. One per endpoint group. Constructor pattern with interface deps. |
| `server/middleware/` | Auth, workspace resolution, CSRF protection. |
| `bench/` | Benchmark framework: generate, run, compare, stress test. |
| `migrate/` | V1 SQLite import (migration from v1 schema). |

---

## Database Schema

### Tables (from migrations)

| Table | Purpose |
|-------|---------|
| `workspaces` | Registered project roots (hash, name, path) |
| `collections` | Named paths within workspaces (glob, exclude, extensions) |
| `documents` | Source file metadata + content (versioned via `supersedes_id`) |
| `chunks` | Sub-document units (symbol-aware, heading, or fixed) |
| `embeddings` | Vector representations (provider, model, vector) |
| `graph_edges` | Directed edges: `source_node → target_node` with type |
| `pagerank_scores` | Pre-computed PageRank importance scores |
| `function_flowcharts` | Pre-materialized control-flow graphs |
| `chunk_entities` | Entity mentions within chunks |
| `telemetry_logs` | Query logging and metrics |
| `code_summarization_usage` | Daily API usage tracking |
| `code_summarization_failures` | Failed summarization attempts |

### Key Relationships

```
workspaces 1──* collections 1──* documents 1──* chunks 1──* embeddings
     │                          │
     └──* graph_edges           └──* chunk_entities
     └──* pagerank_scores
     └──* function_flowcharts
     └──* telemetry_logs
```

---

## Naming Conventions

| Element | Convention | Example |
|---------|-----------|---------|
| Go packages | lowercase, single word | `search`, `graph`, `embed` |
| Go files | snake_case | `opencode_sqlite.go`, `pagerank_loader.go` |
| Go types | PascalCase | `SearchService`, `GraphEdge`, `EmbedQueue` |
| Go interfaces | Role-based, small | `Embedder`, `Querier`, `Harvester`, `Publisher` |
| SQL tables | snake_case, plural | `documents`, `graph_edges`, `embeddings` |
| SQL queries | PascalCase, descriptive | `GetDocumentByPath`, `ListGraphEdges` |
| Config keys | snake_case | `session_poll`, `max_depth`, `rrf_k` |
| HTTP routes | kebab-case | `/api/v1/multi-get`, `/api/v1/reload-config` |
| MCP tool names | snake_case, prefixed | `memory_query`, `memory_impact`, `memory_trace` |
| CLI commands | kebab-case | `backfill-summaries`, `detect-changes`, `code-impact` |
| Env vars | SCREAMING_SNAKE | `NANO_BRAIN_SERVER_PORT`, `DATABASE_URL` |
| Migrations | Sequential zero-padded | `00001_initial_schema.sql` → `00027_reconcile_edge_type.sql` |

---

## Key File Locations

| What | Where |
|------|-------|
| Server startup | `cmd/nano-brain/main.go:229` (`startServer()`) |
| Route registration | `internal/server/routes.go:14` (`registerRoutes()`) |
| MCP tool registration | `internal/mcp/tools.go:28` (`RegisterTools()`) |
| Config loading | `internal/config/config.go:294` (`Load()`) |
| Pool creation | `internal/storage/pool.go` (`NewPool()`) |
| Migration runner | `internal/storage/migrate.go` (`RunMigrations()`) |
| Workspace hash | `internal/storage/workspace.go` (SHA-256 of root path) |
| Search pipeline | `internal/search/service.go` (`SearchService`) |
| Graph registry | `internal/graph/registry.go` (`Registry`) |
| Symbol registry | `internal/symbol/symbol.go` (`Registry`) |
| Watcher main loop | `internal/watcher/watcher.go` (`Run()`) |
| Embed queue worker | `internal/embed/queue.go` (`Run()`) |
| Harvester runner | `internal/harvest/runner.go` (`Run()`) |
| Flow materializer | `internal/flow/materializer.go` (`Materializer`) |
| Code summarizer | `internal/codesummarize/service.go` (`Service`) |

---

## Configuration

Default path: `~/.nano-brain/config.yml`

Precedence: `--config` flag > `NANO_BRAIN_CONFIG` env > default path > built-in defaults.

| Section | Key Fields |
|---------|-----------|
| `server` | `host`, `port`, `auth.enabled`, `serve_only` |
| `database` | `url` (PostgreSQL DSN) |
| `embedding` | `provider`, `url`, `model`, `dimension`, `concurrency`, `max_chars` |
| `search` | `rrf_k`, `recency_weight`, `limit`, `pagerank_enabled`, `hyde.enabled`, `reranking.enabled` |
| `harvester` | `opencode.session_dir`, `opencode.db_path`, `opencode.db_root`, `claudecode.session_dir` |
| `watcher` | `debounce_ms`, `reindex_interval`, `exclude_patterns`, `allowed_extensions` |
| `summarization` | `enabled`, `provider_url`, `model`, `concurrency`, `write_to_disk` |
| `code_summarization` | `enabled`, `provider_url`, `model`, `batch_size`, `workers` |
| `flow` | `enabled`, `max_depth`, `max_fanout`, `summary_enabled` |
| `intelligence` | `enabled`, `provider_url`, `model`, `consolidation_age` |

---

## Build & Test

```bash
# Build
CGO_ENABLED=0 go build -o nano-brain ./cmd/nano-brain

# Quick test
go build ./... && go test -race -short ./...

# Integration tests
go test -race -tags=integration ./...

# Lint
golangci-lint run

# sqlc codegen
sqlc generate

# Migrations
./nano-brain db:migrate
```
