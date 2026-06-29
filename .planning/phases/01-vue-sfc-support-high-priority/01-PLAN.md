# Phase 1 Plan: Vue SFC Support

**Tracking:** nano-step/nano-brain#505
**Date:** 2026-06-28
**Status:** Ready for execution

---

## Goal

Add Vue Single File Component (SFC) code intelligence to nano-brain's graph layer. Parse `.vue` files using gotreesitter's InjectionParser, extract contains/imports/calls edges from script blocks, and detect component imports.

---

## Success Criteria

1. Parse .vue files with `<script>`, `<script setup>`, `<template>`, `<style>` blocks
2. Extract contains edges from script blocks (file → symbols)
3. Extract imports edges from script blocks (file → import paths)
4. Extract calls edges from script blocks (function → callees)
5. Detect component imports (import path ending with `.vue`)
6. No regression: P@5 ≥ 0.75 baseline maintained
7. Tests pass: `go test -race -short ./...`

---

## Tasks

### Task 1: Create VueExtractor skeleton

**File:** `internal/graph/vue_sfc_extractor.go`

**Goal:** Implement Extractor interface for .vue files using InjectionParser.

**Steps:**
1. Create `VueExtractor` struct with fields:
   - `logger zerolog.Logger`
   - `vueLang *gotreesitter.Language`
   - `tsLang *gotreesitter.Language`
   - `jsLang *gotreesitter.Language`
   - `ip *gotreesitter.InjectionParser`
   - `tsContainsQ *gotreesitter.Query`
   - `tsImportQ *gotreesitter.Query`
   - `tsCallFuncQ *gotreesitter.Query`
   - `tsCallExprQ *gotreesitter.Query`
   - `jsContainsQ *gotreesitter.Query`
   - `jsImportQ *gotreesitter.Query`
   - `jsCallFuncQ *gotreesitter.Query`
   - `jsCallExprQ *gotreesitter.Query`

2. Implement constructor `NewVueSFCExtractor(logger zerolog.Logger) (*VueSFCExtractor, error)`:
   - Load `grammars.VueLanguage()`, `grammars.TypescriptLanguage()`, `grammars.JavascriptLanguage()`
   - Create `gotreesitter.NewInjectionParser()`
   - Register Vue grammar: `ip.RegisterLanguage("vue", vueLang)`
   - Register injection queries (TypeScript and JavaScript)
   - Pre-compile TS/JS queries (reuse from TypeScriptGraphExtractor pattern)

3. Implement `Supports(ext string) bool`:
   - Return `ext == ".vue"`

4. Implement `ExtractEdges(filePath string, content []byte) ([]Edge, error)`:
   - Parse with `ip.Parse(content, "vue")`
   - Iterate `result.Injections`
   - For each injection with non-nil Tree:
     - Switch on `inj.Language` (typescript/javascript)
     - Call extract helpers for contains/imports/calls
   - Return concatenated edges

5. Add compile-time check: `var _ Extractor = (*VueSFCExtractor)(nil)`

**Acceptance criteria:**
- File compiles without errors
- Constructor loads languages and creates InjectionParser
- Supports(".vue") returns true
- ExtractEdges returns nil, nil for empty input

---

### Task 2: Implement injection queries

**Goal:** Register tree-sitter queries for Vue SFC script block injection.

**Steps:**
1. Define TypeScript injection query:
```scheme
((script_element
   (start_tag (attribute (attribute_name) @_lang
                         (quoted_attribute_value (attribute_value) @_val)))
   (raw_text) @injection.content
   (#eq? @_lang "lang")
   (#any-of? @_val "ts" "typescript" "tsx"))
 (#set! injection.language "typescript"))
```

2. Define JavaScript injection query (default):
```scheme
(script_element (raw_text) @injection.content
 (#set! injection.language "javascript"))
```

3. Register both queries with InjectionParser

**Acceptance criteria:**
- TypeScript injection triggers for `<script lang="ts">`
- JavaScript injection triggers for `<script>` without lang
- No duplicate injections for same script block

---

### Task 3: Extract contains edges

**Goal:** Extract symbol definitions from injected script blocks.

**Steps:**
1. Define contains query (reuse TypeScriptGraphExtractor pattern):
```scheme
(function_declaration name: (identifier) @name) @decl
(class_declaration name: (type_identifier) @name) @decl
(lexical_declaration (variable_declarator name: (identifier) @name)) @decl
```

2. Implement `extractContainsEdges` helper:
   - Execute query against injected tree
   - For each match: emit EdgeContains with SourceNode=<file>, TargetNode=<file>::<name>
   - Use `lineForByte()` for line numbers

**Acceptance criteria:**
- Functions, classes, variables detected in script blocks
- Line numbers correct (document-relative)
- Edges have Language="typescript" or "javascript"

---

### Task 4: Extract imports edges

**Goal:** Extract import statements from injected script blocks.

