# Roadmap: nano-brain

## Overview

Strengthen nano-brain's code intelligence and search across v1: add Vue SFC support, fix import-edge resolution, improve search quality and Ruby/Rails parsing, complete flow visualization, add benchmarks/infra, and finish HyDE + documentation.

## Phases

**Phase Numbering:** Integer phases are planned milestone work. Decimal phases (e.g. 2.1) are urgent insertions, marked INSERTED.

- [x] **Phase 1: Vue SFC Support** - Parse .vue files, extract script edges, detect component composition (completed 2026-06-28)
- [ ] **Phase 2: Import Edge Fix** - Resolve unresolved import specifiers to improve memory_impact accuracy
- [ ] **Phase 3: Search Quality** - Reduce false positives, add debugging-intent detection
- [ ] **Phase 4: Ruby/Rails Improvements** - Fix Ruby AST gaps, improve Rails code intelligence
- [ ] **Phase 5: Flow Visualization** - Complete execution-flow pipeline and sequence diagrams
- [ ] **Phase 6: Benchmarks & Infrastructure** - LLM quality benchmarks, OpenAI embeddings, dashboard split
- [ ] **Phase 7: HyDE & Documentation** - Auto-generate HyDE hints, complete docs
- [x] **Phase 8: Session Harvest Unification & Ticket Linking** - Pluggable multi-source harvest, one sessions collection, cross-source/cross-repo ticket linking (completed 2026-06-29)
- [x] **Phase 9: MCP workspace config binding** - Bind a default workspace to the MCP connection via a `?workspace=` URL query param so agents skip manual workspace discovery (completed 2026-07-01, PR #524)

## Phase Details

### Phase 1: Vue SFC Support

**Goal**: Add Vue Single File Component code intelligence — parse .vue files, extract edges, detect component composition.
**Depends on**: Nothing (first phase)
**Requirements**: REQ-CI-01
**Success Criteria** (what must be TRUE):

  1. Parse .vue files with script/template/style blocks
  2. Extract contains, imports, calls edges from script content
  3. Detect component usage via AST tag_name nodes (PascalCase filter)
  4. P@5 ≥ 0.75 baseline maintained (no regression)

**Plans**: 1 plan

Plans:

- [x] 01-01: Vue SFC extractor (parse blocks, two-pass TS/JS edge extraction, component detection)

### Phase 2: Import Edge Fix

**Goal**: Fix import edge target resolution to improve memory_impact accuracy.
**Depends on**: Nothing (parallel with Phase 1)
**Requirements**: REQ-CI-02
**Success Criteria** (what must be TRUE):

  1. Import edges resolve unresolved specifiers
  2. memory_impact returns accurate results for import chains
  3. No false negatives in impact analysis

**Plans**: TBD

Plans:

- [ ] 02-01: TBD (run /gsd-plan-phase 2)

### Phase 3: Search Quality Improvements

**Goal**: Improve search quality and add debugging intent detection.
**Depends on**: Phase 1, Phase 2
**Requirements**: REQ-SQ-01, REQ-SQ-02
**Success Criteria** (what must be TRUE):

  1. Reduce false positives in search results
  2. Detect debugging intent and adjust search behavior
  3. Improve ranking for domain-specific queries

**Plans**: TBD

### Phase 4: Ruby/Rails Improvements

**Goal**: Fix Ruby AST parsing gaps and improve Rails code intelligence.
**Depends on**: Nothing (parallel)
**Requirements**: REQ-RUBY-01, REQ-RUBY-02, REQ-RUBY-03, REQ-RUBY-04
**Success Criteria** (what must be TRUE):

  1. Fix unresolved call edges in Ruby
  2. Improve class indexing accuracy
  3. Support cross-file method calls in Rails
  4. Extract Rails-specific DSL patterns

**Plans**: TBD

### Phase 5: Flow Visualization

**Goal**: Complete execution flow visualization pipeline and sequence diagrams.
**Depends on**: Phase 4
**Requirements**: REQ-FLOW-01, REQ-FLOW-02
**Success Criteria** (what must be TRUE):

  1. Complete execution flow phase 3
  2. Add internal logic to sequence diagrams
  3. Clean up internal labels

**Plans**: TBD

### Phase 6: Benchmarks & Infrastructure

**Goal**: Add LLM quality benchmarks, OpenAI embedding provider, and dashboard split.
**Depends on**: Nothing (parallel)
**Requirements**: REQ-BENCH-01, REQ-BENCH-02, REQ-INFRA-01, REQ-INFRA-02
**Success Criteria** (what must be TRUE):

  1. LLM quality benchmark framework operational
  2. Ruby benchmark comparison complete
  3. OpenAI-compatible embedding provider working
  4. Dashboard split into independent module

**Plans**: TBD

### Phase 7: HyDE & Documentation

**Goal**: Auto-generate HyDE context hints and complete documentation.
**Depends on**: Phase 6
**Requirements**: REQ-CI-04, REQ-DOC-01, REQ-DOC-02
**Success Criteria** (what must be TRUE):

  1. Auto-generate HyDE context hints from project files
  2. Complete MCP agent tool guide
  3. Clean up sequence internal labels

**Plans**: TBD

### Phase 8: Session Harvest Unification & Ticket Linking

**Goal**: Refactor session harvesting into a pluggable multi-source architecture (OpenCode, Claude Code, and future agents like Codex/Cursor behind one adapter interface), unify all sessions into one `sessions` collection, and link sessions across sources and repos by ticket/issue.
**Depends on**: Claude harvester fix (this branch)
**Requirements**: REQ-CI-05
**Success Criteria** (what must be TRUE):

  1. A `SessionSource` adapter interface exists; OpenCode + Claude are adapters; adding a new source needs only a new adapter (no core changes)
  2. A normalized session model carries source, session_id, parent_id, branch, cwd, content
  3. All sources persist to ONE collection (`sessions`); existing `session-summary` docs migrated; memory_wake_up reports correct counts
  4. Each session is tagged with ticket IDs derived from content + branch + parent inheritance
  5. A cross-workspace query returns all sessions for a ticket regardless of source or repo
  6. No regression: existing OpenCode/Claude harvest still works; `go test -race -short ./...` passes

**Plans**: 3/3 plans complete

- [x] 08-01-PLAN.md
- [x] 08-02-PLAN.md
- [x] 08-03-PLAN.md

## Parallel Execution Opportunities

```
Phase 1 (Vue SFC) ─────────┐
Phase 2 (Import Fix) ───────┤
                             ├─→ Phase 3 (Search Quality)
Phase 4 (Ruby/Rails) ───────┤
                             ├─→ Phase 5 (Flow Visualization)
Phase 6 (Benchmarks) ───────┤
                             └─→ Phase 7 (HyDE & Docs)
```

Phases 1–2 parallel. Phases 4 & 6 parallel. Phase 3 depends on 1–2. Phase 5 depends on 4. Phase 7 depends on 6.

## Milestones

### Milestone 1: Vue + Import Fix

- Phase 1: Vue SFC Support ✓
- Phase 2: Import Edge Fix

### Milestone 2: Search + Ruby

- Phase 3: Search Quality
- Phase 4: Ruby/Rails Improvements

### Milestone 3: Flow + Benchmarks

- Phase 5: Flow Visualization
- Phase 6: Benchmarks & Infrastructure

### Milestone 4: Polish

- Phase 7: HyDE & Documentation

## Success Metrics

| Metric | Current | Target |
|--------|---------|--------|
| P@5 (Vue/Nuxt) | 0.75 | ≥ 0.75 (no regression) |
| P@5 (Go) | 1.000 | 1.000 (maintain) |
| P@5 (Ruby) | 0.795 | ≥ 0.85 |
| memory_impact accuracy | Unknown | ≥ 90% |
| Latency (code intel) | ~42ms | < 50ms |

## Backlog

### Phase 999.1: Avoid full reindex on git worktree create / checkout (BACKLOG)

**Goal:** Eliminate the indexing lag on `serve` startup, new-worktree registration, and git checkout. Root cause (confirmed, D-02/D-05): the cheap mtime+size skip is in-memory only, so every file falls through to tree-sitter graph re-extraction on each process start. Fix = persist the fast-path fingerprint (Fix B, D-06b) so unchanged files are skipped after restart, and reorder the content-hash dedup before edge extraction (Fix A, D-06a) so byte-identical content never re-extracts edges (also covers checkout, whose mtime rewrite defeats Fix B). No re-embedding regression (chunk dedup already holds).
**Requirements:** none (backlog perf bugfix; no mapped REQ IDs)
**Plans:** 2/3 plans executed

Notes:

- Reported by user; "feels like" all files get re-indexed on worktree create / checkout — needs verification of the actual trigger before fixing.
- NOT for immediate fix — parked for investigation.
- Investigate: fsnotify behavior on worktree dirs, how the indexer decides what changed, whether checkout bumps mtimes triggering full enqueue.

Plans:

- [x] 999.1-01-PLAN.md — Fix A: reorder content-hash dedup before graph edge extraction (wave 1)
- [x] 999.1-02-PLAN.md — Fix B schema: add documents.mod_time+file_size, extend upsert, add preload query (wave 1)
- [x] 999.1-03-PLAN.md — Fix B wiring: persist fingerprint via os.Stat + warm fileCache from DB at startup (wave 2)

### Phase 9: MCP workspace config binding — bind a default workspace to the MCP connection via a URL query param so agents skip manual workspace discovery

**Goal:** Let a `.mcp.json` MCP server entry bind a single default workspace via a `?workspace=<name-or-hash>` URL query param, so tool calls from that connection can omit the `workspace` argument and still resolve correctly; explicit `workspace` args continue to win, and no-arg + no-default still errors exactly as today.
**Requirements**: none (feature phase, no formal REQ IDs)
**Depends on:** Phase 8
**Plans:** 3/3 plans complete

Plans:

- [x] 09-01-PLAN.md — Core mechanism: WrapStreamableHandler middleware + context-key + requireWorkspace/requireRegisteredWorkspace fallback + routes.go wiring (wave 1)
- [x] 09-02-PLAN.md — Drop "workspace" from required-fields across 14 tool schemas + description updates + schema-assertion test (wave 2)
- [x] 09-03-PLAN.md — Full-HTTP integration test (Pitfall 1) + write-path fallback test + ?workspace= config docs (wave 2)

### Phase 10: Interactive MCP client auto-configuration — after workspace registration, prompt which AI clients (Claude Code, OpenCode, Codex, other) to auto-configure MCP for, writing each client's config with the workspace-bound ?workspace= URL from Phase 9

**Goal:** After `nano-brain init --root=<path>` registers a workspace on a TTY, interactively prompt which AI clients (Claude Code, OpenCode, Codex CLI) to auto-configure, writing each selected client's MCP config bound to `?workspace=<name>` — idempotent, merge-safe, and skipped under `--json`/non-TTY.
**Requirements**: none mapped (feature phase, tracked via issue #525; scoped by CONTEXT.md D-01..D-08)
**Depends on:** Phase 9
**Plans:** 3/3 plans complete

Plans:

- [x] 10-01-PLAN.md — Add workspace `name` to the /api/v1/init response + CLI decode (prerequisite for name-bound URL) (wave 1)
- [x] 10-02-PLAN.md — Config detection + JSON/TOML merge core + 3 client writers + prompt orchestration wired into runInitCmd (wave 2)
- [x] 10-03-PLAN.md — Fix stale OpenCode `type:http` → `type:remote` in docs (SETUP_AGENT.md, README.md, reference-readme.md) (wave 1)

### Phase 12: Add OpenAPI 3.0 spec for the REST API (issue #530) — cover all ~60 existing routes in internal/server/routes.go, served at runtime (GET /api/openapi.json), single source of truth with the route table so drift is caught, not a hand-maintained doc

**Goal:** nano-brain's REST API is self-describing — a generated, drift-checked OpenAPI 3.0 document covering all ~60 routes in internal/server/routes.go is served at GET /api/openapi.json, kept in sync with the route table by an automated test so it can never silently go stale (issue #530).
**Requirements**: D-01..D-06 (CONTEXT.md); issue #530 AC-1..AC-4
**Depends on:** none (independent of Phase 11 — that work lives on a separate unmerged branch)
**Plans:** 4 plans

Plans:
**Wave 1**

- [x] 12-01-PLAN.md — Foundation + Assumption A1 spike (blocking gate): add swaggo/swag + kin-openapi deps, doc.go @securityDefinitions, internal/openapigen generation pipeline (swag → openapi2conv.ToV3), Makefile target, initial committed spec, drift + schema-validation tests [Wave 1] — Assumption A1 PASSED

**Wave 2** *(blocked on Wave 1 completion)*

- [x] 12-02-PLAN.md — Annotate core handler group (workspace/config/doctor/collections/documents/symbols/tags/wakeup/ticket/harvest/reload/events) + protocol-tunnel (/mcp,/sse) presence [Wave 2, parallel with 03]
- [x] 12-03-PLAN.md — Annotate graph/search handler group (query/search/graph/flow/impact/trace/links/embed/reindex/summarize/code-summarize/stats) with per-tier @Security [Wave 2, parallel with 02]

**Wave 3** *(blocked on Wave 2 completion)*

- [ ] 12-04-PLAN.md — Regenerate complete spec, serve at GET /api/openapi.json (public via BypassPaths), route-reconciliation drift test (single source of truth), handler test, docs pointer (README/SETUP_AGENT/CLAUDE.md) [Wave 3]

**Cross-cutting constraints:**

- The whole project still builds and the existing test suite stays green — annotations are pure comments, zero behavior change
