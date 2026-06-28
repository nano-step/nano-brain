# External Integrations — nano-brain

Last updated: 2026-06-28

## Overview

nano-brain integrates with external services for embeddings, LLM processing, reranking, and data sources. All external calls use standard HTTP REST APIs with configurable timeouts and retry logic.

## Databases

### PostgreSQL 17 + pgvector

| Detail | Value |
|--------|-------|
| Default URL | `postgres://nanobrain:nanobrain@localhost:5432/nanobrain_dev` |
| Driver | `jackc/pgx/v5` (connection pool) |
| Extensions | pgvector 0.8.2 (HNSW vector index), tsvector/tsquery (BM25) |
| Migrations | 27 goose SQL files in `migrations/` |
| Codegen | sqlc v2 → `internal/storage/sqlc/` |

Purpose: Primary data store for documents, chunks, embeddings, knowledge graph, code symbols, search telemetry.

### SQLite (read-only)

| Detail | Value |
|--------|-------|
| Driver | `modernc.org/sqlite` (pure Go, no CGO) |
| Source | OpenCode session database |
| Access | `internal/harvest/opencode_sqlite.go` |

Purpose: Read-only access to OpenCode's per-project SQLite databases for session harvesting.

## Embedding Providers

### Ollama (default)

| Detail | Value |
|--------|-------|
| Endpoint | `POST {url}/api/embed` |
| Default URL | `http://localhost:11434` |
| Default model | `nomic-embed-text` |
| Dimension | 768 |
| Timeout | 60s |
| Auth | None (local) |

Purpose: Generate vector embeddings for semantic search. Runs locally or in Docker.

### Voyage AI

| Detail | Value |
|--------|-------|
| Endpoint | `POST https://api.voyageai.com/v1/embeddings` |
| Default model | `voyage-3` |
| Dimension | 1024 |
| Timeout | 60s |
| Auth | `Authorization: Bearer {VOYAGE_API_KEY}` |
| Config key | `embedding.voyage_api_key` |

Purpose: Cloud-hosted embedding alternative to Ollama. Requires API key.

## LLM Providers (OpenAI-compatible)

All LLM integrations use the OpenAI chat completions API format (`POST /chat/completions`). Any OpenAI-compatible endpoint works.

### Session Summarization

| Detail | Value |
|--------|-------|
| Endpoint | `{summarization.provider_url}/chat/completions` |
| Default model | `nano-brain` |
| Timeout | 120s |
| Auth | `Authorization: Bearer {summarization.api_key}` |
| Features | SSE streaming, retry (3 attempts, exponential backoff) |

Purpose: Summarize harvested sessions via map-reduce LLM pipeline.

### Code Symbol Summarization

| Detail | Value |
|--------|-------|
| Endpoint | `{code_summarization.provider_url}/chat/completions` |
| Default model | `nano-brain` |
| Timeout | 600s (configurable) |
| Auth | `Authorization: Bearer {code_summarization.api_key}` |
| Features | Batch processing, JSON response format, token budget management |

Purpose: Generate AI summaries for code symbols (functions, types, interfaces). Batched for efficiency.

### HyDE (Hypothetical Document Embedding)

| Detail | Value |
|--------|-------|
| Endpoint | `{search.hyde.provider_url}/chat/completions` |
| Default timeout | 500ms |
| Auth | `Authorization: Bearer {search.hyde.api_key}` |

Purpose: Rewrite search queries as "ideal answer" passages before embedding to improve semantic retrieval.

### Query Preprocessing

| Detail | Value |
|--------|-------|
| Endpoint | `{search.query_preprocessing.provider_url}/chat/completions` |
| Default timeout | 500ms |
| Auth | `Authorization: Bearer {search.query_preprocessing.api_key}` |

Purpose: Classify query intent (keyword/conceptual/temporal), extract time filters, expand queries.

### Intelligence (Categorization + Consolidation)

| Detail | Value |
|--------|-------|
| Endpoint | `{intelligence.provider_url}/chat/completions` |
| Default model | `claude-sonnet-4-5` |
| Auth | `Authorization: Bearer {intelligence.api_key}` |

Purpose: Auto-tag uncategorized documents. Merge semantically similar documents to reduce redundancy.

## Reranking Providers

### Cohere (default)

| Detail | Value |
|--------|-------|
| Endpoint | `POST https://api.cohere.ai/v1/rerank` |
| Default model | `rerank-v4.0-pro` |
| Timeout | 2s |
| Auth | `Authorization: Bearer {search.reranking.api_key}` |
| Features | Retry (3 attempts), min score threshold |

