---
title: "Product Brief: nano-brain v2"
status: discovery-complete
created: 2026-05-22
updated: 2026-05-22
project_name: nano-brain-greenfield
facilitator: Sisyphus (BMAD)
user: BMad
language: English
---

# Product Brief: nano-brain v2

> **Status:** Discovery complete. Brief ready for review.
> **North Star (locked 2026-05-22):** Reliability / correctness. Other dimensions are secondary.
> **Recreation thesis (locked 2026-05-22):** Full rewrite of nano-brain — same feature set, new architecture, no refactor.

## Executive Summary

nano-brain v2 is a greenfield rewrite of nano-brain, a persistent memory and code intelligence layer for AI coding agents. v1 proved the concept but accumulated critical correctness bugs, including SQLite corruption under concurrent container access and workspace routing that could return data from the wrong workspace. v2 rebuilds the same feature set on a correctness-first architecture, replacing SQLite with PostgreSQL + pgvector and replacing MCP stdio with an HTTP API, to eliminate entire classes of bugs rather than patch them. The core success criterion for v2 is reliability: zero corruption events, perfect workspace isolation, and search quality that is at least as good as v1.

## The Problem

AI coding agents work best with continuity: memory of past decisions, awareness of the codebase structure, and the ability to recall relevant context without re-reading everything from scratch. nano-brain v1 was built to provide that continuity — a persistent memory and hybrid search layer that agents could write to and query across sessions.

v1 shipped and was used. But two bugs made it unreliable in the exact scenario it was built for: multiple AI agent containers working in parallel.

**SQLite corruption under concurrent access.** SQLite is not designed for concurrent writes from separate processes. When two containers wrote simultaneously, the database could corrupt. Recovery was manual and partial. There was no graceful degradation.

**Workspace routing returning wrong data.** v1 routed queries by workspace, so an agent working in project A should only see data from project A. A routing bug broke this isolation, allowing queries to return data from a different workspace. For an agent relying on memory for context, this is a correctness failure, not a minor UX issue.

These aren't edge cases. Concurrent container access is the primary use pattern. Both bugs sit at the foundation of the product, in the storage layer and the query routing layer. Patching them on top of v1's architecture would be fragile; the foundation needs replacing.

## The Solution

nano-brain v2 is a full greenfield rewrite. The external contract stays the same: agents can harvest session context, write and query a persistent memory store, run hybrid search (BM25 + vector + RRF), and interact via HTTP API or MCP. What changes is everything underneath.

The storage foundation moves from SQLite to PostgreSQL with the pgvector extension. A single PostgreSQL instance handles both metadata/FTS and vector workloads, with pgvector keeping embeddings in the same database.

The interface simplifies. v1 exposed 24 MCP tools primarily via stdio, which introduced lifecycle and buffering bugs specific to the stdio transport. v2 drops MCP stdio and makes the HTTP API the canonical interface. MCP-compatible agents connect via MCP-over-HTTP (SSE), which is debuggable, container-friendly, and avoids the lifecycle issues of stdio. A CLI wrapper remains as a convenience layer over the HTTP API.

Deployment is a single `docker-compose up` that brings up nano-brain and its PostgreSQL dependency together. No external services, no cloud accounts, no configuration ceremony.

## What Makes This Different (v2 vs v1)

Three things v2 is not:

**Not a refactor.** v2 starts fresh — no carry-over of v1's storage layer, transport layer, or internal architecture, because the bugs are architectural, not incidental.

**Not a feature subset.** v2 targets the full v1 feature set staged across two tiers, with a data migration path so existing users don't lose stored memory.

**Not a different product.** The external API contract, agent integration patterns, and core value proposition are unchanged.

What v2 adds is a design constraint that v1 lacked: every architectural decision is evaluated against the correctness criterion first. PostgreSQL over SQLite because MVCC prevents the corruption class. HTTP over stdio because stateless transport removes lifecycle bugs. Success criteria over aspirations because workspace isolation and concurrent write safety are testable and binary.

## Who This Serves

**Primary: developers running AI coding agents on long-lived projects.**

Specifically, someone who uses an AI coding agent (OpenCode, Claude Code, or similar) across multiple sessions on a single codebase and wants the agent to accumulate context over time rather than start cold each session. They're comfortable running Docker Compose locally or on a dev server. They don't want a cloud dependency or a managed service.

**Secondary: teams running parallel AI agent workflows.**

Multiple containers, each running an agent on its own task, sharing a single nano-brain instance. This is the scenario that broke v1 most visibly. v2 is explicitly designed for it.

**Not the target user:**

- Someone who wants a hosted, managed memory service. nano-brain is self-hosted by design.
- Someone building a consumer-facing AI product that needs multi-tenant memory at scale. That's a different architecture problem.
- Someone who needs a GUI-first tool. nano-brain is API-first and CLI-first; agents are the primary consumers, not humans.

The solo-developer-with-AI-agents operating model shapes the project constraints directly: no hard release deadline, small initial scope, quality over speed.

## Success Criteria

These are the v2.0 release gates. All seven must pass before v2 ships.

1. **Zero corruption under concurrent access.** N containers writing simultaneously produce no data loss and no corruption. This is tested by the benchmarking suite, not assumed.

2. **Workspace isolation correct 100%.** A query scoped to workspace A never returns data from workspace B. This is binary: either the invariant holds for all cases, or v2 is not ready.

3. **Search quality >= v1.** Hybrid search results (BM25 + vector + RRF) are benchmarked against v1 baselines. v2 must meet or exceed them. The benchmarking suite ships as part of the product.

