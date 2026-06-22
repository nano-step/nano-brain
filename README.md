# nano-brain

**Your AI agent remembers everything.**

Persistent memory and code intelligence for AI coding agents. Across sessions, machines, and team members.

[![Go 1.23](https://img.shields.io/badge/Go-1.23-00ADD8?logo=go)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![GitHub](https://img.shields.io/badge/GitHub-nano--step%2Fnano--brain-181717?logo=github)](https://github.com/nano-step/nano-brain)

---

## TL;DR

```bash
# Install
npm install -g @nano-step/nano-brain

# Start
nano-brain serve -d

# Your AI agent now has memory
```

---

## Why Star This Project?

**If you've ever wished your AI agent remembered what you told it yesterday.**

nano-brain is the missing memory layer for AI coding agents. It's:

- **Self-hosted** — Your data stays on your server. No cloud dependency.
- **Works everywhere** — OpenCode, Claude Code, Cursor, any MCP client.
- **Actually useful** — Not a toy demo. Production-ready with 14 MCP tools, hybrid search, and code intelligence.
- **Built for developers** — Go binary, PostgreSQL, zero magic. You can read the code.
- **Beating competitors** — P@5 of 0.749 vs LlamaIndex's 0.55 and Qdrant's 0.27 on real-world queries.

Star it if you want AI agents that actually learn from context.

---

## What It Does

nano-brain solves **session amnesia** — the problem where AI agents forget everything when the session ends.

It automatically:
- **Ingests** AI sessions, notes, and codebase files
- **Indexes** everything with hybrid search (BM25 + pgvector)
- **Serves** memories via 14 MCP tools and REST API

Built in Go with PostgreSQL. Single static binary. Zero CGO dependencies.

---

## Architecture

```mermaid
graph LR
    A[Your AI Agent] -->|MCP Protocol| B[nano-brain]
    B --> C[PostgreSQL + pgvector]
    B --> D[Session Harvesting]
    B --> E[Code Intelligence]
    B --> F[Hybrid Search]
    
    D --> D1[OpenCode Sessions]
    D --> D2[Claude Code Sessions]
    
    E --> E1[Symbol Graph]
    E --> E2[Flow Diagrams]
    E --> E3[Impact Analysis]
    
    F --> F1[BM25 Full-Text]
    F --> F2[Vector Similarity]
    F --> F3[RRF Fusion]
```

---

## Key Features

### Hybrid Search

```mermaid
graph LR
    Q[Query] --> BM25[BM25 Full-Text]
    Q --> Vector[Vector Similarity]
    BM25 --> RRF[RRF Fusion]
    Vector --> RRF
    RRF --> Results[Ranked Results]
```

BM25 full-text + pgvector HNSW cosine similarity + Reciprocal Rank Fusion + recency decay.

### Code Intelligence

```mermaid
graph TD
    A[Entry Point] --> B[Function Call]
    B --> C[Method Call]
    B --> D[Database Query]
    C --> E[External Service]
    D --> F[Redis Cache]
```

- **Symbol extraction** — Functions, types, interfaces, constants
- **Call chain tracing** — Follow execution paths across files
- **Impact analysis** — "What breaks if I change this?"
- **Flow diagrams** — Mermaid flowcharts and sequence diagrams

### Session Harvesting

```mermaid
graph LR
    S1[OpenCode DB] --> H[Harvester]
    S2[Claude Code JSONL] --> H
    H --> L[LLM Summarizer]
    L --> I[Indexer]
    I --> DB[PostgreSQL]
```

Auto-ingest from OpenCode and Claude Code sessions. Map-reduce LLM summarization. Incremental harvest with dedup.

### 14 MCP Tools

| Tool | Description |
|------|-------------|
| `memory_query` | Hybrid search (BM25 + vector + RRF) |
| `memory_search` | BM25 keyword search |
| `memory_vsearch` | Vector similarity search |
| `memory_get` | Get document by path |
| `memory_write` | Write/update document |
| `memory_graph` | Knowledge graph view |
| `memory_trace` | Call chain trace |
| `memory_impact` | Cross-file impact analysis |
| `memory_symbols` | Symbol search |
| `memory_flow` | Execution flow visualization |
| `memory_tags` | List tags with counts |
| `memory_status` | Server status |
| `memory_update` | Trigger re-embedding |
| `memory_wake_up` | Workspace briefing |

---

## Quick Start

### Prerequisites

- **Go 1.23+** OR pre-built binary
- **PostgreSQL 17** with **pgvector 0.8.2**
- **Ollama** (for embeddings) or any OpenAI-compatible provider

### Install

```bash
# Via npm (recommended)
npm install -g @nano-step/nano-brain

# Or build from source
CGO_ENABLED=0 go build -o nano-brain ./cmd/nano-brain
```

### Start

```bash
# Start PostgreSQL
docker run -d --name nanobrain-pg -p 5432:5432 \
  -e POSTGRES_USER=nanobrain -e POSTGRES_PASSWORD=nanobrain -e POSTGRES_DB=nanobrain_dev \
  pgvector/pgvector:pg17

# Start nano-brain
nano-brain serve -d

# Register your project
nano-brain init --root=/path/to/your/project
```

### Configure Your AI Agent

Add to your MCP client config (Claude Code, OpenCode, Cursor, etc.):

```json
{
  "mcp": {
    "nano-brain": {
      "type": "http",
      "url": "http://localhost:3100/mcp"
    }
  }
}
```

---

## Use Cases

### Multi-machine developer
Work on office PC, home laptop, personal machine — each with different sessions. Deploy nano-brain on a VPS. Every session gets harvested. Switch machines, pick up where you left off.

### Team knowledge base
One server, whole team. Every developer's AI agent connects to the same PostgreSQL. Decisions, architecture notes, code intelligence — instantly shared. New hires get full context from day one.

### Legacy codebase archaeology
Inherit a 5-year-old codebase with no docs? Index it. Your AI agent can now answer "what does this function do?", "why does this class exist?", "if I change this file, what else breaks?"

### Pre-commit impact check
Before pushing, run `memory_impact` on changed files. Discover what else depends on them. Catch breaking changes before CI.

---

## Performance

### Benchmark Results

| Metric | nano-brain | LlamaIndex | Qdrant/Mem0 |
|--------|------------|------------|-------------|
| P@5 | **0.749** | 0.55 | 0.27 |
| MRR | **0.967** | — | — |
| Latency | 42ms | — | — |

Tested on 60 domain-specific queries across 3 workspaces (gaming, Go codebase, Rails app).

### Search Quality

- **BM25 OR fallback** — Retries with OR semantics when AND returns 0 results
- **Incoming edges symbol fallback** — Falls back to symbol name when target lookup fails
- **Workspace-specific queries** — Each project gets queries tailored to its domain

---

## Ruby / Rails Support

nano-brain supports Ruby and Ruby on Rails code intelligence:

- **Rails routes** — `resources`, `get`/`post`/`patch`/`put`/`delete`, `namespace`
- **Control-flow graphs** — `if`/`else`, loops, `begin`/`rescue`, method defs
- **Cross-file resolution** — Class→file index, resolver, reconcile edges
- **Flow diagrams** — Controller→service→model chains (20-34 nodes)

Example flow for a Rails controller action:

```mermaid
flowchart LR
  POST_/users["POST /users"]
  POST_/users --> UsersController#create
  UsersController#create --> User.create
  UsersController#create --> Mailer.welcome
```

---

## Tech Stack

- **Go 1.23** — Single static binary (`CGO_ENABLED=0`)
- **PostgreSQL 17** — Full-text search (tsvector/tsquery)
- **pgvector 0.8.2** — HNSW vector indexing
- **Echo v4** — HTTP framework
- **sqlc** — Type-safe SQL code generation
- **goose v3** — Database migrations
- **zerolog** — Structured JSON logging
- **koanf** — YAML + env configuration
- **fsnotify** — File system watching

---

## Configuration

Config file: `~/.nano-brain/config.yml`

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

See [Configuration](docs/CONFIGURATION.md) for full options.

---

## Documentation

- [Configuration](docs/CONFIGURATION.md) — All config options
- [REST API](docs/API.md) — HTTP endpoints
- [CLI Commands](docs/CLI.md) — Command reference
- [MCP Tools](docs/MCP.md) — Tool documentation
- [Architecture](docs/ARCHITECTURE.md) — System design
- [Changelog](CHANGELOG.md) — What's new
- [Roadmap](docs/ROADMAP.md) — What's planned

---

## License

MIT — see [LICENSE](LICENSE) for details.
