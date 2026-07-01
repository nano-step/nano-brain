# nano-brain

[![npm version](https://img.shields.io/npm/v/@nano-step/nano-brain?color=blue&label=npm)](https://www.npmjs.com/package/@nano-step/nano-brain)
[![npm downloads](https://img.shields.io/npm/dm/@nano-step/nano-brain?color=blue)](https://www.npmjs.com/package/@nano-step/nano-brain)
[![Publish Stable](https://github.com/nano-step/nano-brain/actions/workflows/publish-stable.yml/badge.svg?branch=master)](https://github.com/nano-step/nano-brain/actions/workflows/publish-stable.yml)
[![Publish Beta](https://github.com/nano-step/nano-brain/actions/workflows/publish-beta.yml/badge.svg?branch=develop)](https://github.com/nano-step/nano-brain/actions/workflows/publish-beta.yml)
[![Go 1.23](https://img.shields.io/badge/Go-1.23-00ADD8?logo=go)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/license-MIT-green)](LICENSE)

Persistent memory and code intelligence for AI coding agents.

## What It Does

A persistent memory server for AI coding agents. It solves the #1 problem with AI assistants: **they forget everything between sessions.**

nano-brain automatically ingests your AI sessions, notes, and codebase, indexes everything with full-text search + vector embeddings + knowledge graph, and serves memories via 16 MCP tools.

## Key Features

- **Hybrid search pipeline** — BM25 full-text + pgvector HNSW cosine similarity + RRF fusion with recency decay
- **Code intelligence** — symbol graph, call flow detection, impact analysis
- **Session harvesting** — auto-ingest from OpenCode and Claude Code sessions
- **Multi-workspace isolation** — per-workspace data, cross-workspace search with `scope=all`
- **Flexible embedding providers** — Ollama, OpenAI-compatible, VoyageAI
- **PostgreSQL + pgvector** — single well-understood data store; no separate vector service required
- **Privacy-first** — 100% local processing option, your code never leaves your machine
- **MCP + REST** — HTTP transport for local or containerized environments
- **Self-hosted** — Go static binary, PostgreSQL, zero magic

## Architecture

```
User Query
    │
    ▼
┌─────────────────┐
│  BM25 (tsvector)│
│  +              │
│  pgvector HNSW  │
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  RRF Fusion     │  k=60
└────────┬────────┘
         │
         ▼
┌─────────────────┐
│  Recency Decay  │  weight: 0.3
└────────┬────────┘
         │
         ▼
    Final Results
```

### Session Harvesting

```
OpenCode DB        ──┐
Claude Code JSONL  ──┤─→ Harvester → LLM Summarizer → Indexer → PostgreSQL
```

## Search Pipeline

**`memory_search`** — BM25 full-text only (fast, exact keyword matching)

**`memory_vsearch`** — Vector only (semantic similarity via pgvector)

**`memory_query`** — Full hybrid pipeline:

1. **BM25 full-text scoring** — PostgreSQL tsvector/tsquery
2. **Vector cosine similarity** — pgvector HNSW index
3. **RRF fusion** — k=60
4. **Recency boost** — exponential decay, weight 0.3

## Code Intelligence

**`memory_graph`** — One-hop callers/callees/imports

**`memory_trace`** — Downstream call chain trace

**`memory_impact`** — Pre-change blast radius analysis

**`memory_symbols`** — Symbol search (functions, types, interfaces)

**`memory_flow`** — HTTP route execution flow

**`memory_flowchart`** — Function-level control-flow graph

## Data Ingestion

**Session harvesting** — Converts OpenCode and Claude Code sessions into searchable content:
- OpenCode: reads from the OpenCode SQLite DB
- Claude Code: reads JSONL session files
- Incremental harvest with hash-based deduplication

**File watching** — Monitors workspace for changes and reindexes automatically

**Codebase indexing** — Parses Go, TypeScript, JavaScript, Python, and Ruby source files for symbols and call graphs

## MCP Tools (16 Total)

| Tool | Description |
|------|-------------|
| `memory_query` | Hybrid search — default first tool for broad questions |
| `memory_search` | BM25 keyword search for exact text/errors |
| `memory_vsearch` | Vector similarity for fuzzy concepts |
| `memory_get` | Get document by path or ID |
| `memory_write` | Write/update document |
| `memory_graph` | One-hop callers/callees/imports |
| `memory_trace` | Downstream call chain trace |
| `memory_impact` | Pre-change blast radius analysis |
| `memory_symbols` | Symbol search (functions, types, interfaces) |
| `memory_flow` | HTTP route execution flow |
| `memory_flowchart` | Function-level control-flow graph |
| `memory_workspaces_resolve` | Resolve path to workspace hash |
| `memory_tags` | List tags with counts |
| `memory_status` | Server and queue health |
| `memory_update` | Trigger re-embedding |
| `memory_wake_up` | Session-start workspace briefing |

## Installation & Quick Start

```bash
# Install globally
npm install -g @nano-step/nano-brain

# Or build from source
CGO_ENABLED=0 go build -o nano-brain ./cmd/nano-brain
```

### Start PostgreSQL

```bash
docker run -d --name nanobrain-pg -p 5432:5432 \
  -e POSTGRES_USER=nanobrain \
  -e POSTGRES_PASSWORD=nanobrain \
  -e POSTGRES_DB=nanobrain_dev \
  pgvector/pgvector:pg17
```

### Start nano-brain

```bash
nano-brain serve -d
```

### Register your project

```bash
nano-brain init --root=/path/to/your/project
```

### Check status

```bash
nano-brain status
```

### MCP Configuration

Add to your AI agent's MCP config (Claude Code, OpenCode, Cursor, etc.):

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

Optionally bind a default workspace to the connection by appending `?workspace=<name-or-hash>` to the URL (e.g. `"url": "http://localhost:3100/mcp?workspace=nano-brain"`) — tool calls can then omit the `workspace` argument. Run `nano-brain workspaces list` to see the registered name/hash for a project. An explicit `workspace` argument always overrides the connection default; the value must be a name or full hash, not `"all"`.

## Configuration

Config file: `~/.nano-brain/config.yml` (see `config.default.yml` / README for the full reference).

```yaml
server:
  host: localhost
  port: 3100

database:
  url: postgres://nanobrain:nanobrain@localhost:5432/nanobrain_dev

embedding:
  provider: ollama
  url: http://localhost:11434
  model: nomic-embed-text

search:
  rrf_k: 60
  recency_weight: 0.3
  limit: 20
```

See [Configuration](CONFIGURATION.md) for full options.

## CLI Commands

### Setup & Initialization

```bash
nano-brain init               # Full initialization (config, index, embed)
nano-brain init --root=/path  # Initialize for specific project
nano-brain status             # Show index health, collections, model status
nano-brain doctor             # Check prerequisites (PostgreSQL, pgvector, embedder)
```

### Server

```bash
nano-brain serve -d           # Start server as background daemon
nano-brain serve              # Start server in foreground
nano-brain stop               # Stop the running daemon
```

### Search

```bash
nano-brain search "query"     # BM25 keyword search
nano-brain vsearch "query"    # Vector semantic search
nano-brain query "query"      # Hybrid search (BM25 + vector + RRF)
```

### Memory Management

```bash
nano-brain write "content"            # Write to memory
nano-brain write "content" --tags=decision,architecture
nano-brain get <path>                 # Retrieve document by path
nano-brain tags                       # List all tags with counts
```

### Index Management

```bash
nano-brain reindex            # Reindex codebase in current workspace
nano-brain harvest            # Run session harvest manually
```

### Code Intelligence

```bash
nano-brain context <symbol>   # 360° view of a code symbol
nano-brain code-impact <sym>  # Analyze impact of changing a symbol
nano-brain detect-changes     # Map current git changes to affected symbols
nano-brain wake-up            # Session-start workspace briefing
```

### Workspace Management

```bash
nano-brain workspaces         # List all workspaces
```

### Logs

```bash
nano-brain logs               # Show recent logs
nano-brain logs -f            # Tail logs in real-time
```

### Benchmarking

```bash
nano-brain bench generate     # Generate fixtures
nano-brain bench run          # Run benchmark suite
nano-brain bench compare new.json baseline.json
```

## Project Structure

```
nano-brain/
├── cmd/nano-brain/       # CLI dispatcher + server startup
├── internal/
│   ├── config/           # Configuration management
│   ├── server/           # HTTP server + handlers
│   ├── storage/          # PostgreSQL + sqlc
│   ├── search/           # Hybrid search pipeline
│   ├── embed/            # Embedding queue
│   ├── watcher/          # File system watcher
│   ├── harvest/          # Session harvesting
│   ├── mcp/              # MCP protocol tools
│   ├── graph/            # Code intelligence
│   └── ...
├── migrations/           # Database migrations (goose)
└── benchmarks/           # Performance benchmarks
```

## Tech Stack

- **Go 1.23** — Single static binary (`CGO_ENABLED=0`)
- **PostgreSQL 17** — Full-text search (tsvector/tsquery)
- **pgvector 0.8.2** — HNSW vector indexing
- **Echo v4** — HTTP framework
- **sqlc** — Type-safe SQL code generation
- **goose v3** — Database migrations
- **zerolog** — Structured JSON logging
- **koanf** — YAML + env configuration

## Embedding Providers

- **Ollama** — nomic-embed-text, mxbai-embed-large, etc. (local, free, recommended for getting started)
- **OpenAI-compatible** — Azure OpenAI, LM Studio, VoyageAI, custom endpoints

## How nano-brain Compares

| | nano-brain | Mem0 | Zep / Graphiti | Letta |
|---|---|---|---|---|
| **Search** | Hybrid (BM25 + pgvector + RRF) | Vector only | Graph + vector | Agent-managed |
| **Storage** | PostgreSQL + pgvector | PostgreSQL + external vector DB | Neo4j | PostgreSQL / SQLite |
| **MCP Tools** | 16 | 4-9 | 9-10 | 7 |
| **Code Intelligence** | Yes (symbol graph, impact analysis, call tracing) | No | No | No |
| **Session Recall** | Yes (OpenCode + Claude Code) | No | No | No |
| **Local-First** | Yes (Ollama) | Requires OpenAI key | Requires Docker + Neo4j | Yes |

## License

MIT
