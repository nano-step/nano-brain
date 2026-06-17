## Why

The "Flow" view currently renders a **call graph** (function → function) and a sequence diagram derived from it. Both show *structure* (who-calls-whom), not the *logic* of a request: there is no representation of conditions, branches, or outcomes. Users looking at a flow cannot answer "what decisions does this endpoint make and where does each path lead" — e.g. `request → ‹authenticated?› → no → 401 / yes → continue`.

The data to answer this exists in the source AST (every `if`/`switch`/`return`), but the graph extractors only record a `conditional` *boolean* on call edges — they discard the predicate, the branch structure, and the terminal outcomes. We choose **static control-flow extraction** over LLM inference: the structure is deterministically present in the code, so we should derive it exactly rather than spend tokens approximating it.

### Deep-Design Findings

A multi-agent deep-design pipeline (Metis + Oracle + cross-critique + Momus sanity) identified critical issues with the original proposal:

1. **JS/TS-first for zengamingx** — zengamingx workspace is the main project that must be supported first. JS/TS CFG extraction ships before Go.
2. **Anonymous functions can't be keyed by symbol** — Express handlers are often anonymous arrow functions. Keying by `(workspace_hash, source_node = file::symbol)` fails for these. **Solution: key by `file::startLine-endLine`.**
3. **Readable condition labels aren't achievable statically** — The mock showed "valid game id?" but static extraction gives raw predicates like `!param || (param != 730 && param != 570 && …)`. **Solution: accept raw predicates as default; optional LLM pass for labels in Phase 2.**
4. **Single-function scope produces trivial charts for clean code** — Well-architected controllers delegate to services. The real logic lives cross-function. **Solution: honest documentation — Phase 1 is thin-handler only; cross-function is Phase 3.**
5. **`format:"flowchart"` creates response-shape collision** — Adding a new format to the existing endpoint produces two entirely different response shapes behind one method+path. **Solution: new endpoint `POST /api/v1/graph/flowchart`.**
6. **Layout is the dominant risk** — Spine+gutter handles guard-clause handlers; complex cases need full layered layout (Phase 2).

## What Changes

### Phase 1a (2-3 days): Enrich Existing Conditional Labels

A quick win that ships immediately useful enrichment with zero new infrastructure:

- **Extend** `FlowEdge.Conditional bool` → `FlowEdge.ConditionLabel string` on existing graph edges
- **Store** condition predicates in existing graph edge metadata (`Metadata["condition_label"]`)
- **Surface** condition labels in the `edges` JSON response from the existing flow endpoint
- **Update** mermaid renderer to label conditional edges with their predicates
- **Dashboard** shows condition labels on dotted edges in the existing Flow view

### Phase 1b (2 weeks): JS/TS CFG Extraction + Dashboard

Full control-flow graph extraction for JS/TS HTTP handlers (zengamingx workspace):

- **New** `internal/graph/cflow.go` — CFG types + `ControlFlowExtractor` interface
- **New** `internal/graph/js_cflow.go` — JS/TS CFG extractor (supports Express, Koa, Fastify handlers)
- **New** `function_flowcharts` table keyed by `(workspace_hash, source_file, start_line, end_line)`
- **Watcher integration** — `extractAndUpsertCFGs` called from `processFile`
- **New endpoint** — `POST /api/v1/graph/flowchart` returning `{found, entry, cfg}`
- **New MCP tool** — `memory_flowchart` (separate from `memory_flow` to avoid contract breakage)
- **Dashboard** — `Flowchart.tsx` component with spine+gutter layout, toggle in Flow panel

### Phase 2 (future): Go + Layout + Condition Condenser

- Go CFG extractor (dogfoodable against nano-brain)
- Full layered layout (dagre/ELK)
- Loop/try-catch/ternary expansion
- Condition label condenser (optional LLM pass for readable labels)

## Capabilities

### New Capabilities

- `conditional-label-enrichment`: Extends existing `conditional` flag on graph edges to carry predicate text. Surface in flow API response and mermaid rendering.
- `control-flow-extraction`: AST-based intra-procedural CFG extraction for JS/TS functions, producing decision/step/terminal nodes and labeled branch edges. Stored per function location and refreshed by the watcher.
- `flowchart-api`: New `POST /api/v1/graph/flowchart` endpoint returns the CFG spec for an entry's handler.
- `flowchart-mcp-tool`: New `memory_flowchart` MCP tool for agents to query control-flow graphs.
- `flowchart-dashboard`: New `Flowchart.tsx` component with spine+gutter layout for guard-clause handlers.

### Modified Capabilities

- `graph-overview-endpoint` / flow API: Existing edges now carry `condition_label` field (additive, non-breaking).

## Impact

- **Indexing**: Phase 1a has zero indexing impact (enriches existing metadata). Phase 1b adds CFG extraction for JS/TS functions during `processFile`; bounded by handler-function scope and skipped for minified files.
- **Storage**: Phase 1a uses existing `graph_edges.metadata` JSONB. Phase 1b adds `function_flowcharts` table; CFG JSON is small (tens of nodes typically).
- **API**: Phase 1a adds `condition_label` to existing edge response (additive). Phase 1b adds new endpoint; no change to existing clients.
- **Dashboard**: Phase 1a shows labels on existing dotted edges. Phase 1b adds `Flowchart.tsx` component with spine+gutter layout.
- **Languages**: Phase 1b is JS/TS only; Go returns empty CFG (dashboard falls back to Graph view) until Phase 2.
- **Accuracy ceiling**: static CFG reflects code structure, not runtime (which branch actually executed, loop iteration counts). This is inherent and acceptable — it shows *possible* paths, like any flowchart.

## Constraints

- Must remain `CGO_ENABLED=0` (no C dependencies)
- Single binary distribution (no Python sidecar)
- All new features opt-in via config (zero breaking changes)
- CFG extraction reuses existing tree-sitter infrastructure (gotreesitter)
- Phase 0 validates gotreesitter `ChildByFieldName` on JS/TS grammar before any implementation
- Max 500 nodes per CFG; truncate with warning for pathological functions

## Risks

| Risk | Mitigation |
|------|-----------|
| gotreesitter walk API may be insufficient for JS/TS | Phase 0: write 15-line test proving `ChildByFieldName` works on JS/TS grammar |
| Anonymous functions can't be keyed by symbol | Key by `file::startLine-endLine` (not symbol) |
| Readable condition labels not achievable statically | Accept raw predicates; optional LLM pass for labels in Phase 2 |
| Single-function scope produces trivial charts for clean code | Honest documentation: Phase 1 is thin-handler only; cross-function is Phase 3 |
| Layout breaks for complex cases | Spine+gutter only; fallback to Graph view for >30 nodes |
| Graph rebuild destroys CFG references | Key by stable identifiers (workspace_hash + source_path + line range), not graph node IDs |
| Intra-function cycles (tail recursion) | CFG adjacency list must be DAG-with-back-edges; detect and terminate on cycles |
| Watcher timing: CFG not indexed when API called | Return `status: "pending"` in API response; dashboard shows loading state |
| `if (err) return;` pattern varies across JS/TS styles | Document supported patterns; Phase 2 handles more idioms |