**Steps:**
1. Define imports query:
```scheme
(import_statement source: (string) @path)
```

2. Implement `extractImportsEdges` helper:
   - Execute query against injected tree
   - For each match: emit EdgeImports with SourceNode=<file>, TargetNode=<import path>
   - Strip quotes from path

**Acceptance criteria:**
- ES6 imports detected
- Import paths extracted correctly
- Component imports (path ending with .vue) included

---

### Task 5: Extract calls edges

**Goal:** Extract function calls from injected script blocks.

**Steps:**
1. Define call queries (reuse TypeScriptGraphExtractor pattern):
```scheme
; Function declarations for scope tracking
(function_declaration name: (identifier) @name) @func
(lexical_declaration (variable_declarator name: (identifier) @name (arrow_function))) @func

; Call expressions
(call_expression function: (identifier) @callee)
(call_expression function: (member_expression property: (property_identifier) @callee))
```

2. Implement `extractCallEdges` helper:
   - First pass: collect function ranges (name, startByte, endByte)
   - Second pass: for each call, find enclosing function via `enclosingFunc()`
   - Emit EdgeCalls with SourceNode=<file>::<enclosingFn>, TargetNode=<callee>
   - Add `{"conditional": true}` metadata for calls inside if blocks

**Acceptance criteria:**
- Function calls detected
- Enclosing function correctly identified
- Conditional calls marked in metadata

---

### Task 6: Add component import detection

**Goal:** Detect imports ending with `.vue` as component imports.

**Steps:**
1. In `extractImportsEdges`, check if import path ends with `.vue`
2. Add metadata `{"component": true}` for Vue imports
3. Emit additional EdgeImports with TargetNode=<component name> (extracted from path)

**Acceptance criteria:**
- `import Foo from './Foo.vue'` emits component metadata
- Component name extracted correctly (Foo from ./Foo.vue)
- Non-Vue imports unaffected

---

### Task 7: Create tests

**File:** `internal/graph/vue_sfc_extractor_test.go`

**Goal:** Verify VueExtractor works correctly.

**Steps:**
1. Create test fixture: `.vue` file with:
   - `<script setup lang="ts">` block
   - Import statements
   - Function definitions
   - Function calls
   - Component import

2. Write test cases:
   - TestSupports: `.vue` returns true, `.ts` returns false
   - TestExtractEdges: verify contains/imports/calls edges
   - TestComponentDetection: verify component imports detected
   - TestLineNumbers: verify correct line numbers
   - TestEmptyScript: verify no edges for empty script
   - TestMalformedSFC: verify graceful handling

3. Run tests: `go test -race -short ./internal/graph/...`

**Acceptance criteria:**
- All tests pass
- Coverage ≥ 80% for new code
- No race conditions

---

### Task 8: Wire into main.go

**File:** `cmd/nano-brain/main.go`

**Goal:** Register VueExtractor in the graph registry.

**Steps:**
1. Find wiring block (lines 325-420)
2. Add inside `if cfg.Flow.Enabled {` block:
```go
if vueGE, err := graph.NewVueSFCExtractor(logger); err != nil {
    logger.Warn().Err(err).Msg("vue sfc extractor init failed, skipping")
} else {
    graphExtractors = append(graphExtractors, vueGE)
    logger.Info().Msg("graph: vue sfc extractor enabled")
}
```

3. Verify compilation: `go build ./...`

**Acceptance criteria:**
- Binary compiles without errors
- Log message appears when Vue files are indexed
- No interference with existing extractors

---

### Task 9: Integration test

**Goal:** Verify end-to-end with a real .vue file.

**Steps:**
1. Create test workspace with sample .vue files
2. Start nano-brain server
3. Index test workspace
4. Query memory_graph for .vue file
5. Verify edges returned

**Acceptance criteria:**
- Edges stored in database
- memory_graph returns correct edges
- No errors in logs

---

## Dependencies

- gotreesitter v0.19.1 (existing dependency)
- TypeScriptGraphExtractor (existing, reuse queries)
- JavaScriptGraphExtractor (existing, reuse queries)
- lineForByte, enclosingFunc, funcRange (existing helpers)

## Risks

| Risk | Mitigation |
|------|------------|
| Vue grammar version drift | gotreesitter ships grammar as embedded blob, update dep to pull updates |
| InjectionParser not thread-safe | Wrap in sync.Mutex or create per-goroutine instance |
| Multiple script blocks | InjectionParser supports multiple injections, iterate all |
| Empty script blocks | Skip if raw_text has 0-byte range |
| Malformed SFCs | Don't gate on HasError, try injection query anyway |

---

## Evidence

- `/tmp/opencode/vue-probe-final.txt` — proof-of-concept probe output
- Probe shows: HasError=false, injection language=typescript, TS symbols recovered
- Incremental reparse verified: 158× faster than naive

---

*Generated by plan-phase workflow*
