# nano-brain

**Built for agents. Not humans.**

Agent-oriented memory and code intelligence over MCP. AI agents don't read docs — they call tools. nano-brain gives them structured context, symbol lookup, call-chain tracing, and change-impact analysis across sessions and repos.

[![Go 1.23](https://img.shields.io/badge/Go-1.23-00ADD8?logo=go)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![GitHub](https://img.shields.io/badge/GitHub-nano--step%2Fnano--brain-181717?logo=github)](https://github.com/nano-step/nano-brain)
[![npm](https://img.shields.io/badge/npm-@nano--step%2Fnano--brain-CC3533?logo=npm)](https://www.npmjs.com/package/@nano-step/nano-brain)
[![Docker](https://img.shields.io/badge/Docker-2496ED?logo=docker&logoColor=white)](https://hub.docker.com/r/nano-step/nano-brain)
[![Discord](https://img.shields.io/badge/Discord-5865F2?logo=discord&logoColor=white)](https://discord.gg/nano-brain)

---

## What it is

nano-brain is an infrastructure layer between your AI agent and your code. It solves two problems agents have:

1. **Session amnesia** — agents forget everything when a session ends. nano-brain harvests, indexes, and retrieves past sessions and saved decisions.
2. **Codebase blindness** — agents can't cheaply trace dependencies, measure blast radius, or map an execution path. nano-brain builds a code graph and exposes it as MCP tools.

Self-hosted (Go binary + PostgreSQL), works with any MCP client (Claude Code, OpenCode, Cursor, …), and returns structured results — not raw file bytes.

---

## Install

```bash
# Recommended — one-line installer (no Node.js needed): downloads the prebuilt
# binary for your platform from GitHub Releases and verifies its SHA-256.
curl -fsSL https://raw.githubusercontent.com/nano-step/nano-brain/master/install.sh | bash

# Or via npm (handy if you're already in a JS/agent toolchain)
npm install -g @nano-step/nano-brain

# Or build from source
CGO_ENABLED=0 go build -o nano-brain ./cmd/nano-brain
```

Prefer to read the installer first? `curl -fsSL -o install.sh …/install.sh && less install.sh && bash install.sh`.

## Start

```bash
# One command — the interactive wizard provisions PostgreSQL (Docker or remote URL),
# configures embeddings, starts the server, registers this project, and wires up your MCP client.
nano-brain init
```

Manual, per-step setup (VPS / team / no Docker / Windows): [docs/SETUP_AGENT.md](docs/SETUP_AGENT.md).

### Connect your agent

Add to your MCP client config:

```json
{
  "mcp": {
    "nano-brain": { "type": "remote", "url": "http://localhost:3100/mcp" }
  }
}
```

Bind a default workspace by appending `?workspace=<name-or-hash>` to the URL (e.g. `…/mcp?workspace=my-project`) so tool calls can omit the `workspace` argument. An explicit `workspace` argument always overrides it; the value must be a name or full hash (not `"all"`). Run `nano-brain workspaces list` to see registered names/hashes.

---

## The agent workflow

A cold agent should follow this order — it maps 1:1 to the tools:

1. **Orient** — `memory_wake_up` for a workspace briefing (recent activity, collections, stats).
2. **Locate the workspace** — `memory_workspaces_resolve(path)` returns a deterministic hash and whether it's registered. Registering the repo root also covers its subdirectories, so resolve the root, not a single sub-repo.
3. **Understand** — `memory_query` (hybrid, best for natural-language questions), `memory_search` (exact identifiers/errors), or `memory_vsearch` (fuzzy concepts). Search returns 500-char snippets; fetch full text with `memory_get` only for the hits you keep.
4. **Navigate code** — `memory_symbols` to find a definition (no indexing needed), then `memory_graph` / `memory_trace` for call chains and `memory_flow` for an HTTP route's execution.
5. **Before a risky edit** — `memory_impact(node, direction="in")` for the blast radius (who calls / imports this).
6. **Persist** — `memory_write` to save a decision, lesson, or handoff for the next session.

```
wake_up → resolve ─▶ query / search / symbols ─▶ graph / trace / flow / impact ─▶ (edit) ─▶ write
```

---

## MCP tools (18)

**Search & recall**

| Tool | Use it for |
|------|------------|
| `memory_query` | Hybrid BM25 + vector + RRF + recency — default first tool for broad questions |
| `memory_search` | Exact keyword/BM25 — error strings, identifiers, config keys, channel/topic names |
| `memory_vsearch` | Vector similarity — fuzzy concepts (best on single-concept queries) |
| `memory_get` | Fetch full content (or a line range) for one known document/symbol |
| `memory_write` | Persist a decision/lesson/handoff (supports `supersedes`) |
| `memory_ticket` | All sessions tagged with a ticket ID (e.g. `PROJ-1234`) across the workspace |
| `memory_wake_up` | Session-start briefing: recent memories, active collections, stats |
| `memory_tags` | List collections and document counts |

**Code intelligence**

| Tool | Use it for |
|------|------------|
| `memory_symbols` | Find a function/type/interface/const by name (reads the filesystem — no indexing required) |
| `memory_graph` | One-hop neighbors: direct callers/callees, imports, containment |
| `memory_trace` | Downstream call chain from an entry symbol |
| `memory_impact` | Reverse blast radius before an edit: who calls/imports this |
| `memory_flow` | HTTP route execution flow (middleware → handler → downstream), Mermaid/sequence/JSON |
| `memory_flowchart` | Control-flow graph of a single function (branches, loops, returns) |

**Workspace & ops**

| Tool | Use it for |
|------|------------|
| `memory_workspaces_resolve` | Resolve a path to its workspace hash + registration status |
| `memory_workspaces_list` | List registered workspaces with paths, hashes, document counts |
| `memory_status` | Server / DB / embedding-queue health |
| `memory_update` | Trigger a delta re-scan / re-embedding for a workspace |

> Every tool is self-describing over MCP — your client lists each tool's name, parameters, and description on connect.

---

## Architecture

```mermaid
graph LR
    A[Your AI Agent] -->|MCP| B[nano-brain]
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

- **Hybrid search** — BM25 full-text + pgvector HNSW cosine similarity + Reciprocal Rank Fusion + recency decay.
- **Code intelligence** — symbol extraction, cross-file call-chain tracing, reverse-dependency (impact) analysis, and Mermaid flow/flowchart generation. Multi-language, including Ruby/Rails route + control-flow support.
- **Session harvesting** — auto-ingest from OpenCode and Claude Code sessions, map-reduce LLM summarization, incremental harvest with dedup.

Collections: `code` (indexed source + docs), `sessions` (harvested agent sessions), `memory` (agent-written decisions/notes).

---

## Performance

Code-intelligence lookups (`memory_symbols`, `memory_graph`, `memory_impact`, `memory_vsearch`, exact `memory_search`) return in tens of milliseconds — fast enough that agents can run impact analysis on **every** edit rather than skipping it.

### Search quality vs. document-memory tools

| Metric | nano-brain | LlamaIndex | Qdrant/Mem0 |
|--------|------------|------------|-------------|
| P@5 | **80%** | 55% | 27% |
| MRR | **95%** | — | — |
| Code intelligence (symbols / impact / flow) | ✅ | ❌ | ❌ |

Measured on 60 domain-specific queries across three workspaces (a Go daemon, a TypeScript service, and a Rails app). nano-brain is the only one of these with code intelligence — the others target conversation memory and document retrieval.

### Agent-oriented capability benchmarks

Each benchmark runs a deterministic agent workflow — natural-language question → optimized query → symbol lookup — mirroring how an agent actually explores a codebase.

| Workspace | Overall | Multi-tool | Search-QA | Symbol-Lookup |
|-----------|---------|------------|-----------|---------------|
| Go daemon | **1.000** | 1.000 | 1.000 | 1.000 |
| TypeScript service | **0.885** | 1.000 | 0.817 | 1.000 |
| Rails app | **0.795** | 1.000 | 0.726 | 0.667 |

Run them: see the per-suite READMEs under [`benchmarks/`](benchmarks/).

---

## Tech stack

Go 1.23 (single static binary, `CGO_ENABLED=0`) · PostgreSQL 17 + pgvector 0.8.2 (HNSW) · Echo v4 · sqlc · goose · zerolog · koanf · fsnotify · Ollama (or any OpenAI-compatible embeddings provider).

## Configuration

`~/.nano-brain/config.yml`:

```yaml
server:   { host: localhost, port: 3100 }
database: { url: postgres://nanobrain:nanobrain@localhost:5432/nanobrain_dev }
embedding: { provider: ollama, url: http://localhost:11434, model: nomic-embed-text }
search:   { rrf_k: 60, recency_weight: 0.3, limit: 20 }
```

Full options: [docs/CONFIGURATION.md](docs/CONFIGURATION.md).

## REST API

Everything is available over MCP; a REST surface exists for scripting and ops (writes go through MCP). `GET /api/openapi.json` serves the live OpenAPI 3.0 spec — import it into Postman/Swagger.

```bash
# Understand
curl -X POST http://localhost:3100/api/v1/query   -H 'Content-Type: application/json' \
  -d '{"workspace":"<hash>","query":"how does authentication work"}'
# Blast radius
curl -X POST http://localhost:3100/api/v1/graph/impact -H 'Content-Type: application/json' \
  -d '{"workspace":"<hash>","node":"src/auth/login.ts","direction":"in"}'
```

---

## Documentation

- [Setup (agent / manual)](docs/SETUP_AGENT.md)
- [Configuration](docs/CONFIGURATION.md)
- [Architecture](docs/architecture.md)
- [Features](docs/FEATURES.md)
- [Roadmap](docs/ROADMAP.md)
- [Changelog](CHANGELOG.md)

## Contributing

Contributions welcome — open an issue or a PR.

```bash
git clone https://github.com/nano-step/nano-brain.git && cd nano-brain
CGO_ENABLED=0 go build -o nano-brain ./cmd/nano-brain
go test -race -short ./...                    # unit
go test -race -tags=integration ./...         # integration (needs PostgreSQL)
```

Regenerate the OpenAPI spec after route changes: `make generate-openapi`.

## Community

[GitHub Discussions](https://github.com/nano-step/nano-brain/discussions) · [Discord](https://discord.gg/nano-brain)

## License

Licensed under the MIT License.
