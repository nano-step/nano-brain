# Tasks — Static control-flow flowcharts (JS/TS-first)

## Phase 0: Validate gotreesitter for JS/TS (1 day)

- [x] Verify gotreesitter `ChildByFieldName` works on JS/TS grammar for `if_statement`, `switch_statement`, `return_statement`, `block`, `arrow_function`, `function_declaration`
- [x] Write a minimal JS/TS test file and extract AST structure using gotreesitter
- [x] Document any gotreesitter limitations found

## Phase 1a: Enrich existing conditional labels (2-3 days)

### Core changes
- [x] Add `ConditionLabel string` field to `FlowEdge` struct in `internal/flow/builder.go`
- [x] Update `BuildFlow` in `internal/flow/builder.go` to populate `ConditionLabel` from AST metadata (`edgeConditionLabel` reads `metadata["condition_label"]`)
- [x] Update JSON serialization in `internal/server/handlers/flow.go` to include `condition_label`
- [x] Store `condition_label` in `graph_edges.metadata` JSONB during `updateGraphEdges`

### Mermaid renderer
- [x] Update `renderMermaid` in `internal/flow/mermaid.go` to label conditional edges with predicate text
- [x] Handle long labels: truncate at 80 chars with `…` in mermaid output

### Tests
- [ ] Add test: conditional edge carries predicate label
- [ ] Add test: long predicate is truncated with `…`
- [ ] Add test: existing `conditional: true` behavior is preserved

### Verification
- [x] `go build ./...` passes
- [x] `go test -race -short ./...` passes
- [ ] Manual test: `POST /api/v1/graph/flow` with `format:"json"` returns `condition_label` on conditional edges

---

## Phase 1b: JS/TS CFG extractor + storage + API + MCP + dashboard (2 weeks)

### Week 1: CFG types + JS/TS extractor

### Week 1: CFG types + JS/TS extractor

- [x] Define `CFGNode`, `CFGEdge`, `CFG` types in `internal/graph/cflow.go`
- [x] Define `ControlFlowExtractor` interface in `internal/graph/cflow.go` (method is `SupportsCFG`, not `Supports`)
- [x] Implement `JSControlFlowExtractor` in `internal/graph/js_cflow.go`
  - [x] Reuse JS/TS grammar from gotreesitter (already vendored)
  - [x] Implement `buildBlock(block, preds)` recursive descent algorithm
  - [x] Handle `if/else`, `switch`, `for/while/do`, sequential statements, returns/throws/early-exit
  - [x] Handle early-return idiom (`if (err) return; ...`) as terminal node
  - [x] Handle `try/catch` as annotated step
  - [x] Handle ternary expressions as annotated step
  - [x] Implement 500-node cap with `status: "truncated"`
- [x] Register `JSControlFlowExtractor` in the extractor registry (parallel to `JSIntegrationExtractor`) via `graphRegistry.RegisterControlFlowExtractor` in `main.go:409`
- [x] Watcher calls `ExtractCFGs` on JS/TS files during `processFile` via `extractAndUpsertCFGs`
- [x] `go build ./...` passes (verified)
- [x] `internal/graph/js_cflow_test.go` exists with 13 test cases

### Storage

- [x] Create `function_flowcharts` table via goose migration `migrations/00026_function_flowcharts.sql`
- [x] Add sqlc queries: `UpsertFunctionFlowchart`, `GetFunctionFlowchart`, `GetFunctionFlowchartByHandler`, `DeleteFunctionFlowchartsByFile` in `internal/storage/queries/flowcharts.sql`
- [x] Regenerate sqlc code (`sqlc generate`)

### Watcher integration

- [x] `processFile` in `internal/watcher/watcher.go` calls `ExtractCFGs` for JS/TS files (line 698)
- [x] `extractAndUpsertCFGs` calls `DeleteFunctionFlowchartsByFile` before upserting new CFGs
- [x] Minified files skipped via `isMinified` in `CFGRegistry.ExtractCFGs`

### API

- [x] `POST /api/v1/graph/flowchart` handler in `internal/server/handlers/flowchart.go`
- [x] Entry → handler resolution via HTTP edges (reuses flow endpoint logic)
- [x] Query `function_flowcharts` by `(workspace_hash, source_file, start_line, end_line)`
- [x] Return `{ found, entry, method, path, cfg, status }` response

### MCP

- [x] `memory_flowchart` tool in `internal/mcp/flowchart.go`
- [x] Accept `workspace` + `node: "file::startLine-endLine"` format
- [x] Return `{ found, cfg }` or `{ found: false }`

