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
