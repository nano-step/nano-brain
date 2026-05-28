# nano-brain

**Persistent memory and code intelligence for AI coding agents.**

[![Go 1.23](https://img.shields.io/badge/Go-1.23-00ADD8?logo=go)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![GitHub](https://img.shields.io/badge/GitHub-nano--step%2Fnano--brain-181717?logo=github)](https://github.com/nano-step/nano-brain)

## What It Does

nano-brain is a persistent memory server for AI coding agents that solves session amnesia. It automatically ingests AI sessions, notes, and codebase files, indexes everything with hybrid search (BM25 + pgvector), and serves memories via MCP tools and REST API. Built in Go with PostgreSQL — single static binary, zero CGO dependencies.

## Key Features

- **Hybrid search** — BM25 full-text + pgvector HNSW cosine similarity + RRF fusion + recency decay
- **9 MCP tools** — query, search, vsearch, get, write, tags, status, update, wake_up
- **Session harvesting** — auto-ingest OpenCode and Claude Code sessions
- **File watcher** — fsnotify-based directory monitoring with debounce
- **Content-addressed storage** — SHA-256 deduplication
- **Heading-aware markdown chunking**
- **Multi-workspace isolation** with per-workspace data
- **Config hot-reload** — `POST /api/reload-config`
- **V1 migration** — import from SQLite (pure Go, no CGO)
- **Benchmarking suite** — generate, run, compare, stress
- **Search telemetry** — local-only, 90-day retention, non-blocking

## Prerequisites

- **Go 1.23+** (building from source) OR pre-built binary
- **PostgreSQL 17** with **pgvector 0.8.2** extension
- **Embedding provider:** Ollama (default, local) or Voyage AI

## Quick Start

### Option A: Via npx (no Go required)

```bash
# Start PostgreSQL + pgvector
docker run -d --name nanobrain-pg -p 5432:5432 \
  -e POSTGRES_USER=nanobrain -e POSTGRES_PASSWORD=nanobrain -e POSTGRES_DB=nanobrain_dev \
  pgvector/pgvector:pg17

# Start Ollama + pull embedding model
ollama pull nomic-embed-text

# Check prerequisites
npx @nano-step/nano-brain@beta doctor

# Start server
npx @nano-step/nano-brain@beta
```

> **Also available as:** `npx nano-brain@beta` (unscoped alias)
>
> **Note:** Do NOT run `npx nano-brain` from the nano-brain source directory — npm will resolve the local package instead of the registry. Run from any other directory.

### Option B: Build from source

```bash
# Build
CGO_ENABLED=0 go build -o nano-brain ./cmd/nano-brain

# Start PostgreSQL + pgvector (example with Docker)
docker run -d --name nanobrain-pg -p 5432:5432 \
  -e POSTGRES_USER=nanobrain -e POSTGRES_PASSWORD=nanobrain -e POSTGRES_DB=nanobrain_dev \
  pgvector/pgvector:pg17

# Start server
DATABASE_URL="postgres://nanobrain:nanobrain@localhost:5432/nanobrain_dev" ./nano-brain

# Register workspace
curl -X POST http://localhost:3100/api/v1/init \
  -H "Content-Type: application/json" \
  -d '{"root_path":"/path/to/project","name":"my-project"}'

# Write a document
curl -X POST http://localhost:3100/api/v1/write \
  -H "Content-Type: application/json" \
  -d '{"workspace":"<hash>","source_path":"notes/decision.md","content":"# Decision\nUse PostgreSQL.","tags":["decision"]}'

# Search
curl -X POST http://localhost:3100/api/v1/query \
  -H "Content-Type: application/json" \
  -d '{"workspace":"<hash>","query":"database decision"}'
```

## Configuration

Config file: `~/.nano-brain/config.yml`

```yaml
server:
  host: localhost
  port: 3100

database:
  url: postgres://nanobrain:nanobrain@localhost:5432/nanobrain_dev

embedding:
  provider: ollama              # ollama or voyage
  url: http://localhost:11434
  model: nomic-embed-text
  dimension: 0                  # auto-detect from provider
  concurrency: 3

search:
  rrf_k: 60
  recency_weight: 0.3
  recency_half_life_days: 180
  limit: 20

harvester:
  opencode:
    session_dir: ""             # e.g., ~/.local/share/opencode/storage
  claudecode:
    enabled: false
    session_dir: ""

watcher:
  debounce_ms: 2000
  reindex_interval: 300

storage:
  max_file_size: 314572800      # 300MB
  max_size: 10737418240         # 10GB

telemetry:
  retention_days: 90

logging:
  level: info
  file: ""                      # empty = stdout only

summarization:
  enabled: false                # set to true to generate LLM summaries of harvested sessions
  provider_url: ""              # OpenAI-compatible endpoint, e.g. https://ai-proxy.example.com/v1
  api_key: ""                   # or set NANO_BRAIN_SUMMARIZE_API_KEY env var
  model: "nano-brain"           # model name passed to the provider
  max_tokens: 4096              # max tokens per LLM completion
  concurrency: 3                # parallel map-phase LLM calls
  output_dir: "~/.nano-brain/summaries"  # directory for .md summary files
```

### Session Summarization

When `summarization.enabled: true`, nano-brain automatically generates structured markdown summaries of each harvested session using an OpenAI-compatible LLM provider. Summaries are:

- Written as `.md` files to `output_dir` (`{source}_{title-slug}_{YYYY-MM-DD}.md`)
- Stored in the vector DB under collection `session-summary` for semantic search
- Idempotent — unchanged sessions are skipped; re-harvested sessions overwrite old summaries

**Quick setup with ai-proxy:**

```yaml
summarization:
  enabled: true
  provider_url: "https://ai-proxy.example.com/v1"
  api_key: ""           # set NANO_BRAIN_SUMMARIZE_API_KEY instead
  model: "claude-sonnet-4-5"
  max_tokens: 4096
  concurrency: 3
  output_dir: "~/.nano-brain/summaries"
```

Or via environment variable:

```bash
export NANO_BRAIN_SUMMARIZE_API_KEY="sk-..."
```

Large sessions (100K+ tokens) are handled via map-reduce chunking — no session is too large.

### Environment Variables

| Variable | Description |
|----------|-------------|
| `DATABASE_URL` | PostgreSQL connection string |
| `VOYAGE_API_KEY` | Voyage AI API key |
| `OPENCODE_STORAGE_DIR` | OpenCode session directory |
| `NANO_BRAIN_SUMMARIZE_API_KEY` | API key for the summarization LLM provider |
| `NANO_BRAIN_*` | Override any config (e.g., `NANO_BRAIN_SERVER_PORT=3100`) |

## REST API

### Public Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| GET | `/api/status` | Server status with version, uptime, workspace stats |
| POST | `/api/v1/init` | Register workspace |
| GET | `/api/v1/workspaces` | List all workspaces (with doc counts) |
| GET | `/api/v1/wake-up` | Workspace briefing |
| POST | `/api/harvest` | Trigger session harvesting |
| POST | `/api/reload-config` | Hot-reload configuration |

### Workspace-Scoped Endpoints

Workspace is passed in the JSON body for POST, query param for GET.

| Method | Path | Description |
|--------|------|-------------|
| POST | `/api/v1/write` | Write/update document |
| POST | `/api/v1/embed` | Trigger embedding |
| POST | `/api/v1/search` | BM25 keyword search |
| POST | `/api/v1/vsearch` | Vector similarity search |
| POST | `/api/v1/query` | Hybrid search (BM25 + vector + RRF + recency) |
| POST | `/api/v1/collections` | Add collection |
| GET | `/api/v1/collections` | List collections |
| PUT | `/api/v1/collections/:name` | Rename collection |
| DELETE | `/api/v1/collections/:name` | Remove collection |
| GET | `/api/v1/tags` | List tags with counts |
| POST | `/api/v1/reindex` | Queue reindex (202) |
| POST | `/api/v1/update` | Queue update (202) |
| POST | `/api/v1/summarize` | Trigger LLM summarization of harvested sessions |
| POST | `/api/v1/wake-up` | Workspace briefing with session_dir |

### MCP Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET/POST | `/mcp` | Streamable HTTP (MCP 2025-03-26) |
| GET/POST | `/sse` | SSE transport (legacy) |

## CLI Commands

| Command | Description |
|---------|-------------|
| `nano-brain` (no args) | Start HTTP server (default: port 3100) |
| `nano-brain init --root=<path>` | Register workspace |
| `nano-brain write` | Write document via CLI |
| `nano-brain query` | Hybrid search |
| `nano-brain search` | BM25 keyword search |
| `nano-brain vsearch` | Vector similarity search |
| `nano-brain collection add\|remove\|list` | Manage collections |
| `nano-brain harvest` | Trigger session harvesting |
| `nano-brain bench generate\|run\|compare\|stress` | Benchmarking suite |
| `nano-brain db:migrate` | Run pending goose migrations |
| `nano-brain db:migrate --from-v1 <path>` | Import V1 SQLite data |
| `nano-brain logs [-n 50] [-f]` | Tail log file |
| `nano-brain docker start\|stop\|status` | Docker compose management |
| `nano-brain status [--json]` | Server status |
| `nano-brain doctor [--json]` | Check prerequisites (config, PostgreSQL, pgvector, Ollama, model) |

## MCP Tools

nano-brain exposes 9 tools via MCP (Model Context Protocol):

| Tool | Description |
|------|-------------|
| `memory_query` | Hybrid search (BM25 + vector + RRF + recency) |
| `memory_search` | BM25 keyword search |
| `memory_vsearch` | Vector similarity search |
| `memory_get` | Get document by path |
| `memory_write` | Write/update document |
| `memory_tags` | List tags with counts |
| `memory_status` | Server and embedding status |
| `memory_update` | Trigger re-embedding |
| `memory_wake_up` | Workspace briefing |

### MCP Configuration

```json
{
  "mcp": {
    "nano-brain": {
      "type": "remote",
      "url": "http://localhost:3100/mcp"
    }
  }
}
```

## Search Pipeline

```
Query --> BM25 (ts_rank_cd) ---+
                               +--> RRF Fusion (k=60) --> Recency Decay --> Results
Query --> Vector (HNSW cos) ---+
```

- **BM25:** `websearch_to_tsquery` + `ts_rank_cd` on PostgreSQL tsvector
- **Vector:** pgvector HNSW index with cosine distance
- **RRF:** Reciprocal Rank Fusion (k=60), scores normalized to [0,1]
- **Recency:** exponential half-life decay (default 180 days, weight 0.3)

## Architecture

- 15 internal packages: config, server, handlers, storage, sqlc, embed, search, watcher, harvest, mcp, migrate, telemetry, health, bench
- 7 goose SQL migrations (embedded)
- Constructor injection (no DI framework)
- errgroup + context for goroutine lifecycle
- Echo v4 middleware: workspace extraction, content-type enforcement, version header

## Migration from V1

```bash
# Import V1 SQLite data to PostgreSQL
nano-brain db:migrate --from-v1 /path/to/old/index.db

# Idempotent — safe to run multiple times
# Uses content-addressed SHA-256 hashing
# Pure Go SQLite reader (modernc.org/sqlite, no CGO)
```

## Tech Stack

- **Go 1.23** — compiled to single static binary (`CGO_ENABLED=0`)
- **PostgreSQL 17** — relational storage + full-text search (tsvector/tsquery)
- **pgvector 0.8.2** — HNSW vector indexing
- **Echo v4** — HTTP framework
- **sqlc** — type-safe SQL code generation
- **goose v3** — database migrations
- **zerolog** — structured JSON logging
- **koanf** — YAML + env configuration
- **fsnotify** — file system watching
- **modernc.org/sqlite** — V1 migration reader (pure Go)

## License

MIT