Purpose: Cross-encoder reranking of hybrid search results for improved precision.

### Jina AI

| Detail | Value |
|--------|-------|
| Endpoint | `POST https://api.jina.ai/v1/rerank` |
| Default model | `jina-reranker-v2-base-multilingual` |
| Auth | `Authorization: Bearer {search.reranking.api_key}` |

Purpose: Alternative reranking provider. Selected when `reranking.provider = "jina"`.

## Session Harvesters

### OpenCode Sessions

| Detail | Value |
|--------|-------|
| Sources | SQLite DB (per-project), JSON session files |
| Config | `harvester.opencode.db_root`, `harvester.opencode.db_path`, `harvester.opencode.session_dir` |
| Resolved paths | DB root (new layout) > DB path (legacy) > session dir (legacy) |

Purpose: Ingest AI coding sessions from OpenCode for persistent memory.

### Claude Code Sessions

| Detail | Value |
|--------|-------|
| Source | JSONL session files |
| Config | `harvester.claudecode.session_dir` |
| Toggle | `harvester.claudecode.enabled` |

Purpose: Ingest AI coding sessions from Claude Code for persistent memory.

### Git History

| Detail | Value |
|--------|-------|
| Toggle | `harvester.git.enabled` |
| Access | `internal/harvest/git.go` |

Purpose: Harvest commit messages and file changes from local git history.

## File System

### fsnotify Watcher

| Detail | Value |
|--------|-------|
| Library | `fsnotify/fsnotify` v1.9.0 |
| Config | `watcher.debounce_ms` (2000ms default), `watcher.reindex_interval` (300s) |
| Filters | `watcher.exclude_patterns`, `watcher.allowed_extensions` |
| Per-workspace | `watcher.workspaces.<path>.exclude_patterns` |

Purpose: Watch workspace files for changes and trigger incremental re-indexing.

## MCP Protocol

| Detail | Value |
|--------|-------|
| Library | `modelcontextprotocol/go-sdk` v0.8.0 |
| Transports | Streamable HTTP (`POST /mcp`), SSE (`GET /sse` + `POST /sse`) |
| Tools | 16 registered tools |
| Keep-alive | 30s ping interval |

Purpose: Primary agent interface. All capabilities exposed as MCP tool calls.

## Authentication

| Detail | Value |
|--------|-------|
| Method | HTTP Basic Auth or Bearer token |
| Password | bcrypt hash (configurable) |
| Config | `server.auth.enabled`, `server.auth.users`, `server.auth.tokens` |
| Bypass | `server.auth.bypass_paths` (default: `/health`) |

Purpose: Protect HTTP API in VPS/remote deployments. Optional — disabled by default.

## CI/CD

### GitHub Actions

| Workflow | Trigger | Purpose |
|----------|---------|---------|
| `ci.yml` | Push to master, PRs | Build + test with PG service container |
| `auto-tag.yml` | Push to master | Compute date-based tag (`v{YYYY}.{M}.{DDNN}`) |
| `release.yml` | `v*` tag push | Cross-build 4 binaries + GH Release + npm publish |
| `gemini-review.yml` | PR open/sync | Gemini AI code review |

### npm Publishing

| Detail | Value |
|--------|-------|
| Registry | `https://registry.npmjs.org` |
| Package | `@nano-step/nano-brain` + `nano-brain` (unscoped alias) |
| Auth | `NPM_TOKEN` secret |
| Tag logic | `latest` (stable) or `beta` (pre-release) |

## Docker Compose Services

| Service | Image | Port | Purpose |
|---------|-------|------|---------|
| `postgres` | `pgvector/pgvector:pg17` | 5432 | Primary database |
| `ollama` | `ollama/ollama:latest` | 11434 | Local embeddings |
| `nano-brain` | `./Dockerfile` | 3100 | Application server |

## Webhook / Event Integrations

| System | Mechanism | Purpose |
|--------|-----------|---------|
| Internal event bus | `internal/eventbus` | Decouple reindex notifications |
| fsnotify | File system events | Trigger incremental indexing |
| OpenCode MCP | Streamable HTTP | Agent ↔ daemon communication |

## External Tool Dependencies

| Tool | Used by | Purpose |
|------|---------|---------|
| Tree-sitter | `internal/graph/` | AST parsing for 15+ languages |
| pgvector HNSW | PostgreSQL extension | Vector similarity search |
| tsvector/tsquery | PostgreSQL built-in | BM25 full-text search |
