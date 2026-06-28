# Technology Stack — nano-brain

Last updated: 2026-06-28

## Runtime

| Component | Version | Notes |
|-----------|---------|-------|
| Go | 1.23 | `CGO_ENABLED=0` static binary |
| PostgreSQL | 17 | Primary data store |
| pgvector | 0.8.2 | Vector similarity (HNSW indexing) |
| Node.js | 24 | npm package only (postinstall, CLI shim) |
| Alpine Linux | 3.21 | Docker base image |

## Languages

- **Go 1.23** — 100% of application logic (cmd/, internal/)
- **SQL** — 27 goose migrations + sqlc queries
- **YAML** — Config files, GitHub Actions workflows
- **JavaScript** — npm shim only (npm/run.js, npm/postinstall.js) — not application code

## Frameworks & Libraries

### HTTP / API

| Library | Purpose |
|---------|---------|
| `labstack/echo/v4` | HTTP framework, routing, middleware |
| `modelcontextprotocol/go-sdk` v0.8.0 | MCP protocol server (streamable HTTP + SSE) |

### Database

| Library | Purpose |
|---------|---------|
| `jackc/pgx/v5` v5.7.2 | PostgreSQL driver + connection pool |
| `pgvector/pgvector-go` v0.2.2 | pgvector type support for Go |
| `sqlc-dev/pqtype` v0.3.0 | sqlc JSON/array type mappings |
| `lib/pq` v1.12.3 | PostgreSQL driver (compat) |
| `pressly/goose/v3` v3.22.1 | Database migrations |
| `sqlc` v2 | Type-safe SQL code generation |

### Configuration

| Library | Purpose |
|---------|---------|
| `knadh/koanf/v2` v2.3.4 | YAML + env config loading |
| `knadh/koanf/parsers/yaml` | YAML parser for koanf |
| `knadh/koanf/providers/file` | File-based config provider |
| `knadh/koanf/providers/structs` | Default struct config provider |

### Logging & Observability

| Library | Purpose |
|---------|---------|
| `rs/zerolog` v1.35.1 | Structured JSON logging |
| `natefinch/lumberjack` v2.0.0 | Log file rotation |
| `internal/telemetry` | Search telemetry (embedded) |

### Caching & Data Structures

| Library | Purpose |
|---------|---------|
| `hashicorp/golang-lru/v2` v2.0.7 | LRU cache (PageRank, search) |
| `google/uuid` v1.6.0 | UUID generation |
| `sabhiram/go-gitignore` | .gitignore pattern matching |
| `odvcencio/gotreesitter` v0.19.1 | Tree-sitter AST parsing |

### Search & AI

| Library | Purpose |
|---------|---------|
| `modernc.org/sqlite` v1.38.2 | Pure-Go SQLite (OpenCode session DB) |
| `golang.org/x/sync` v0.15.0 | errgroup for parallel operations |
| `golang.org/x/time` v0.9.0 | Rate limiting (LLM calls) |
| `golang.org/x/crypto` v0.31.0 | bcrypt (auth password hashing) |

### Testing

| Library | Purpose |
|---------|---------|
| `stretchr/testify` v1.9.0 | Test assertions + require |

## Code Generation

| Tool | Config | Output |
|------|--------|--------|
| `sqlc` | `sqlc.yaml` | `internal/storage/sqlc/` — type-safe query wrappers |
| `goose` | `migrations/*.sql` | Schema versioning (27 migrations) |

## Build System

| Tool | Purpose |
|------|---------|
| `Makefile` | `build`, `lint`, `test`, `test-integration`, `test-e2e` |
| `golangci-lint` | Static analysis (errcheck, govet, staticcheck, unused) |
| `Dockerfile` | Multi-stage build (golang:1.23-alpine → alpine:3.21) |
| `docker-compose.yml` | 3-service stack (postgres + ollama + nano-brain) |

## Configuration

