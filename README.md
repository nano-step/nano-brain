# nano-brain

**Built for agents. Not humans.**

Agent-oriented memory and code intelligence. AI agents don't read docs — they need structured context, impact analysis, and call chains. nano-brain provides exactly that via MCP.

[![Go 1.23](https://img.shields.io/badge/Go-1.23-00ADD8?logo=go)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![GitHub](https://img.shields.io/badge/GitHub-nano--step%2Fnano--brain-181717?logo=github)](https://github.com/nano-step/nano-brain)
[![npm](https://img.shields.io/badge/npm-@nano--step%2Fnano--brain-CC3533?logo=npm)](https://www.npmjs.com/package/@nano-step/nano-brain)
[![Docker](https://img.shields.io/badge/Docker-2496ED?logo=docker&logoColor=white)](https://hub.docker.com/r/nano-step/nano-brain)
[![Discord](https://img.shields.io/badge/Discord-5865F2?logo=discord&logoColor=white)](https://discord.gg/nano-brain)


### Install

```bash
# Recommended — one-line installer (no Node.js needed): downloads the prebuilt
# binary for your platform from GitHub Releases and verifies its SHA-256.
curl -fsSL https://raw.githubusercontent.com/nano-step/nano-brain/master/install.sh | bash

# Or via npm (handy if you're already in a JS/agent toolchain)
npm install -g @nano-step/nano-brain

# Or build from source
CGO_ENABLED=0 go build -o nano-brain ./cmd/nano-brain
```

Prefer to read the installer before running it? Download, inspect, then run:

```bash
curl -fsSL -o install.sh https://raw.githubusercontent.com/nano-step/nano-brain/master/install.sh
less install.sh && bash install.sh
```

### Start

```bash
# One command — the interactive wizard provisions PostgreSQL (via Docker or a
# remote URL), configures embeddings, starts the server, registers this
# project, and sets up your MCP client.
nano-brain init
```

For a manual, per-step setup (VPS / team, no Docker, or Windows), see [docs/SETUP_AGENT.md](docs/SETUP_AGENT.md).
---

## Why Star This Project?

**If you've ever wished your AI agent stopped flying blind in your codebase.**

Most memory tools optimize for conversation recall. nano-brain optimizes for **agent comprehension** — the ability to understand codebases, trace dependencies, and predict the blast radius of changes.

nano-brain is:

- **Agent-oriented** — Built around how agents actually work: impact analysis before edits, call chain tracing, symbol lookup. Not a document store with MCP slapped on top.
- **Self-hosted** — Your data stays on your server. No cloud dependency.
- **Works everywhere** — OpenCode, Claude Code, Cursor, any MCP client.
- **Actually useful** — Not a toy demo. Production-ready with 16 MCP tools, hybrid search, code intelligence, and agent-oriented benchmarks.
- **Built for developers** — Go binary, PostgreSQL, zero magic. You can read the code.
- **Beating competitors** — P@5 of 80% vs LlamaIndex's 55% and Qdrant's 27% on real-world queries.

Star it if you want agents that understand your code, not just search it.

---

## What It Does

nano-brain is an **agent-oriented infrastructure layer** that sits between your AI agent and your codebase.

It solves two problems agents have:

1. **Session amnesia** — Agents forget everything when the session ends. nano-brain persists context across sessions via harvesting, indexing, and retrieval.
2. **Codebase blindness** — Agents can't trace dependencies, measure blast radius, or understand control flow. nano-brain builds a live code graph and exposes it via 16 MCP tools.

**Why MCP?** Because agents don't read docs. They call tools. Every capability is a tool call — no REST API ceremony, no JSON parsing, no manual file reading.

### How agents use it

| Agent needs to... | Tool | What it returns |
|---|---|---|
| Understand a feature | `memory_query` | Hybrid search results with context |
| Check what breaks before editing | `memory_impact` | Blast radius — all dependent files |
| Trace an execution path | `memory_trace` | Call chain from entry point |
| Find a function definition | `memory_symbols` | Symbol location + kind |
| Recall a past decision | `memory_query` | Past session context |
| Save a discovery | `memory_write` | Persisted for future sessions |

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

## Agent-Oriented Design

nano-brain isn't a memory tool with MCP bolted on. It's designed from the ground up around **how agents actually behave**.

### The agent workflow loop

```
┌─────────────┐     ┌──────────────┐     ┌─────────────┐
│  Agent       │────▶│  memory_query │────▶│  Context     │
│  receives    │     │  /impact/trace│     │  window      │
│  task        │     │              │     │  filled      │
└─────────────┘     └──────────────┘     └──────┬──────┘
                                                 │
                                          ┌──────▼──────┐
                                          │  Agent       │
                                          │  implements  │
                                          │  change      │
                                          └──────┬──────┘
                                                 │
                                          ┌──────▼──────┐
                                          │  memory_write │
                                          │  (persist)    │
                                          └─────────────┘
```

### Why agent behavior matters

| Human workflow | Agent workflow | nano-brain response |
|---|---|---|
| Opens file, reads it | `memory_get` or `memory_search` | Returns structured content, not raw bytes |
| Traces call chain manually | `memory_trace` | Returns function-by-function chain with line numbers |
| Greps for callers | `memory_graph(direction="in")` | Returns all callers in one call |
| Thinks "what breaks?" | `memory_impact` | Returns full blast radius in <50ms |
| Remembers past decisions | `memory_query` | Returns cross-session context |

### The 50ms rule

At 50ms latency, agents run impact analysis on every edit. At 500ms, they skip it. nano-brain is designed for the 50ms world — every code intelligence tool call is sub-50ms, making it practical for agents to use them on every operation.

### What agents actually need

Research from 15+ production code intelligence tools shows:

1. **Impact analysis is #1** — "What breaks if I change this?" is the most common agent query
2. **Call chains > control flow** — Agents trace across files (inter-procedural), not within functions (intra-procedural)
3. **Component composition > internal logic** — For frontend frameworks, "who uses this component?" matters more than "what does the template do?"

nano-brain optimizes for exactly these three patterns.

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

### Configure Your AI Agent

Add to your MCP client config (Claude Code, OpenCode, Cursor, etc.):

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

### Agent-assisted refactoring
Before refactoring, your agent calls `memory_impact` on the target function. Gets the full blast radius. Decides whether to split the change. After refactoring, runs affected tests only — not the full suite.

### Multi-session feature development
Session 1: Agent explores the codebase, discovers patterns. `memory_write` saves findings. Session 2: Agent recalls session 1's discoveries via `memory_query`. No context lost between sessions.

### Legacy codebase onboarding
Index a 5-year-old codebase. Your agent can now answer "what does this function do?", "why does this class exist?", "if I change this file, what else breaks?" — without reading every file.

### Cross-service debugging
Agent traces a bug from frontend to backend. `memory_trace` follows the call chain across services. `memory_graph` shows which microservices depend on the failing endpoint.

### Team knowledge sharing
One server, whole team. Every developer's AI agent connects to the same PostgreSQL. Decisions, architecture notes, code intelligence — instantly shared. New hires get full context from day one.

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

**What nano-brain adds (agent-oriented):**
- **Impact analysis** — "What breaks if I change this?" — the #1 question agents ask. Pre-computed blast radius in <50ms.
- **Call chain tracing** — Follow execution paths across files. Agent gets a structured trace, not raw source.
- **Symbol graph** — Find definitions, callers, callees. `memory_symbols` + `memory_graph`.
- **Agent-oriented benchmarks** — Measures how well agents find context for domain tasks — not just search precision in isolation.

**The difference:** Competitors optimize for "did the agent find the right document?" nano-brain optimizes for "did the agent understand the codebase well enough to make the right change?"

At 50ms latency, agents run impact analysis on every edit. At 500ms, they skip it. nano-brain is designed for the 50ms world.

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

`GET /api/openapi.json` serves the current OpenAPI 3.0 spec for the full REST API — import it into Postman, Swagger UI, or any codegen tool to discover the endpoint surface without reading Go source. Regenerate it via `make generate-openapi` after adding or changing routes.

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
