## Context

nano-brain's code intelligence layer supports Go, Ruby/Rails, and JS/TS — but has **zero support for Vue Single File Components**. Vue/Nuxt workspaces show P@5 of 0.75 (vs 1.000 for Go), primarily because:

1. No `.vue` parsing — the chunker treats them as plain text
2. No script extraction — `<script>` blocks are not re-parsed with TS/JS grammars
3. No component detection — template references to child components are invisible

The gotreesitter v0.19.1 Vue grammar (`grammars.VueLanguage()`) is already available — no new dependencies needed. Live parse verification (2026-06-26) confirmed:

- Vue grammar emits `template_element`, `script_element`, `style_element` nodes
- Script body is a single `raw_text` child — **no language injection**
- Two-pass extraction is mandatory (Vue parse → TS/JS re-parse)

## Goals / Non-Goals

**Goals:**
- Parse `.vue` files and extract script-level symbols and edges
- Detect component composition via AST `tag_name` nodes (PascalCase filter)
- Maintain P@5 ≥ 0.75 baseline (no regression)
- Universal extractor — runs for all `.vue` files (not framework-gated)

**Non-Goals (deferred to v2):**
- CFG extraction for `.vue` files
- Template-level intelligence (v-if/v-for as CFG nodes)
- Props/emits tracking
- Composable usage patterns (useXxx)
- Store dependency tracking (Pinia/Vuex)
- Unified JSX+Vue component detection

## Decisions

### D1: Two-Phase Parsing (verified)

**Decision**: Parse `.vue` with `VueLanguage()`, extract `raw_text` from `script_element`, re-parse with `TypescriptLanguage()` or `JavascriptLanguage()`.

**Alternatives considered:**
- Regex SFC splitting — rejected (breaks codebase patterns, false positives)
- Single-pass with language injection — rejected (Vue grammar has no injection)

**Rationale**: Verified by live parse. The Vue grammar yields opaque `raw_text` for script blocks. Two-pass is the only correct approach.

### D2: Component Detection via AST (not regex)

**Decision**: Use `tag_name` nodes from Vue grammar, filter PascalCase, create `component_usage` edges.

**Alternatives considered:**
- Regex on raw text — rejected (false positives from comments, HTML elements)
- Unified JSX+Vue detection — deferred to v2

**Rationale**: Vue grammar already emits `tag_name`="MyChild" inside `element`/`self_closing_tag`. AST-based detection is reliable; regex is fragile.

### D3: Universal Extractor (not framework-gated)

**Decision**: New Vue extractor runs for ALL `.vue` files. No `detectVue` rule in `detector.go`.

**Alternatives considered:**
- Framework-aware (only when `vue` in package.json) — rejected (plain Vue projects also need support)

**Rationale**: Vue SFC parsing is generic. `NuxtExtractor` stays framework-gated for routes only. They coexist via registry's edge dedup logic.

### D4: Line Number Offset via RootNodeWithOffset

**Decision**: Use `tree.RootNodeWithOffset(scriptStartByte, point)` when re-parsing extracted script blocks. Only add manual `byteOffset` field if insufficient.

**Alternatives considered:**
- Manual byte offset calculation — more error-prone
- Adjust all line numbers post-parse — fragile

**Rationale**: `RootNodeWithOffset` exists in gotreesitter (`tree.go:2178`) and handles offset correctly. Test with `offset.vue` fixture (script starts past line 1).

### D5: Multiple Script Blocks

**Decision**: Iterate ALL `script_element` nodes. An SFC may have BOTH `<script setup>` AND `<script>` (Options API).

**Rationale**: Both are valid Vue 3 patterns and can coexist in one file. Missing either block loses symbols/edges.

### D6: Error Handling for Parse Failures

**Decision**: Wrap Phase 2 re-parse in `recover`, return `Status: "parse_error"` for that script block. Don't crash the extractor.

**Rationale**: Malformed `<script>` blocks should not crash the indexer. Graceful degradation is better than failure.

## Risks / Trade-offs

| Risk | Mitigation |
|------|------------|
| Line number offset bugs with multi-block SFCs | Use `RootNodeWithOffset`; test with `offset.vue` fixture (script starts at line 400+) |
| NuxtExtractor conflict | New extractor produces ONLY contains/imports/calls — NOT http edges; registry dedup handles coexistence |
| Multiple script blocks missed | Iterate all `script_element` nodes; test with `dual_script.vue` fixture |
| Phase 2 parse failure | Wrap in recover, return parse_error status; test with `parse_error.vue` fixture |
| Chunker gap (Phase 1 limitation) | Defer `.vue` chunker case to Phase 2; search baseline won't improve until then |
| Symbol extraction gap (Phase 1 limitation) | Defer `.vue` symbol extractor to Phase 2; `memory_symbols` returns nothing for `.vue` until then |

## Implementation Phases

### Phase 1 (MVP): Edges + Component Graph

Parse `.vue` files, extract script-level symbols and component composition edges.

**Key files:**
- `internal/graph/vue_sfc_parser.go` (new) — Vue SFC block splitter
- `internal/graph/vue_sfc_extractor.go` (new) — Script re-parsing, edge extraction, component detection
- `internal/graph/registry.go` — Wire new extractors (universal, no `RequiresFrameworks`)

**Test fixtures:**
- `script_setup.vue` — `<script setup lang="ts">`
- `options_api.vue` — plain `<script>` Options API
- `dual_script.vue` — both `<script>` and `<script setup>`
- `no_script.vue` — template + style only
- `parse_error.vue` — malformed script body
- `offset.vue` — template before script (line number test)

### Phase 2: CFG + Symbols

Add control flow graphs and symbol extraction for `.vue` files.

**Key files:**
- `internal/graph/vue_sfc_cflow.go` (new) — CFG extraction
- `internal/symbol/vue_sfc_extractor.go` (new) — Symbol extraction
- `internal/chunker/dispatcher.go` — Add `.vue` case
- `internal/server/handlers/reindex_cfg.go` — Add `.vue` to `cfgExts`

### Phase 3 (v2): Template Intelligence

Deep template analysis, props/emits tracking, composable patterns.

**Deferred items:**
- Props/emits tracking from `<script setup>`
- `useFetch`/`useAsyncData` patterns
- Composable tracking (useXxx with package filtering)
- Store dependency tracking (Pinia/Vuex)
- Unified JSX+Vue component detection

## Migration Plan

1. **Phase 1**: Add Vue SFC parser + extractor → wire into registry
2. **Phase 2**: Add CFG + symbol extraction → update chunker
3. **Verification**: Re-run Vue-workspace benchmark, confirm P@5 ≥ 0.75 baseline

**Rollback**: Remove new extractor files, revert registry changes. No data migration needed.

## Open Questions

None — all decisions resolved via deep-design pipeline (Phase 1-2) and user decisions (Q1-Q3).