### Docs

- [x] `internal/graph/AGENTS.md` — documents `cflow.go` types + `js_cflow.go` extractor + known limitations
- [x] `internal/server/handlers/AGENTS.md` — already covers handler pattern; flowchart endpoint follows existing convention
- [x] `internal/mcp/AGENTS.md` — updated with memory_flowchart tool
- [x] `internal/storage/AGENTS.md` — already covers migration + query patterns

### Dashboard (separate repo: `@nano-step/nano-brain-dashboard`)

- [ ] Add `Flowchart` toggle to Flow panel alongside Graph and Sequence
- [ ] Implement `Flowchart.tsx` component with spine+gutter layout
- [ ] Render decision nodes as diamonds, steps as boxes, terminals as pills
- [ ] Show branch labels (`yes`/`no`/`case`) on edges
- [ ] Error terminals (`kind:"error"`) in right-hand gutter, success path as vertical spine
- [ ] Fallback to Graph view when `cfg: null` or >30 nodes
- [ ] Use existing design tokens, no new dependencies

### Tests

#### Extractor tests (in `internal/graph/js_cflow_test.go`)
- [x] Test: guard-clause handler produces decisions and terminals (`TestJSControlFlowExtractor_GuardClauseProducesDecisionAndTerminals`)
- [x] Test: function with no branches yields empty CFG (`TestJSControlFlowExtractor_NoBranchesYieldsEmptyCFG`)
- [x] Test: switch with cases produces case-labeled edges (`TestJSControlFlowExtractor_SwitchProducesDecision`)
- [x] Test: early-return idiom collapses to one terminal (`TestJSControlFlowExtractor_EarlyReturnIdiom`)
- [x] Test: file with 5 functions returns 5 CFGs (`TestJSControlFlowExtractor_BatchFiveFunctions`)
- [x] Test: large function (>500 nodes) is truncated (`TestJSControlFlowExtractor_LargeFunctionTruncated`)
- [x] Test: throw terminal (`TestJSControlFlowExtractor_ThrowTerminal`)
- [x] Test: arrow function assigned to variable (`TestJSControlFlowExtractor_ArrowFunctionAssigned`)
- [x] Test: no functions yields empty CFGs (`TestJSControlFlowExtractor_NoFunctions`)
- [x] Test: syntax errors don't panic (`TestJSControlFlowExtractor_SyntaxError`)
- [x] Test: file with only sequential statements (`TestJSControlFlowExtractor_NoBranchesYieldsEmptyCFG`)

#### Storage tests
- [ ] Test: upsert + get round-trip
- [ ] Test: delete by file removes all CFGs for that file
- [ ] Test: re-indexing refreshes CFGs

#### API tests
- [ ] Test: flowchart request returns CFG for JS/TS handler
- [ ] Test: flowchart request returns `cfg: null` for non-JS/TS handler
- [ ] Test: flowchart request returns `found: false` for unknown entry
- [ ] Test: existing flow endpoint is unaffected

#### Dashboard tests (separate repo)
- [ ] Test: Flowchart toggle renders CFG nodes and edges
- [ ] Test: error terminals are visually distinct
- [ ] Test: fallback to Graph view when no CFG
- [ ] Test: long condition labels are truncated in UI

### Verification
- [x] `go build ./...` passes
- [x] `go test -race -short ./...` passes
- [ ] Manual test: `POST /api/v1/graph/flowchart` with `{"entry": "POST /express-app/api/game"}` returns a CFG
- [ ] Manual test: MCP `memory_flowchart` returns a CFG
- [ ] Manual test: Dashboard Flow panel shows Flowchart toggle and renders CFG

---

## Phase 2: Go CFG extractor (future)

- [ ] Implement `GoControlFlowExtractor` in `internal/graph/go_cflow.go`
- [ ] Handle Go-specific patterns: `c.JSON()` + `return`, `if err != nil`, `defer`
- [ ] Register in extractor registry
- [ ] Update watcher to extract CFGs for Go files
- [ ] Tests for Go-specific extraction

## Risks

- gotreesitter `ChildByFieldName` may not work on all JS/TS grammar nodes → redesign extraction if needed
- Watcher concurrency may cause duplicate extractions → use file-level locking
- Large CFGs may slow down API responses → enforce 500-node cap
- `if (err) return;` pattern may vary across JS/TS styles → document supported patterns
- Dashboard layout may not handle all CFG shapes → fallback to Graph view for complex cases
