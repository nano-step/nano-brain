# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

nano-brain — persistent memory and code intelligence server for AI coding agents. Go 1.23, PostgreSQL 17 + pgvector, single static binary (`CGO_ENABLED=0`). Hybrid search (BM25 + vector + RRF fusion + recency decay). 9 MCP tools, REST API, CLI.

## Commands

```bash
# Build
CGO_ENABLED=0 go build -o nano-brain ./cmd/nano-brain

# Test (unit — fast, no DB required)
go test -race -short ./...

# Test (integration — requires PostgreSQL + pgvector)
go test -race -tags=integration ./...

# Run tests for a single package
go test -race -short ./internal/search/...

# Start server
DATABASE_URL="postgres://nanobrain:nanobrain@localhost:5432/nanobrain_dev" ./nano-brain

# SQL codegen (after editing queries/*.sql)
sqlc generate

# Database migrations
go run ./cmd/nano-brain db:migrate
```

## Architecture

- `cmd/nano-brain/` — CLI entry + server startup. Custom dispatcher (no cobra). See `cmd/nano-brain/AGENTS.md`
- `internal/server/` — Echo v4 HTTP server, middleware (request logging, version header, workspace extraction)
- `internal/server/handlers/` — 34 handler files, one per endpoint group. See `handlers/AGENTS.md`
- `internal/storage/` — PostgreSQL pool + sqlc-generated queries (DO NOT EDIT `sqlc/` files). See `storage/AGENTS.md`
- `internal/search/` — Hybrid search: BM25 + vector + RRF fusion + recency decay. See `search/AGENTS.md`
- `internal/embed/` — Embedding queue worker, Ollama/Voyage providers. See `embed/AGENTS.md`
- `internal/harvest/` — Session harvesting from OpenCode/Claude Code SQLite DBs. See `harvest/AGENTS.md`
- `internal/mcp/` — MCP protocol server, 9 tools. Largest file: `tools.go` (1206 lines). See `mcp/AGENTS.md`
- `internal/config/` — koanf YAML + env config with hot-reload
- `internal/watcher/` — fsnotify file watching with debounce
- `internal/summarize/` — LLM-powered session summarization (map-reduce)

## Key Conventions

- **DI pattern**: constructor injection — config, logger, DB pool passed as params. No globals, no DI framework.
- **Errors**: `fmt.Errorf("<context>: %w", err)` — no custom error types. Always wrap.
- **Logging**: zerolog structured. Scoped: `logger.With().Str("component","x").Logger()` at construction.
- **Context**: `ctx context.Context` first param on all I/O ops. `errgroup` for goroutine lifecycle.
- **Interfaces**: small, role-based, consumer-side (`Embedder`, `Querier`, `Harvester`).
- **DB access**: `storage.NewPool()` → `*pgxpool.Pool` → `sqlc.Queries`. All SQL in `queries/*.sql`, codegen via `sqlc generate`.
- **Tests**: `package <name>_test` (external). Inline struct mocks. Table-driven. `testutil.SetupTestDB(t)` for integration tests with isolated PG schemas.
- **Config**: YAML (`~/.nano-brain/config.yml`) + env (`NANO_BRAIN_*`). Hot-reload via `POST /api/reload-config`.

## Git & PR Conventions

- No agent footers (`Co-Authored-By`, `Generated-with`) in commits or PRs
- Create a GitHub issue before starting any task
- Conventional commits preferred