4. **All MVP features working end-to-end.** The full harvest → store → search → return pipeline works for all Tier 1 features (session harvesting, hybrid search, per-workspace isolation, file watcher, embedding providers).

5. **Drop-in replacement.** Existing v1 users have a working data migration path. They don't lose stored memory when upgrading.

6. **1-command setup.** `docker-compose up` starts a working nano-brain instance with no additional configuration required.

7. **Documentation complete.** README, HTTP API reference, and migration guide are all published before release.

**Explicitly not a release gate:** test coverage percentage. A coverage number is not a proxy for the correctness properties above. Coverage may be tracked as a leading indicator but won't block release on its own.

## Feature Tiers

It's better to ship fewer features with proven reliability than more features with unverified correctness.

### Tier 1 (MVP v2.0)

These features ship at v2.0 release. All are required for the success criteria to be testable.

- **Session harvesting** — ingest session context from OpenCode and Claude Code; the primary write path for agent memory.
- **Hybrid search** — BM25 full-text search + vector similarity + Reciprocal Rank Fusion (RRF) for result merging. Benchmarked against v1 baselines.
- **Per-workspace isolation** — strict query routing so agents see only their own workspace's data. Tested for correctness, not assumed.
- **File watcher + collection scanning** — automatic ingestion of workspace files without manual triggering.
- **Embedding providers** — Ollama (local, offline) and VoyageAI (cloud). Configurable per deployment.
- **Benchmarking suite** — developer-facing tool for measuring search quality and concurrency correctness. Ships as part of the product, not as an afterthought.
- **Corruption detection and recovery** — built into the storage layer. PostgreSQL MVCC prevents the corruption class; recovery tooling handles the case where something still goes wrong.
- **Chunking strategy** — document chunking required for effective hybrid search. Chunking approach will be determined during technical research.

### Tier 2 (v2.1 and later)

These features require a correct, stable Tier 1 foundation before they're useful. Scope and sequencing within Tier 2 will be determined after v2.0 ships.

- **Code intelligence** — Tree-sitter symbol extraction, call graph analysis, PageRank-based importance scoring, Louvain community detection for module clustering.
- **Knowledge graph** — LLM-assisted entity and relationship extraction from stored memory.
- **Self-learning** — Thompson Sampling for ranking feedback, preference learning from agent interaction patterns.
- **Consolidation** — LLM-driven summarization to compress old memory and reduce retrieval noise over time.
- **Web UI and dashboards** — human-readable view of stored memory, search, and system health. Not required for agent workflows.

## Technical Decisions

Decisions locked during Discovery. Stack language is the one open item, pending formal technical research.

**Database: PostgreSQL + pgvector**
SQLite's concurrency model is incompatible with multi-container concurrent writes. PostgreSQL with MVCC gives proper isolation and eliminates the corruption class entirely. The pgvector extension stores embeddings in the same database, removing a separate vector store dependency. A single PostgreSQL instance serves both metadata/FTS and vector workloads.

**Interface: HTTP API + MCP-over-HTTP (SSE)**
The HTTP API is the canonical interface. All CLI commands are wrappers over it. MCP-compatible agents connect via MCP-over-HTTP using Server-Sent Events, which is stateless and container-friendly. This replaces v1's dual stdio + HTTP SSE setup.

**MCP stdio: removed**
Stdio transport introduced lifecycle and buffering bugs that were specific to the transport mechanism. Removing it eliminates that bug class. Existing MCP clients that support HTTP can connect via the SSE endpoint.

**Application stack: Go (golang)**
Go 1.23+ selected following formal technical research (2026-05-23). Key stack: pgx v5 + pgvector-go for storage, sqlc for compile-time SQL checking, official MCP Go SDK for the protocol layer, goose for migrations, testcontainers-go for integration testing. Runtime race detection via `go test -race` in CI mitigates Go's compile-time race gap vs. Rust. Architecture keeps all shared mutable state in PostgreSQL (via goroutine-safe pgxpool), minimizing in-process concurrency risks. See research doc `docs/research/technical-nano-brain-v2-stack-selection-research-2026-05-22.md` for full evaluation.

**Deployment: Docker Compose**
`docker-compose up` starts nano-brain and PostgreSQL together. No other infrastructure required. Primary use case is AI agent containers connecting to a shared nano-brain instance on the same host or network.

## Non-Goals

These are explicit anti-features for v2. They're not on the roadmap and shouldn't be designed around.

- **GUI-first experience.** nano-brain is API-first and CLI-first. Agents are the primary consumers. A web UI may ship in Tier 2, but it will always be secondary to the HTTP API.
- **Mobile or cross-platform client.** Out of scope entirely.
- **Real-time collaboration / multi-user editing.** nano-brain manages agent memory, not collaborative documents. Multi-user concurrent writes are handled at the infrastructure level (PostgreSQL MVCC), not as a product feature.
- **Cloud-hosted vector databases.** Pinecone, Weaviate Cloud, and similar services are not supported. nano-brain is self-hosted. The vector store runs inside the same PostgreSQL instance.

The following are deferred, not excluded. They remain on the table for later versions:

- Multi-tenant SaaS packaging
- Plugin marketplace for custom embedding providers or search strategies

## Vision

nano-brain v2 should be the memory layer that AI coding agents can actually trust. Not trust in the sense of "it usually works" but in the sense that correctness is provable: workspace isolation holds as an invariant, concurrent writes don't corrupt data, and search results come from where they're supposed to come from.

The broader bet is that reliable agent memory compounds. An agent with a trustworthy memory of past decisions, code patterns, and project context performs meaningfully better on long-lived projects than one starting cold each session. v1 demonstrated that the concept works. v2 makes the infrastructure worthy of that concept.

