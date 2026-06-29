# Phase 1 Context: Vue SFC Support

**Tracking:** nano-step/nano-brain#505
**Date:** 2026-06-28
**Status:** Decisions captured

---

## Domain

Add Vue Single File Component (SFC) code intelligence to nano-brain's graph layer. Parse `.vue` files, extract symbols and edges from script blocks, detect component composition in templates.

---

## Decisions

### Parser Strategy: gotreesitter InjectionParser

**Decision:** Use gotreesitter's `InjectionParser` with Vue grammar to parse SFCs, then inject TypeScript/JavaScript for script blocks.

**Rationale:**
- Vue grammar already ships with gotreesitter v0.19.1 (existing dependency)
- InjectionParser designed exactly for this use case (HTML+JS+CSS, Vue/Svelte)
- Child tree coordinates rebased to document space (line numbers work)
- Incremental reparse: 158× faster than naive reparse
- Reuses existing TypeScriptGraphExtractor queries without modification

**Rejected alternatives:**
- Manual regex: brittle (</script> in strings, multi-line attrs)
- Pure Vue grammar + native TS query: raw_text is opaque, TS queries match nothing
- SetIncludedRanges: fails with root type=ERROR

### Scope: MVP (Script Blocks Only)

**Decision:** Implement script block extraction only. Template component detection deferred to v2.

**Rationale:**
- Highest-value missing piece (agents need script-level symbols/edges)
- Lowest complexity (reuses existing TS/JS queries)
- Template detection is v2 (higher complexity, lower immediate value)

**MVP deliverables:**
1. Parse .vue file with Vue grammar
2. Identify `<script>` and `<script setup>` blocks
3. Inject TypeScript (lang="ts") or JavaScript (default)
4. Extract contains/imports/calls edges from injected script
5. Register in main.go with cfg.Flow.Enabled gating

### Script Block Language Detection

**Decision:** Use static `#set! injection.language "typescript"` for `<script lang="ts">` and default to TypeScript for `<script setup>` without lang attribute.

**Rationale:**
- Vue 3 + `<script setup>` overwhelmingly uses TypeScript
- Static injection simpler than dynamic `@injection.language` capture
- No need to alias-map "ts" → "typescript", "js" → "javascript"

**Implementation:**
```scheme
; TypeScript injection
((script_element
   (start_tag (attribute (attribute_name) @_lang
                         (quoted_attribute_value (attribute_value) @_val)))
   (raw_text) @injection.content
   (#eq? @_lang "lang")
   (#any-of? @_val "ts" "typescript" "tsx"))
 (#set! injection.language "typescript"))

; JavaScript injection (default)
(script_element (raw_text) @injection.content
 (#set! injection.language "javascript"))
```

### Component Detection (MVP scope)

**Decision:** Extract component imports from script blocks only (not template usage).

**Rationale:**
- Import path ending with `.vue` = component (Volar's approach)
- PascalCase filter unreliable (kebab-case imports exist)
- Template detection deferred to v2

**Implementation:**
- Check if import path ends with `.vue` (configurable extensions list)
- Emit `imports` edge: `file.vue` → `./Component.vue`

### Gating: cfg.Flow.Enabled

**Decision:** Register VueExtractor inside `if cfg.Flow.Enabled {` block.

**Rationale:**
- Matches NuxtExtractor pattern (framework-specific code intelligence)
- Vue SFC is a frontend code-intelligence feature, not core graph primitive
- Users opt-in via config

### Edge Schema

**Decision:** Use existing edge kinds (contains, imports, calls) with Language tag "vue".

**Rationale:**
- No new edge kinds needed for MVP
- Registry de-duplicates by `(Kind, SourceNode, TargetNode, Line)`
- Language tag "vue" distinguishes from pure JS/TS edges

**Edge patterns:**
| Kind | SourceNode | TargetNode | Language | Metadata |
|------|------------|------------|----------|----------|
| contains | `<relfile>.vue` | `<relfile>.vue::<symbol>` | typescript/javascript | nil |
| imports | `<relfile>.vue` | `<import path>` | typescript/javascript | nil |
| calls | `<relfile>.vue::<fn>` | `<callee>` | typescript/javascript | nil |

---

## Prior Decisions (from STATE.md)

- **Defer CFG to v2** — lowest ROI, agents use memory_trace/memory_impact more
- **Include template detection** — highest-value missing piece, AST-based detection
- **Universal extractor** — runs for all .vue files, not framework-gated

---

## Canonical Refs

- `internal/graph/edge.go` — Extractor interface, Edge types
- `internal/graph/javascript_extractor.go` — Reference pattern for TS/JS extraction
- `internal/graph/typescript_extractor.go` — Sister to JS extractor
- `internal/graph/nuxtjs_extractor.go` — Existing .vue handler (routing only)
- `internal/graph/registry.go` — Registry pattern, FrameworkAwareExtractor
- `cmd/nano-brain/main.go:325-420` — Wiring site for extractors
- `go.mod` — gotreesitter v0.19.1 dependency

---

## Codebase Context

### Reusable Assets
- `TypeScriptGraphExtractor` — contains/imports/calls queries (reusable on injected script)
- `JavaScriptGraphExtractor` — same pattern for JS
- `lineForByte()` — line number from byte offset (reusable)
- `enclosingFunc()` — find enclosing function for call edges (reusable)
- `funcRange` type — shared by Go/JS/TS/Python/Ruby

### Established Patterns
- Extractor interface: `Supports(ext) bool` + `ExtractEdges(path, content) ([]Edge, error)`
- FrameworkAwareExtractor: optional `RequiresFrameworks() []string`
- Compile-time check: `var _ Extractor = (*MyExtractor)(nil)`
- Constructor injection: `NewXxxExtractor(logger zerolog.Logger) (*XxxExtractor, error)`

### Integration Points
- `cmd/nano-brain/main.go:325-420` — where graphExtractors slice is built
- `graph.NewRegistry(graphExtractors...)` — registry instantiation
- `internal/graph/registry.go:SetActiveFrameworks()` — framework filtering

---

## Deferred Ideas

- **Template component detection** — v2 (detect `<MyComponent />` in template, emit imports edge)
- **Props/emits tracking** — v3 (extract defineProps, defineEmits from script setup)
- **Style block parsing** — out of scope (CSS symbols not in current requirements)
- **Vue-specific edge kinds** — not needed for MVP (existing kinds suffice)

---

*Generated by discuss-phase workflow*