### Config file
- Default: `~/.nano-brain/config.yml`
- Override: `--config` flag or `NANO_BRAIN_CONFIG` env var

### Config sections

| Section | Key | Default |
|---------|-----|---------|
| `server` | `host:port` | `localhost:3100` |
| `database` | `url` | `postgres://nanobrain:nanobrain@localhost:5432/nanobrain_dev` |
| `embedding` | `provider,url,model` | `ollama, http://localhost:11434, nomic-embed-text` |
| `search` | `rrf_k, recency_weight, limit` | `60, 0.3, 20` |
| `watcher` | `debounce_ms, reindex_interval` | `2000, 300` |
| `summarization` | `enabled, provider_url, model` | disabled |
| `code_summarization` | `enabled, provider_url, model` | disabled |
| `flow` | `enabled, max_depth` | disabled, 10 |
| `intelligence` | `enabled, model` | disabled, claude-sonnet-4-5 |

### Environment variables

| Variable | Maps to | Notes |
|----------|---------|-------|
| `NANO_BRAIN_CONFIG` | config file path | |
| `NANO_BRAIN_SERVER_PORT` | `server.port` | |
| `NANO_BRAIN_DATABASE_URL` | `database.url` | |
| `DATABASE_URL` | `database.url` | Postgres convention |
| `VOYAGE_API_KEY` | `embedding.voyage_api_key` | |
| `OPENCODE_STORAGE_DIR` | `harvester.opencode.session_dir` | |
| `OPENCODE_DB_PATH` | `harvester.opencode.db_path` | |
| `OPENCODE_DB_ROOT` | `harvester.opencode.db_root` | |

## Docker Stack

```yaml
services:
  postgres:   pgvector/pgvector:pg17  (port 5432)
  ollama:     ollama/ollama:latest    (port 11434)
  nano-brain: ./Dockerfile            (port 3100)
```

## npm Package

- Name: `@nano-step/nano-brain`
- Purpose: Distribute pre-built Go binary via npm
- Dependencies: `@nano-step/oh-my-harness` (dev workflow only)
- Binary shim: `npm/run.js` → spawns Go binary

## Database Schema

27 goose migrations covering:
- Documents + chunks + embeddings (HNSW vector index)
- BM25 full-text search (tsvector/tsquery)
- Knowledge graph (nodes, edges, types)
- Code symbols + call graph
- Flow charts + execution flows
- Search telemetry
- Code summarization usage/failures
- Auto-summarization

## Internal Packages (17)

| Package | Responsibility |
|---------|---------------|
| `cmd/nano-brain` | CLI dispatcher, daemon startup, 53+ command files |
| `internal/server` | HTTP server, routes, middleware |
| `internal/storage` | PostgreSQL pool, sqlc, migrations |
| `internal/search` | Hybrid search pipeline (BM25 + vector + RRF) |
| `internal/embed` | Embedding queue (Ollama, VoyageAI) |
| `internal/mcp` | MCP protocol server, 16 tools |
| `internal/graph` | Code intelligence (15+ language extractors) |
| `internal/symbol` | Symbol extraction (Go, JS, TS, Python, Ruby) |
| `internal/flow` | Execution flow visualization |
| `internal/harvest` | Session harvesting (OpenCode, Claude Code, Git) |
| `internal/summarize` | LLM session summarization |
| `internal/codesummarize` | Batched LLM code symbol summarization |
| `internal/intelligence` | Memory consolidation + categorization |
| `internal/watcher` | File system watching + incremental indexing |
| `internal/chunker` | Document chunking |
| `internal/chunk` | Chunk types |
| `internal/config` | Configuration management |
| `internal/eventbus` | Internal event bus |
| `internal/telemetry` | Search telemetry |
| `internal/timefilter` | Time range parsing |
| `internal/links` | Link resolution |
| `internal/bench` | Benchmark harness |
| `internal/testutil` | Test utilities |
