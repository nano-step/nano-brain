# Requirements: nano-brain

**Defined:** 2026-06-28
**Core Value:** Impact analysis — "What breaks if I change this?" — accurate, sub-50ms

## v1 Requirements

### Code Intelligence

- [x] **REQ-CI-01**: Vue SFC code intelligence support — parse .vue files, extract script-level edges, detect component composition via AST
- [ ] **REQ-CI-02**: Fix import edge target resolution — resolve unresolved specifiers in import edges to improve memory_impact accuracy
- [ ] **REQ-CI-03**: Ruby graph/flowchart/impact improvements — fix Ruby AST parsing gaps for graph, flowchart, impact, deep flow
- [ ] **REQ-CI-04**: Auto-generate HyDE context hints from project files — improve search quality with project-specific context
- [x] **REQ-CI-05**: Pluggable multi-source session harvest + cross-source/cross-repo ticket linking — unify OpenCode/Claude (and future agents) into one sessions collection, link by ticket/branch/parent (Phase 8: 1/3 plans done)

### Search Quality

- [ ] **REQ-SQ-01**: Improve search quality — reduce false positives, improve ranking for domain-specific queries
- [ ] **REQ-SQ-02**: Debugging search mode — detect debugging intent and adjust search behavior

### Benchmarks

- [ ] **REQ-BENCH-01**: LLM quality benchmark framework — measure LLM-generated summary quality across workspaces
- [ ] **REQ-BENCH-02**: Ruby benchmark comparison — compare Ruby code intelligence quality against Go/TypeScript

### Infrastructure

- [ ] **REQ-INFRA-01**: OpenAI-compatible embedding provider — support OpenAI API for embeddings
- [ ] **REQ-INFRA-02**: Dashboard split — separate web dashboard into independent module

### Ruby/Rails

- [ ] **REQ-RUBY-01**: Ruby extractor unresolved calls — fix unresolved call edges in Ruby code intelligence
- [ ] **REQ-RUBY-02**: Ruby class index fix — improve class indexing accuracy
- [ ] **REQ-RUBY-03**: Rails cross-file calls — support cross-file method calls in Rails
- [ ] **REQ-RUBY-04**: Rails DSL extraction — extract Rails-specific DSL patterns (resources, callbacks, etc.)

### Flow Visualization

- [ ] **REQ-FLOW-01**: Execution flow phase 3 — complete execution flow visualization pipeline
- [ ] **REQ-FLOW-02**: Sequence diagram internal logic — add internal logic to sequence diagrams

### Documentation

- [ ] **REQ-DOC-01**: MCP agent tool guide — document MCP tools for agent developers
- [ ] **REQ-DOC-02**: Clean sequence internal labels — improve sequence diagram labeling

## v2 Requirements (Deferred)

- [ ] CFG extraction for .vue files
- [ ] Template-level intelligence (v-if/v-for as CFG nodes)
- [ ] Props/emits tracking
- [ ] Composable usage patterns (useXxx)
- [ ] Store dependency tracking (Pinia/Vuex)
- [ ] Unified JSX+Vue component detection

## Out of Scope

- Multi-tenant SaaS deployment — self-hosted only
- Real-time collaboration — single-agent focus
- Browser-based IDE integration — CLI/MCP only

## Traceability

| Requirement | OpenSpec Change | Harness Story | Status |
|-------------|-----------------|---------------|--------|
| REQ-CI-01 | vue-sfc-code-intelligence | — | Complete |
| REQ-CI-02 | fix-import-target-resolution | — | Active |
| REQ-CI-03 | (issue #486) | — | Pending |
| REQ-CI-04 | (issue #481) | — | Pending |
| REQ-SQ-01 | improve-search-quality | — | In-progress |
| REQ-SQ-02 | debugging-search-mode | — | Pending |
| REQ-BENCH-01 | benchmark-weaknesses | — | Pending |
| REQ-BENCH-02 | ruby-benchmark-comparison | — | Pending |
| REQ-INFRA-01 | (issue #412) | — | Pending |
| REQ-INFRA-02 | dashboard-split | — | In-progress |
| REQ-RUBY-01 | ruby-extractor-unresolved-calls | — | Pending |
| REQ-RUBY-02 | ruby-class-index-fix | — | Pending |
| REQ-RUBY-03 | rails-cross-file-calls | — | Pending |
| REQ-RUBY-04 | rails-dsl-extraction | — | Pending |
| REQ-FLOW-01 | execution-flow-phase3 | — | Pending |
| REQ-FLOW-02 | sequence-diagram-internal-logic | — | Pending |
| REQ-DOC-01 | add-mcp-agent-tool-guide | — | Complete |
| REQ-DOC-02 | clean-sequence-internal-labels | — | Pending |
