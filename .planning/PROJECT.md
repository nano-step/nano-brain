# PROJECT.md — nano-brain

## What This Is

nano-brain is an **agent-oriented memory and code intelligence daemon** for AI coding agents. It provides persistent context, impact analysis, and call-chain tracing via MCP (Model Context Protocol).

**Core thesis:** Agents don't read docs — they call tools. Every capability is exposed as an MCP tool or REST endpoint.

## What This Does

- **Session memory** — Harvest, index, and retrieve context across coding sessions
- **Code intelligence** — Symbol graph, call chains, impact analysis, flow diagrams
- **Hybrid search** — BM25 + vector + RRF fusion for high-recall retrieval
- **Multi-runtime** — Works with OpenCode, Claude Code, Cursor, any MCP client

## Core Value

The ONE thing that must work: **Impact analysis** — "What breaks if I change this?" must return accurate, sub-50ms results for any file in the workspace.

## Constraints

- **Performance** — Sub-50ms latency for code intelligence tools (agents skip slow tools)
- **Self-hosted** — No cloud dependency, user data stays local
- **Single binary** — `CGO_ENABLED=0` static build, npm wrapper for distribution
- **Backward compatible** — No breaking changes to existing MCP tools

## Requirements

### Validated

- ✓ Hybrid search (BM25 + vector + RRF) — existing
- ✓ Session harvesting (OpenCode, Claude Code) — existing
- ✓ Go/Ruby/JS/TS code intelligence — existing
- ✓ MCP protocol support (16 tools) — existing
- ✓ REST API endpoints — existing
- ✓ PostgreSQL + pgvector storage — existing
- ✓ Embedding queue + providers — existing
- ✓ File system watcher — existing

### Active

- [ ] Vue SFC code intelligence support (proposal ready, OpenSpec created)
- [ ] Fix import edge target resolution (issue #501, high-risk bug-fix)
- [ ] Ruby graph/flowchart/impact improvements (issue #486)
- [ ] Auto-generate HyDE context hints (issue #481)
- [ ] LLM quality benchmark framework (issue #458)
- [ ] OpenAI-compatible embedding provider (issue #412)

### Out of Scope

- Multi-tenant SaaS deployment — self-hosted only
- Real-time collaboration — single-agent focus
- Browser-based IDE integration — CLI/MCP only

## Key Decisions

| Decision | Rationale | Outcome |
|----------|-----------|---------|
| MCP over REST for agents | Agents call tools, not HTTP endpoints | — Active |
| PostgreSQL + pgvector | Mature, reliable, pgvector for semantic search | — Active |
| Tree-sitter for code parsing | Fast, accurate, multi-language support | — Active |
| Constructor injection (no DI) | Simplicity, explicit dependencies | — Active |
| Single binary deployment | Easy installation via npm/npx | — Active |

## Architecture

See `.planning/codebase/ARCHITECTURE.md` for detailed architecture documentation.

**Key patterns:**
- Service daemon with errgroup goroutine orchestration
- Registry pattern for pluggable language extractors
- Event bus for async pub/sub (watcher, embed queue, flow materializer)
- Pipeline architecture: file → chunk → embed → search

## Evolution

This document evolves at phase transitions and milestone boundaries.

**After each phase transition:**
1. Requirements invalidated? → Move to Out of Scope with reason
2. Requirements validated? → Move to Validated with phase reference
3. New requirements emerged? → Add to Active
4. Decisions to log? → Add to Key Decisions
5. "What This Is" still accurate? → Update if drifted

---
*Last updated: 2026-06-28 after initialization*
