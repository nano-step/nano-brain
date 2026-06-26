# nano-brain

**Your AI agent remembers everything.**

Persistent memory and code intelligence for AI coding agents. Across sessions, machines, and team members.

[![Go 1.23](https://img.shields.io/badge/Go-1.23-00ADD8?logo=go)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![GitHub](https://img.shields.io/badge/GitHub-nano--step%2Fnano--brain-181717?logo=github)](https://github.com/nano-step/nano-brain)
[![npm](https://img.shields.io/badge/npm-@nano--step%2Fnano--brain-CC3533?logo=npm)](https://www.npmjs.com/package/@nano-step/nano-brain)
[![Docker](https://img.shields.io/badge/Docker-2496ED?logo=docker&logoColor=white)](https://hub.docker.com/r/nano-step/nano-brain)
[![Discord](https://img.shields.io/badge/Discord-5865F2?logo=discord&logoColor=white)](https://discord.gg/nano-brain)

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
- **Actually useful** — Not a toy demo. Production-ready with 16 MCP tools, hybrid search, code intelligence, and agent-oriented benchmarks.
- **Built for developers** — Go binary, PostgreSQL, zero magic. You can read the code.
- **Beating competitors** — P@5 of 80% vs LlamaIndex's 55% and Qdrant's 27% on real-world queries.

Star it if you want AI agents that actually learn from context.

---

## What It Does

nano-brain solves **session amnesia** — the problem where AI agents forget everything when the session ends.

It automatically:
- **Ingests** AI sessions, notes, and codebase files
- **Indexes** everything with hybrid search (BM25 + pgvector)
- **Serves** memories via 16 MCP tools and REST API

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

### 16 MCP Tools

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

## Demo

### Query Your Codebase

```bash
# Search for authentication patterns
curl -X POST http://localhost:3100/api/v1/query \
  -H "Content-Type: application/json" \
  -d '{"workspace": "abc123", "query": "how does authentication work"}'
```

### Trace Call Chains

```bash
# Trace from entry point
curl -X POST http://localhost:3100/api/v1/graph/trace \
  -H "Content-Type: application/json" \
  -d '{"workspace": "abc123", "node": "main.go::main", "max_depth": 5}'
```

### Analyze Impact

```bash
# What breaks if I change this file?
curl -X POST http://localhost:3100/api/v1/graph/impact \
  -H "Content-Type: application/json" \
  -d '{"workspace": "abc123", "node": "src/auth/login.ts", "max_depth": 2}'
```

### Generate Flow Diagrams

```bash
# Get flow diagram for a controller
curl -X POST http://localhost:3100/api/v1/graph/flow \
  -H "Content-Type: application/json" \
  -d '{"workspace": "abc123", "entry": "POST /users"}'
```

Returns Mermaid flowchart:

```mermaid
flowchart LR
  POST_/users["POST /users"]
  POST_/users --> UsersController#create
  UsersController#create --> User.create
  UsersController#create --> Mailer.welcome
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

### Search Quality vs Competitors

| Metric | nano-brain | LlamaIndex | Qdrant/Mem0 | Cognee | GraphRAG | Zep |
|--------|------------|------------|-------------|--------|----------|-----|
| P@5 | **80%** | 55% | 27% | — | — | — |
| MRR | **95%** | — | — | — | — | — |
| Latency | **42ms** | — | — | — | — | — |
| Code Intelligence | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| Symbol Graph | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| Impact Analysis | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| Flow Diagrams | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |

Tested on 60 domain-specific queries across 3 workspaces. nano-brain is the **only** solution with code intelligence — competitors focus on conversation memory and document retrieval.

### Competitive Landscape

**What competitors offer:**
- **Mem0 / Zep** — Conversation memory, temporal ranking, chat history recall
- **Cognee / GraphRAG** — Document-level knowledge graphs, multi-hop reasoning
- **LlamaIndex** — Flexible RAG pipelines, document retrieval

**What nano-brain adds:**
- **Code intelligence** — Symbol graphs, call chains, impact analysis, flow diagrams
- **Agent-oriented benchmarks** — Measures how well agents find context for domain tasks
- **Hybrid search** — BM25 + pgvector + RRF fusion + recency decay

Competitors optimize for conversation recall. nano-brain optimizes for **agent comprehension** — helping agents understand codebases, not just retrieve documents.

### Agent-Oriented Capability Benchmarks

nano-brain is built for agents. These benchmarks measure how well agents can **find relevant context for real-world domain tasks** using nano-brain's MCP tools — not just search quality in isolation.

Each benchmark runs a deterministic agent workflow:
1. **query_question** — natural-language domain question
2. **query_input** — optimized search query
3. **symbols_identifiers** — symbol lookup for known identifiers

This mimics how a real agent explores a codebase: broad understanding first, then targeted retrieval.

#### Scores

| Workspace | Domain | Overall | Multi-tool | Search-QA | Symbol-Lookup |
|-----------|--------|---------|------------|-----------|---------------|
| **nano-brain** | Go daemon | **1.000** | 1.000 | 1.000 | 1.000 |
| **TypeScript** | CS2 item trading | **0.885** | 1.000 | 0.817 | 1.000 |
| **Rails** | CS2 item trading | **0.795** | 1.000 | 0.726 | 0.667 |

**What this means:**
- **Multi-tool 1.000** — When agents combine search + symbols, they find every expected context item
- **Overall 0.885** — TypeScript workspace: agent finds 88.5% of expected domain artifacts
- **Fixed vs Agent** — Agent workflow improves recall by 15-40% over single-tool queries
- **Unique capability** — No competitor offers agent-oriented benchmarks or code intelligence

#### How to Run

```bash
# TypeScript workspace (CS2 item trading domain)
NANO_BRAIN_URL=http://localhost:3100 \
NANO_BRAIN_WORKSPACE=<your-workspace-hash> \
go test -v -tags=capbench -run TestCapabilityBenchmark \
  ./benchmarks/typescript/capability/

# Rails workspace (CS2 item trading domain)
NANO_BRAIN_URL=http://localhost:3100 \
NANO_BRAIN_WORKSPACE=<your-workspace-hash> \
go test -v -tags=capbench -run TestCapabilityBenchmark \
  ./benchmarks/rails/capability/

# nano-brain itself (Go daemon)
NANO_BRAIN_URL=http://localhost:3100 \
NANO_BRAIN_WORKSPACE=nano-brain \
go test -v -tags=capbench -run TestCapabilityBenchmark \
  ./benchmarks/capability/
```

#### Task Categories

| Category | What It Tests | Tools Used |
|----------|---------------|------------|
| **search-qa** | Domain concept retrieval via search | `query_question`, `query_input` |
| **symbol-lookup** | Known identifier resolution | `query_input`, `symbols_identifiers` |
| **multi-tool** | Cross-tool workflow (search → symbols) | All three tools in sequence |

See individual benchmark READMEs for full task breakdowns:
- [`benchmarks/typescript/capability/README.md`](benchmarks/typescript/capability/README.md)
- [`benchmarks/rails/capability/README.md`](benchmarks/rails/capability/README.md)
- [`benchmarks/capability/README.md`](benchmarks/capability/README.md)

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

- [Getting Started](docs/GETTING_STARTED.md) — Step-by-step setup guide
- [Configuration](docs/CONFIGURATION.md) — All config options
- [REST API](docs/API.md) — HTTP endpoints
- [CLI Commands](docs/CLI.md) — Command reference
- [MCP Tools](docs/MCP.md) — Tool documentation
- [Architecture](docs/ARCHITECTURE.md) — System design
- [Changelog](CHANGELOG.md) — What's new
- [Roadmap](docs/ROADMAP.md) — What's planned
- [Feature Showcase](docs/FEATURES.md) — Visual examples

---

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

### Development Setup

```bash
# Clone the repo
git clone https://github.com/nano-step/nano-brain.git
cd nano-brain

# Build
CGO_ENABLED=0 go build -o nano-brain ./cmd/nano-brain

# Run tests
go test -race -short ./...

# Run integration tests (requires PostgreSQL)
go test -race -tags=integration ./...
```

### Project Structure

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
├── migrations/           # Database migrations
└── benchmarks/           # Performance benchmarks
```

---

## Community

- [GitHub Discussions](https://github.com/nano-step/nano-brain/discussions) — Ask questions, share ideas
- [Discord](https://discord.gg/nano-brain) — Real-time chat
- [Twitter](https://twitter.com/nano_brain) — Updates and announcements

---

## License

MIT — see [LICENSE](LICENSE) for details.

---

## Acknowledgments

Built with:
- [Go](https://go.dev/) — Fast, statically typed language
- [PostgreSQL](https://www.postgresql.org/) — The world's most advanced open source database
- [pgvector](https://github.com/pgvector/pgvector) — Open-source vector similarity search
- [Echo](https://echo.labstack.com/) — High performance, extensible, minimalist Go web framework
- [sqlc](https://sqlc.dev/) — Generate type-safe code from SQL
- [goose](https://github.com/pressly/goose) — Database migration tool
- [zerolog](https://github.com/rs/zerolog) — Zero allocation JSON logger
- [koanf](https://github.com/knadh/koanf) — Configuration manager
- [fsnotify](https://github.com/fsnotify/fsnotify) — Cross-platform file system notifications
