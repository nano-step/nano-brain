# Design Brief — Vue SFC Code Intelligence Support

## Settled Decisions (HIGH confidence)

| Decision | Approach | Basis |
|----------|----------|-------|
| Vue grammar exists | Use `grammars.VueLanguage()` from gotreesitter v0.19.1 — no regex needed | Metis verified in source |
| SFC parsing strategy | Tree-sitter native parsing, NOT regex splitting | **Verified by live parse**: Vue grammar emits `template_element`, `script_element`, `style_element`; script body is a single `raw_text` child |
| No new dependencies | Use existing gotreesitter grammars (Vue + TypeScript/JavaScript) | Already in go.mod |
| Component detection | Use AST `tag_name` nodes (filter PascalCase), **NOT regex on raw text** | **Verified**: grammar already yields `tag_name`="MyChild" inside `element`/`self_closing_tag` — removes Metis's false-positive risk |
| Line number offset | Prefer `tree.RootNodeWithOffset(scriptStartByte, point)` when re-parsing the extracted script; only add a manual `byteOffset` field if that proves insufficient | `RootNodeWithOffset` exists in gotreesitter (`tree.go:2178`) |
| Defer template-level intelligence | No v-if/v-for as CFG nodes, no props/emits tracking in v1 | Both agents agree these are v2 |
| Multiple script blocks | Iterate **all** `script_element` nodes: an SFC may have BOTH `<script setup>` AND `<script>` (Options API) | Both are valid Vue 3 patterns and can coexist in one file |

---

## Architecture Approach

### Two-phase parsing

1. **Phase 1**: Parse `.vue` file with `grammars.VueLanguage()` → AST with template_element, script_element, style_element nodes
2. **Phase 2**: Extract `raw_text` content from script_element → re-parse with `grammars.TypescriptLanguage()` or `JavascriptLanguage()` depending on `lang` attribute

### Key unknown — RESOLVED

Whether gotreesitter's Vue grammar has **language injection** for `<script lang="ts">`.

**Resolved by live parse (2026-06-26): NO injection.** The script body is a single opaque `raw_text` node — the grammar does not parse it as TS/JS. Therefore **two-pass extraction is mandatory**, not optional:

1. Vue parse → locate each `script_element` → read its `raw_text` child + its `lang` attribute
2. Re-parse that byte range with `TypescriptLanguage()` / `JavascriptLanguage()` (see `lang` matrix below)

There is no one-pass branch.

### `lang` attribute → grammar matrix

| `<script lang=...>` | Re-parse with | Notes |
|---------------------|---------------|-------|
| `ts`                | `TypescriptLanguage()` | |
| (absent) / `js`     | `JavascriptLanguage()` | default |
| `tsx`               | `TsxLanguage()` | rare; fall back to TS if unsupported |
| `jsx`               | `JavascriptLanguage()` (jsx) | rare |
| anything else       | skip script extraction, emit `parse_error` | |

---

## Conflict Resolution Log

| Topic | Metis | Oracle | Cross-critique result | Confidence | Decision |
|-------|-------|--------|----------------------|------------|----------|
| Vue grammar exists | Confirmed exists | Assumed doesn't exist | Metis verified in source | **HIGH** | Use tree-sitter, not regex |
| Regex SFC splitting | Unnecessary | Proposed as primary | Metis overrides — breaks codebase patterns | **HIGH** | Use VueLanguage() |
| CFG in v1 | Defer | Include in MVA | Unresolved | **LOW** | **User decision needed** |
| Template detection in v1 | Defer (unified JSX+Vue) | Include as regex | Resolved: include, but via **AST `tag_name`** not regex | **MEDIUM** | Include in v1 (AST-based) |
| NuxtExtractor interaction | Medium risk | Not addressed | Metis only | **MEDIUM** | Define explicitly |
| Language injection for `<script lang="ts">` | HIGH unknown | Not addressed | Verified by live parse: **no injection** | **HIGH** | Two-pass extraction (mandatory) |

---

## Key Risks

| Risk | Source | Confidence | Mitigation |
|------|--------|------------|------------|
| ~~Language injection unverified~~ RESOLVED | Metis (HIGH) | — | Verified no injection; two-pass extraction is the path |
| Line number offset bugs | Both (HIGH) | HIGH | Use `RootNodeWithOffset(scriptStartByte, …)`; test with multi-block SFCs where script starts past line 1 |
| Multiple script blocks missed | New (this review) | MEDIUM | Iterate all `script_element` nodes (setup + Options API can coexist) |
| NuxtExtractor conflict | Metis (MEDIUM) | MEDIUM | New extractor produces ONLY contains/imports/calls edges — NOT http edges |
| Phase 2 parse failure | Metis (MEDIUM) | MEDIUM | Wrap Phase 2 in recover, return Status: "parse_error" CFGs |
| Chunker gap | Metis (MEDIUM) | MEDIUM | Defer to Phase 2 |

---

## Agent Workflow (from research)

### Pattern 1: "What breaks if I change this component?"
```
memory_impact(node="src/components/Button.vue", max_depth=2)
```

### Pattern 2: "What does this component depend on?"
```
memory_graph(node="src/components/Button.vue", direction="out", edge_type="imports")
```

### Pattern 3: "Who uses this composable?"
```
memory_impact(node="src/composables/useAuth.ts", max_depth=3)
```

### Pattern 4: "Show me the component tree from App.vue"
```
memory_trace(node="src/App.vue", max_depth=5)
```
