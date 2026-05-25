# RRI-T Phase 1: PREPARE — nano-brain HTTP API

**Feature:** nano-brain HTTP API (all endpoints)
**Version:** 2026.7.0-rc.19
**Date:** 2026-03-27
**Type:** HTTP REST API (Node.js, no framework)
**Test Scope:** All HTTP endpoints — health, search, write, maintenance, REST v1, MCP transport

---

## Feature Overview

nano-brain exposes an HTTP API server for persistent memory and code intelligence. The server runs inside Docker, serving both direct HTTP endpoints and MCP (Model Context Protocol) transport. Recent fixes addressed POST body reading (was hanging), added query timeout guards, and disabled query expansion by default.

---

## Testable Components

### Core API Endpoints

| Endpoint | Method | Purpose | Key Params |
|----------|--------|---------|------------|
| `/health` | GET | Health check | — |
| `/api/status` | GET | Full index health, models, workspace | — |
| `/api/query` | POST | Hybrid search (FTS + vector + reranking) | `query` (req), `limit`, `tags`, `scope` |
| `/api/search` | POST | BM25 full-text keyword search | `query` (req), `limit` |
| `/api/write` | POST | Write to daily memory log | `content` (req), `tags`, `workspace` |
| `/api/reindex` | POST | Trigger background reindex | `root` |
| `/api/embed` | POST | Trigger background embedding | — |
| `/api/init` | POST | Blocked — always returns 400 | — |
| `/api/maintenance/prepare` | POST | Enter maintenance mode | — |
| `/api/maintenance/resume` | POST | Exit maintenance mode | — |

### REST v1 Endpoints

| Endpoint | Method | Purpose | Key Params |
|----------|--------|---------|------------|
| `/api/v1/status` | GET | Brief status for REST clients | — |
| `/api/v1/workspaces` | GET | List all workspaces | — |
| `/api/v1/search` | GET | REST-style search | `q` (req), `limit`, `workspace` |
| `/api/v1/graph/entities` | GET | Memory entity graph | `workspace` |
| `/api/v1/graph/stats` | GET | Graph statistics | `workspace` |
| `/api/v1/graph/symbols` | GET | Code symbol graph | `workspace` |
| `/api/v1/graph/flows` | GET | Execution flow analysis | `workspace` |
| `/api/v1/graph/connections` | GET | All document connections | `workspace` |
| `/api/v1/graph/infrastructure` | GET | Infrastructure patterns | `workspace` |
| `/api/v1/code/dependencies` | GET | File dependency graph | `workspace` |
| `/api/v1/connections` | GET | Single document connections | `docId` (req), `direction` |
| `/api/v1/telemetry` | GET | Learning analytics | `workspace` |

### MCP Transport

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/sse` | GET | SSE session establishment |
| `/messages` | POST | SSE message delivery (`sessionId` required) |
| `/mcp` | GET/POST | Streamable HTTP transport |

### Web Dashboard

| Endpoint | Method | Purpose |
|----------|--------|---------|
| `/web`, `/web/*` | GET | Static SPA file serving |

---

## Test Environment

### Infrastructure
- **Server**: nano-brain HTTP server (Node.js, `http.createServer`)
- **Runtime**: Docker container, Node.js 22
- **Database**: SQLite (better-sqlite3) at `/root/.nano-brain/db/nano-brain.db`
- **Embedding**: Ollama (currently missing/unreachable)
- **Reranker**: Disabled
- **Expander**: LLM via LiteLLM proxy (`gitlab/claude-haiku-4-5`)
- **Vector Store**: Qdrant at `qdrant:6333`
- **Base URL**: `http://host.docker.internal:3100`

### Current Server State
- Version: 2026.7.0-rc.19
- Documents: 3,522 indexed
- Collections: codebase (3,191), memory (3), zengamingx (328)
- Embedding: **missing** (Ollama unreachable)
- Expansion: **disabled** (config override)
- Reranking: **disabled**

### Known Issues at Test Time
1. Embedding provider unreachable — no vector search, FTS only
2. FTS queries blocking event loop (synchronous better-sqlite3)
3. `/api/query` borderline at 15s timeout with complex queries

---

## Error Handling Architecture

### Body Reading
- `readBody()` helper using `req.on('data')`/`req.on('end')` pattern
- No body size limit
- Empty body → empty string → JSON.parse fails

### Timeout
- `/api/query` only: 15s `Promise.race` guard → 504 Gateway Timeout
- No timeout on `/api/search`, `/api/write`, `/api/reindex`, `/api/embed`

### Maintenance Mode
- `/api/maintenance/prepare` → all endpoints return 503 except `/health` and `/api/maintenance/resume`
- 5-minute auto-resume timeout
- `/api/maintenance/resume` → restores normal operation

### CORS
- Applied to `/api/v1/*` and `/web/*`
- Allowed origins: `localhost`, `127.0.0.1`
- OPTIONS preflight → 204

### Standard Error Response
```json
{ "error": "error message string" }
```

---

## Test Approach

All tests executed via `curl` from within the Docker container against `http://host.docker.internal:3100`. Test results captured as HTTP status code + response body + response time.

### Dimensions to Cover
- **D1: UI/UX** — N/A (no web UI under test, API consumer experience only)
- **D2: API** — Request/response correctness, status codes, content types
- **D3: Performance** — Response times, timeout behavior, concurrent requests
- **D4: Security** — Path traversal, injection, CORS, auth boundaries
- **D5: Data Integrity** — Write persistence, search accuracy, reindex consistency
- **D6: Infrastructure** — Docker networking, maintenance mode, graceful degradation
- **D7: Edge Cases** — Malformed JSON, huge payloads, empty bodies, Unicode, concurrent writes
