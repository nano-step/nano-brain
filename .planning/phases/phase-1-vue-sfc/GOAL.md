## Why

nano-brain's code intelligence layer currently has **zero support for Vue Single File Components (.vue)**. This means:

- **No call graph edges** from `.vue` files — agents can't trace component dependencies
- **No component composition graph** — agents can't see parent→child relationships
- **No symbol extraction** — `memory_symbols` returns nothing for `.vue` files
- **Impact analysis fails** — `memory_impact` misses all Vue component edges

Vue/Nuxt workspaces show **P@5 of 0.75** (vs 1.000 for Go). Adding Vue SFC support is the highest-impact improvement for Vue/Nuxt agent workflows.

**Why now**: The gotreesitter v0.19.1 Vue grammar is already available — no new dependencies needed. The two-pass extraction approach has been verified via live parse.

## What Changes

- **New Vue SFC parser** — splits `.vue` files into template/script/style blocks using tree-sitter
- **Script block re-parsing** — extracts symbols and edges from `<script>` using existing TS/JS extractors
- **Component detection** — identifies `<MyChild />` references in template via AST `tag_name` nodes (PascalCase filter)
- **Universal extractor** — runs for ALL `.vue` files (not framework-gated)
- **Edge types**: `contains`, `imports`, `calls`, `component_usage` (template→child)

**Deferred to v2** (per user decision):
- CFG extraction for `.vue` files
- Template-level intelligence (v-if/v-for as CFG nodes)
- Props/emits tracking
- Composable usage patterns

## Capabilities

### New Capabilities

- `vue-sfc-parsing`: Vue Single File Component parsing — splits .vue files into blocks, re-parses script content with TS/JS grammars, extracts symbols and edges
- `vue-component-detection`: Template component detection — identifies child component references via AST tag_name nodes, creates component_usage edges

### Modified Capabilities

- `code-intelligence`: Add .vue support to existing code intelligence pipeline (edges, symbols, impact analysis)

## Impact

**Affected code:**
- `internal/graph/` — new Vue SFC parser and extractor files
- `internal/graph/registry.go` — wire new extractors
- `internal/symbol/` — add .vue symbol extraction
- `internal/chunker/dispatcher.go` — add .vue case (Phase 2)

**APIs:**
- `memory_impact` — will now include Vue component edges
- `memory_trace` — will now follow Vue component call chains
- `memory_symbols` — will now return Vue component symbols
- `memory_graph` — will now show Vue import/component edges

**Dependencies:**
- No new dependencies — uses existing `grammars.VueLanguage()` from gotreesitter v0.19.1

**Systems:**
- Vue/Nuxt workspaces will see improved search quality (P@5 target: ≥0.75 baseline maintained)
- Component composition graph becomes visible to agents
