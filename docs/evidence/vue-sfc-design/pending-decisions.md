# Pending Decisions — Vue SFC Support

Cần bạn quyết định 3 câu hỏi sau trước khi tiếp tục pipeline.

---

## Q1: CFG extraction trong v1 hay defer?

**Recommendation: Defer to v2.**

### Arguments FOR defer (Metis)
- Highest complexity, lowest ROI for v1
- Agents rarely use `memory_flowchart` — barely tested in benchmarks
- Call chains (`memory_trace`) và impact analysis (`memory_impact`) are primary tools
- CFG chỉ useful cho security analysis, taint tracking — không phải typical agent workflow
- Low incremental cost to add later after edges are working

### Arguments FOR include (Oracle)
- Gives `memory_flowchart` for Vue components — key differentiator
- JSControlFlowExtractor already works on JS/TS — just parse `<script>` block content
- Minimal new code if we extend existing CFG builder

### Research Evidence
- Out of 15+ production code intelligence tools researched, only 2 expose CFG as primary tool
- Gortex, CodeGraph, React Graph AI — all focus on inter-procedural call chains, not intra-procedural CFG
- "Agents operate at the inter-procedural level (across functions/files), not the intra-procedural level (within a single function)"

### My Recommendation
**Defer.** Get edges working first. CFG is low-effort addition in Phase 2.

---

## Q2: Template component detection trong v1 hay deferred?

**Recommendation: Include trong v1.**

### Arguments FOR include (Oracle)
- Component composition graph is 10x more valuable than internal logic
- Every Vue/React-specific tool focuses on component composition
- Missing piece: agents can't see parent→child component relationships
- **Detection via AST, not regex**: live parse confirms the Vue grammar already emits `tag_name` nodes (e.g. `"MyChild"`). Walk `tag_name`, filter PascalCase — Metis's false-positive objection (comments/HTML) is moot.

### Arguments FOR defer (Metis)
- Same JSX gap exists for TSX — should be solved once for both
- Regex on raw text creates false positives (comments, HTML elements)
- More complex than it looks

### Research Evidence
- React Graph AI: "Component render hierarchy, Props flow, Hook dependencies, State propagation"
- vue-harvest: "full interface (props, emits, slots), dependency graph, coupling issues"
- ai-dependency-analyzer: explicitly supports Vue, React, Next.js — all focus on dependency graph

### My Recommendation
**Include in v1, AST-based.** Highest-value missing piece. Use AST `tag_name` + PascalCase filter (not regex); unified JSX+Vue solution in v2.

---

## Q3: Vue extractor framework-aware hay universal?

**Recommendation: Universal.**

### Options
- **Universal**: Runs for ALL `.vue` files (Vue CLI, Vite, Nuxt)
- **Framework-aware**: Only runs when `vue` is detected in package.json

### Analysis
- Vue SFC parsing is NOT Nuxt-specific — works for any Vue 3 project
- `NuxtExtractor` already handles Nuxt-specific routing (framework-gated on `"nuxt"`)
- New Vue extractor handles content edges (imports, calls, component composition)
- They coexist via registry's edge dedup logic
- If universal, it works for plain Vue projects too (broader value)

### My Recommendation
**Universal.** Vue SFC parsing is generic. NuxtExtractor stays framework-gated for routes only.

> **Consistency note:** because this is universal, the new content extractor does **not** need a `detectVue` rule in `detector.go` — it implements no `RequiresFrameworks` and runs for every `.vue`. (Earlier plan draft listed "add detectVue" as P0 — removed; implementation-plan item #5 now reflects universal wiring.)

---

## Decision Log

| # | Question | Your Decision | Date |
|---|---------|---------------|------|
| 1 | CFG in v1? | _pending_ | |
| 2 | Template detection in v1? | _pending_ | |
| 3 | Framework-aware or universal? | _pending_ | |
