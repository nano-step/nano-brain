# Implementation Plan — Vue SFC Support

## Phase 1 (MVP): Edges + Component Graph

**Goal**: Parse .vue files, extract script-level symbols and component composition edges.

| # | Feature | File | Priority | Effort | Verification |
|---|---------|------|----------|--------|--------------|
| 1 | Vue SFC block splitter — iterate **all** `script_element` nodes, read `raw_text` + `lang` attr | `internal/graph/vue_sfc_parser.go` (new) | P0 | Quick | Unit test with fixture .vue files |
| 2 | Script block → re-parse per `lang` matrix (reuse existing TS/JS extractors) | `internal/graph/vue_sfc_extractor.go` (new) | P0 | Medium | `go test -race -short ./internal/graph/ -run TestVue` |
| 3 | contains/imports/calls edges from script | `internal/graph/vue_sfc_extractor.go` | P0 | Medium | Edge extraction tests |
| 4 | Template component detection via **AST `tag_name` nodes** (filter PascalCase), NOT regex | `internal/graph/vue_sfc_extractor.go` | P0 | Low | Component usage edge tests (incl. comment/HTML-tag false-positive cases) |
| 5 | Extractor is **universal** (runs for all `.vue`, see Q3) — no `detectVue` gate needed for content edges | `internal/graph/registry.go` | P0 | Quick | Runs on plain-Vue + Nuxt fixtures |
| 6 | Wire into main.go / registry | `cmd/nano-brain/main.go`, `registry.go` | P0 | Quick | `go build ./...` |

### Required test fixtures (`internal/graph/testdata/`)

None exist yet — create before implementing:

- `script_setup.vue` — `<script setup lang="ts">`
- `options_api.vue` — plain `<script>` Options API
- `dual_script.vue` — both `<script>` and `<script setup>` in one file
- `no_script.vue` — template + style only
- `parse_error.vue` — malformed script body (must yield `parse_error`, not crash)
- `offset.vue` — template before script so script starts past line 1 (line-number test)

### Acceptance Criteria

```bash
# Parse a .vue file and verify edges
go test -race -short ./internal/graph/ -run TestVue

# Verify no regression
go test -race -short ./...

# Verify build
go build ./...
```

---

## Phase 2: CFG + Symbols

**Goal**: Add control flow graphs and symbol extraction for .vue files.

| # | Feature | File | Priority | Effort |
|---|---------|------|----------|--------|
| 1 | `.vue` CFG extraction | `internal/graph/vue_sfc_cflow.go` (new) | P1 | Low |
| 2 | Add `.vue` to CFG reindex allowlist | `internal/server/handlers/reindex_cfg.go` (`cfgExts` map) | P1 | Quick |
| 3 | Vue symbol extraction | `internal/symbol/vue_sfc_extractor.go` (new) | P1 | Short |
| 4 | Chunker support for `.vue` (add case to switch) | `internal/chunker/dispatcher.go` | P2 | Quick |
| 5 | Line offset for CFG: use `RootNodeWithOffset(scriptStartByte, …)`; add `byteOffset` field only if insufficient | `internal/graph/cflow.go` | P1 | Quick |

> **v1 limitation to communicate:** until Phase 2 lands, `.vue` has no chunker case (`dispatcher.go:24` switch lacks `.vue`) and no symbol extractor → `.vue` files chunk as plain text and `memory_symbols` returns nothing for them. The Vue-workspace search baseline (828ms / P@5 0.75) won't improve from symbols in v1.

### Acceptance Criteria

```bash
# CFG extraction
go test -race -short ./internal/graph/ -run TestVueCFG

# Symbol extraction
go test -race -short ./internal/symbol/ -run TestVue

# Full regression
go test -race -short ./...
```

---

## Phase 3 (v2): Template Intelligence

**Goal**: Deep template analysis, props/emits tracking, composable patterns.

| # | Feature | Priority | Effort |
|---|---------|----------|--------|
| 1 | Props/emits tracking from `<script setup>` | P2 | Medium |
| 2 | `useFetch`/`useAsyncData` specific patterns | P2 | Medium |
| 3 | Composable tracking (useXxx with package filtering) | P2 | Medium |
| 4 | Store dependency tracking (Pinia/Vuex) | P2 | Medium |
| 5 | Unified JSX+Vue component detection | P2 | Medium |

---

## Key Files

| File | Current State | Changes Needed |
|------|---------------|----------------|
| `internal/graph/nuxtjs_extractor.go` | Route edges only | Keep as-is, no changes |
| `internal/graph/js_cflow.go` | JS/TS CFG, no .vue | Add .vue support (Phase 2) |
| `internal/graph/typescript_extractor.go` | TS edges | Reuse for script block parsing |
| `internal/graph/javascript_extractor.go` | JS edges | Reuse for script block parsing |
| `internal/graph/detector.go` | `detectNuxt` only, no `detectVue` | No change — Vue extractor is universal (Q3), not framework-gated |
| `internal/graph/registry.go` | No .vue extractors | Wire new extractors (universal, no `RequiresFrameworks`) |
| `internal/graph/cflow.go` | No byteOffset | Prefer `RootNodeWithOffset`; add `byteOffset` field only if needed (Phase 2) |
| `internal/server/handlers/reindex_cfg.go` | `cfgExts` lacks `.vue` | Add `.vue` (Phase 2) |
| `internal/chunker/dispatcher.go` | switch lacks `.vue` | Add `.vue` case (Phase 2) |

---

## Dependencies

- `grammars.VueLanguage()` — already in gotreesitter v0.19.1
- `grammars.TypescriptLanguage()` — already available
- `grammars.JavascriptLanguage()` — already available
- No new go.mod dependencies needed

---

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| ~~Language injection unverified~~ RESOLVED | Verified no injection — two-pass extraction is the implementation |
| Line number offset bugs | Use `RootNodeWithOffset`; test with `offset.vue` (script past line 1) |
| Multiple script blocks | Iterate all `script_element` nodes (`dual_script.vue` fixture) |
| NuxtExtractor conflict | New extractor produces ONLY contains/imports/calls — NOT http edges |
| Parse failure | Wrap re-parse in recover, return Status: "parse_error" |

## Post-implementation gate

Beyond `go test`, re-run the Vue-workspace benchmark and confirm P@5 ≥ 0.75 baseline holds (no regression) and that component/import edges now resolve. Symbol/search uplift is **not** expected until Phase 2.
