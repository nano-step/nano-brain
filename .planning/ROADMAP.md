# ROADMAP.md — nano-brain

## Phase 1: Vue SFC Support (High Priority)

**Goal**: Add Vue Single File Component code intelligence — parse .vue files, extract edges, detect component composition.

**Requirements**: REQ-CI-01

**Success Criteria**:
- Parse .vue files with script/template/style blocks
- Extract contains, imports, calls edges from script content
- Detect component usage via AST tag_name nodes (PascalCase filter)
- P@5 ≥ 0.75 baseline maintained (no regression)

**Estimated effort**: 3-5 days

---

## Phase 2: Import Edge Fix (High Priority)

**Goal**: Fix import edge target resolution to improve memory_impact accuracy.

**Requirements**: REQ-CI-02

**Success Criteria**:
- Import edges resolve unresolved specifiers
- memory_impact returns accurate results for import chains
- No false negatives in impact analysis

**Estimated effort**: 2-3 days

---

## Phase 3: Search Quality Improvements (Medium Priority)

**Goal**: Improve search quality and add debugging intent detection.

**Requirements**: REQ-SQ-01, REQ-SQ-02

**Success Criteria**:
- Reduce false positives in search results
- Detect debugging intent and adjust search behavior
- Improve ranking for domain-specific queries

**Estimated effort**: 3-4 days

---

## Phase 4: Ruby/Rails Improvements (Medium Priority)

**Goal**: Fix Ruby AST parsing gaps and improve Rails code intelligence.

**Requirements**: REQ-RUBY-01, REQ-RUBY-02, REQ-RUBY-03, REQ-RUBY-04

**Success Criteria**:
- Fix unresolved call edges in Ruby
- Improve class indexing accuracy
- Support cross-file method calls in Rails
- Extract Rails-specific DSL patterns

**Estimated effort**: 5-7 days

---

## Phase 5: Flow Visualization (Medium Priority)

**Goal**: Complete execution flow visualization pipeline and sequence diagrams.

**Requirements**: REQ-FLOW-01, REQ-FLOW-02

**Success Criteria**:
- Complete execution flow phase 3
- Add internal logic to sequence diagrams
- Clean up internal labels

**Estimated effort**: 4-6 days

---

## Phase 6: Benchmarks & Infrastructure (Low Priority)

**Goal**: Add LLM quality benchmarks, OpenAI embedding provider, and dashboard split.

**Requirements**: REQ-BENCH-01, REQ-BENCH-02, REQ-INFRA-01, REQ-INFRA-02

**Success Criteria**:
- LLM quality benchmark framework operational
- Ruby benchmark comparison complete
- OpenAI-compatible embedding provider working
- Dashboard split into independent module

**Estimated effort**: 4-6 days

---

## Phase 7: HyDE & Documentation (Low Priority)

**Goal**: Auto-generate HyDE context hints and complete documentation.

**Requirements**: REQ-CI-04, REQ-DOC-01, REQ-DOC-02

**Success Criteria**:
- Auto-generate HyDE context hints from project files
- Complete MCP agent tool guide
- Clean up sequence internal labels

**Estimated effort**: 2-3 days

---

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

**Note**: Phases 1-2 can run in parallel. Phases 4-6 can run in parallel. Phase 3 depends on Phases 1-2. Phase 5 depends on Phase 4. Phase 7 depends on Phase 6.

---

## Milestones

### Milestone 1: Vue + Import Fix
- Phase 1: Vue SFC Support
- Phase 2: Import Edge Fix

### Milestone 2: Search + Ruby
- Phase 3: Search Quality
- Phase 4: Ruby/Rails Improvements

### Milestone 3: Flow + Benchmarks
- Phase 5: Flow Visualization
- Phase 6: Benchmarks & Infrastructure

### Milestone 4: Polish
- Phase 7: HyDE & Documentation

---

## Success Metrics

| Metric | Current | Target |
|--------|---------|--------|
| P@5 (Vue/Nuxt) | 0.75 | ≥ 0.75 (no regression) |
| P@5 (Go) | 1.000 | 1.000 (maintain) |
| P@5 (Ruby) | 0.795 | ≥ 0.85 |
| memory_impact accuracy | Unknown | ≥ 90% |
| Latency (code intel) | ~42ms | < 50ms |
