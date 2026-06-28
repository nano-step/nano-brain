## 1. Test Fixtures

- [ ] 1.1 Create `internal/graph/testdata/script_setup.vue` — `<script setup lang="ts">` with imports and function calls
- [ ] 1.2 Create `internal/graph/testdata/options_api.vue` — plain `<script>` Options API with methods and data
- [ ] 1.3 Create `internal/graph/testdata/dual_script.vue` — both `<script setup>` and `<script>` in one file
- [ ] 1.4 Create `internal/graph/testdata/no_script.vue` — template + style only, no script block
- [ ] 1.5 Create `internal/graph/testdata/parse_error.vue` — malformed script body (syntax errors)
- [ ] 1.6 Create `internal/graph/testdata/offset.vue` — template before script (script starts at line 400+)
- [ ] 1.7 Create `internal/graph/testdata/component-usage.vue` — template with `<MyChild />` references

## 2. Vue SFC Parser

- [ ] 2.1 Create `internal/graph/vue_sfc_parser.go` — VueSFCParser struct with Parse method
- [ ] 2.2 Implement block splitting — iterate `script_element` nodes, extract `raw_text` + `lang` attribute
- [ ] 2.3 Implement `lang` attribute → grammar matrix (ts→TypescriptLanguage, js→JavascriptLanguage, default→JavascriptLanguage)
- [ ] 2.4 Implement error handling — wrap re-parse in recover, return parse_error status for malformed scripts
- [ ] 2.5 Implement line number offset — use `RootNodeWithOffset(scriptStartByte, point)` for correct line numbers

## 3. Vue SFC Extractor

- [ ] 3.1 Add `EdgeComponentUsage EdgeKind = "component_usage"` to `internal/graph/edge.go` (Momus condition)
- [ ] 3.2 Create `internal/graph/vue_sfc_extractor.go` — VueSFCExtractor struct implementing Extractor interface
- [ ] 3.3 Implement script edge extraction — extract contains, imports, calls edges from re-parsed script content, set `Language: "vue"` on all edges
- [ ] 3.4 Implement component detection — walk `tag_name` nodes, filter PascalCase, create component_usage edges
- [ ] 3.5 Implement component edge deduplication — deduplicate component_usage edges for same child component
- [ ] 3.6 Add universal wiring — register extractor in `registry.go` without `RequiresFrameworks` (runs for all .vue)

## 4. Registry Integration

- [ ] 4.1 Update `internal/graph/registry.go` — add VueSFCExtractor to extractor list
- [ ] 4.2 Verify edge dedup — ensure Vue extractor and NuxtExtractor coexist without duplicate edges
- [ ] 4.3 Verify Vue extractor does NOT produce http edges — only contains, imports, calls, component_usage

## 5. Unit Tests

- [ ] 5.1 Write `TestVueSFCParser` — test block splitting for all fixture files
- [ ] 5.2 Write `TestVueSFCExtractor` — test edge extraction from script blocks
- [ ] 5.3 Write `TestVueComponentDetection` — test component_usage edge creation and dedup
- [ ] 5.4 Write `TestVueLineOffset` — test correct line numbers with offset.vue fixture
- [ ] 5.5 Write `TestVueParseError` — verify parse_error status for malformed scripts
- [ ] 5.6 Write `TestVueDualScript` — verify both script blocks are processed

## 6. Integration Verification

- [ ] 6.1 Run `go build ./...` — verify no compilation errors
- [ ] 6.2 Run `go test -race -short ./internal/graph/ -run TestVue` — all Vue tests pass
- [ ] 6.3 Run `go test -race -short ./...` — no regression in existing tests
- [ ] 6.4 Verify `memory_impact` includes Vue component edges — manual verification with test workspace
- [ ] 6.5 Verify `memory_trace` follows Vue component call chains — manual verification with test workspace
