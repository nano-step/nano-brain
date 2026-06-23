# Design — Static control-flow flowcharts

## Goal

Derive a readable **flowchart of a handler's logic** (decisions, branches, outcomes) from the source AST, deterministically and without the LLM. **Phase 1: JS/TS HTTP handlers (express-app workspace), intra-procedural.** Go in Phase 2.

## Deep-Design Conflict Resolution

| Topic | Metis | Oracle | Resolution | Confidence |
|-------|-------|--------|------------|------------|
| Language choice | Go-first for dogfooding | JS/TS-first (users need it) | **JS/TS first** — express-app workspace is the main project that must be supported first | HIGH |
| Storage model | Enrich existing conditional pipeline | Separate table is correct | **Both** — Phase 1a enriches existing; Phase 1b adds dedicated CFG | MEDIUM |
| API design | New endpoint OR enrich existing format | Keep format polymorphism | **New endpoint** `POST /api/v1/graph/flowchart` — avoids response-shape collision | HIGH |
| MCP tool | New tool over modifying memory_flow | Same | **New tool `memory_flowchart`** — MCP contracts are stricter | HIGH |
| Function keying | Key by symbol | Key by symbol | **Key by location** `file::startLine-endLine` — anonymous functions have no symbol | HIGH |
| Condition labels | Accept raw predicates | Accept raw predicates | **Raw predicates** — no NLP/LLM in Phase 1 | HIGH |
| Layout | Spine+gutter is insufficient | Spine+gutter is sufficient for Phase 1 | **Spine+gutter only** — fallback to Graph for complex cases | HIGH |
| Dashboard | Separate repo, separate timeline | Same | **Phase 1b** — API + dashboard ship together for express-app | HIGH |

## 1. Two-Phase Delivery

### Phase 1a: Enrich Existing Conditional Labels (2-3 days)

Zero new infrastructure. Enriches the existing `conditional` boolean on graph edges with predicate text.

**What changes:**
- `FlowEdge.Conditional bool` → `FlowEdge.ConditionLabel string`
- Store predicates in existing `graph_edges.metadata` JSONB (`Metadata["condition_label"]`)
- Surface condition labels in the `edges` JSON response from `POST /api/v1/graph/flow`
- Update mermaid renderer to label conditional edges with their predicates

**Why this ships first:**
- Immediately useful enrichment
- Dogfoodable against nano-brain's own Go endpoints
- Zero new storage, zero new API surface
- Validates the condition-label concept before investing in full CFG extraction

### Phase 1b: Dedicated CFG Extraction (2 weeks)

Full control-flow graph extraction for JS/TS HTTP handlers (express-app workspace).

**New components:**
- `internal/graph/cflow.go` — CFG types + `ControlFlowExtractor` interface
- `internal/graph/js_cflow.go` — JS/TS CFG extractor
- `function_flowcharts` table + sqlc queries
- Watcher integration: `extractAndUpsertCFGs`
- New endpoint: `POST /api/v1/graph/flowchart`
- New MCP tool: `memory_flowchart`
- Dashboard: `Flowchart.tsx` component with spine+gutter layout

## 2. CFG Data Model

The unit is a **control-flow graph** for one function. Shared shape across backend (Go) and dashboard (TS):

```jsonc
{
  "entry": "POST /purchase",           // route the handler serves (when known)
  "source_file": "…/handlers/purchase.go",
  "start_line": 15,
  "end_line": 42,
  "nodes": [
    { "id": "n0", "type": "start",    "label": "POST /purchase", "line": 15 },
    { "id": "n1", "type": "step",     "label": "param := c.Param(\"gameid\")", "line": 16 },
    { "id": "n2", "type": "decision", "label": "param == \"\" || param != \"730\"", "line": 18 },
    { "id": "n3", "type": "terminal", "label": "400 Bad request", "line": 20, "kind": "error" },
    { "id": "n9", "type": "terminal", "label": "200 · inventory", "line": 35, "kind": "return" }
  ],
  "edges": [
    { "from": "n2", "to": "n3", "branch": "yes" },
    { "from": "n2", "to": "n4", "branch": "no" }
  ]
}
```

- **Node types:** `start` (function/route entry), `step` (linear statement or call), `decision` (`if`/`switch`/ternary/loop test), `terminal` (`return`/`throw`/early response), `merge` (join point after a branch).
- **Edge `branch`:** `yes` | `no` | `case:<value>` | `default` | `loop` | `next` (unconditional).
- Calls to other symbols are `step` nodes carrying `call: "<symbol>"` so they can later link to a sub-flow (Phase 3).

**Keying:** By location `(workspace_hash, source_file, start_line, end_line)` — NOT by symbol. Anonymous functions have no symbol name.

Go types in `internal/graph/cflow.go`:

```go
type CFGNode struct {
    ID   string `json:"id"`
    Type string `json:"type"`   // "start" | "step" | "decision" | "terminal" | "merge"
    Label string `json:"label"`
    Line int    `json:"line"`
    Kind string `json:"kind,omitempty"`   // "error" | "return" (terminals only)
    Call string `json:"call,omitempty"`   // target symbol (steps that are calls)
}

type CFGEdge struct {
    From   string `json:"from"`
    To     string `json:"to"`
    Branch string `json:"branch"`  // "yes" | "no" | "case:<v>" | "default" | "loop" | "next"
}

type CFG struct {
    Entry      string    `json:"entry,omitempty"`
    SourceFile string    `json:"source_file"`
    StartLine  int       `json:"start_line"`
    EndLine    int       `json:"end_line"`
    Nodes      []CFGNode `json:"nodes"`
    Edges      []CFGEdge `json:"edges"`
    Status     string    `json:"status"` // "complete" | "truncated" | "parse_error" | "unsupported"
}
```

## 3. Extraction Algorithm

A `ControlFlowExtractor` interface, consumer-side, mirroring the existing extractor registry:

```go
type ControlFlowExtractor interface {
    SupportsCFG(ext string) bool
    ExtractCFGs(filePath string, content []byte) ([]CFG, error)
}
```

Note: `ExtractCFGs` takes the full file and returns ALL functions' CFGs in one parse. This avoids re-parsing the file per function and keeps the extractor stateless.

Phase 1 implements `JSControlFlowExtractor` with tree-sitter (reusing the JS/TS grammar already vendored). Algorithm — a structured recursive walk of the function body that threads a "current predecessor" set:

```
build(block, preds):           # returns the set of open exits after the block
  cur = preds
  for stmt in block.statements:
    switch stmt.type:
      if_statement:
        d = decisionNode(text(stmt.condition))
        link(cur -> d, "next")
        thenExits = build(stmt.consequent, {d:"yes"})
        elseExits = stmt.alternative ? build(stmt.alternative, {d:"no"}) : {d:"no"}
        cur = thenExits ∪ elseExits           # implicit merge
      switch_statement:
        d = decisionNode(text(stmt.discriminant))
        cur = ∪ build(case.body, {d:"case:"+label}) for each case  (+ default)
      return_statement / throw / early-exit:
        t = terminalNode(describe(stmt)); link(cur -> t); cur = {}   # path ends
      call/expression/assignment:
        s = stepNode(text(stmt))              # collapse trivial; keep calls + key ops
        link(cur -> s, "next"); cur = {s}
  return cur
```

- **Terminals:** `return`, `throw`, and early-exit idiom (`if (err) return;` in JS, `res.status(N).send(...)` in Express) collapse to one terminal labeled with the status/body.
- **Condition labels:** take the predicate source, normalize whitespace, cap at 80 chars. Raw text — no NLP condensation in Phase 1. Raw predicate kept in `label`; Phase 2 adds optional LLM condensation.
- **Step collapsing:** consecutive trivial statements (assignments, header sets, logging) collapse into a single step or are dropped, so the chart shows decisions and meaningful calls, not every line.
- Phase 1 handles `if/else`, `switch`, `for/while/do`, sequential statements, returns/throws/early-exit. **The implemented `js_cflow.go` builds loops (`buildLoop`, back-edges tagged `loop`) and `try/catch` (`buildTry`) as real nodes** rather than single annotated steps; ternaries remain a Phase 2 expansion.
- **Cycle detection:** The CFG adjacency list must be a DAG-with-back-edges. Back-edges (tail recursion, mutual recursion) are tagged as `loop`. The extractor detects and terminates on cycles per function.

## 4. Storage

### Phase 1a: Existing Graph Edge Metadata

No new tables. Condition labels stored in existing `graph_edges.metadata` JSONB:

```json
{
  "language": "typescript",
  "line": 18,
  "condition_label": "param === '' || param !== '730'"
}
```

### Phase 1b: New `function_flowcharts` Table

```sql
CREATE TABLE function_flowcharts (
  workspace_hash TEXT NOT NULL,
  source_file    TEXT NOT NULL,
  start_line     INT NOT NULL,
  end_line       INT NOT NULL,
  cfg            JSONB NOT NULL,
  status         TEXT NOT NULL DEFAULT 'complete',  -- complete|truncated|parse_error|unsupported
  created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  CONSTRAINT uq_fc UNIQUE (workspace_hash, source_file, start_line, end_line)
);
CREATE INDEX idx_fc_ws ON function_flowcharts(workspace_hash);
CREATE INDEX idx_fc_file ON function_flowcharts(workspace_hash, source_file);
```

sqlc queries: `UpsertFunctionFlowchart`, `GetFunctionFlowchart`, `DeleteFunctionFlowchartsByFile`. The watcher calls `DeleteFunctionFlowchartsByFile` then upserts during `processFile` (same lifecycle as graph edges), skipping minified files via the existing `isMinified` guard.

**Max size:** 500 nodes per CFG. If exceeded, truncate and set `status: "truncated"`.

## 5. API

### Phase 1a: Enrich Existing Flow Endpoint

`POST /api/v1/graph/flow` with `format:"json"` now includes `condition_label` on edges:

```jsonc
{
  "edges": [
    { "from": "n2", "to": "n3", "kind": "calls", "conditional": true, "condition_label": "param == \"\"" }
  ]
}
```

Mermaid renderer labels conditional edges with their predicates.

### Phase 1b: New Flowchart Endpoint

`POST /api/v1/graph/flowchart` — separate endpoint, separate contract:

```jsonc
// Request
{ "entry": "POST /express-app/api/game" }

// Response (CFG found)
{
  "found": true,
  "entry": "POST /express-app/api/game",
  "method": "POST",
  "path": "/express-app/api/game",
  "cfg": {
    "source_file": "routes/game.ts",
    "start_line": 15,
    "end_line": 42,
    "nodes": [...],
    "edges": [...],
    "status": "complete"
  }
}

// Response (no CFG)
{
  "found": true,
  "entry": "POST /express-app/api/game",
  "cfg": null
}

// Response (CFG too complex)
{
  "found": true,
  "entry": "POST /express-app/api/game",
  "cfg": null,
  "status": "too_complex"
}
```

**Resolution flow:**
1. Resolve entry → handler via existing HTTP edges (same as current flow endpoint)
2. Look up handler's source location from the edge metadata
3. Query `function_flowcharts` by `(workspace_hash, source_file, start_line, end_line)`
4. Return CFG or null

## 6. MCP Tool

### Phase 1a: No MCP changes (condition labels surface via existing `memory_flow` JSON response)

### Phase 1b: New `memory_flowchart` Tool

```jsonc
// Input
{
  "workspace": "<hash>",
  "node": "routes/game.ts::15-42"   // file::startLine-endLine
}

// Output
{
  "found": true,
  "cfg": { "source_file": "...", "start_line": 15, "end_line": 42, "nodes": [...], "edges": [...] }
}
```

Separate tool to avoid breaking `memory_flow` contract. MCP clients are sensitive to response-shape changes.

## 7. Dashboard

### Phase 1a: Label Display

Show condition labels on existing dotted edges in the Flow view. Minimal UI change.

### Phase 1b: Flowchart Component

New `Flowchart.tsx` with spine+gutter layout (guard-clause handlers). Fallback to Graph view for >30 nodes. Full layered layout in later phases.

- **Spine+gutter layout**: Happy path as vertical spine, error terminals in right-hand gutter
- **Node rendering**: Decision diamonds, step boxes, terminal pills
- **Edge labels**: `yes`/`no`/`case` labels on edges
- **Fallback**: Graph view when `cfg: null` or >30 nodes
- **Design tokens**: Use existing tokens, no new dependencies

## 8. Phasing Summary

| Phase | Duration | Scope | Dogfoodable? |
|-------|----------|-------|--------------|
| **Phase 0** | 1 day | Validate gotreesitter `ChildByFieldName` on JS/TS grammar | Yes |
| **Phase 1a** | 2-3 days | Enrich existing `conditional` → `ConditionLabel`; surface in JSON + mermaid | Yes |
| **Phase 1b** | 2 weeks | JS/TS CFG extractor + `function_flowcharts` table + new endpoint + new MCP tool + dashboard | Yes (express-app) |
| **Phase 2** | Future | Go extractor, full layout, loops/try/catch, condition condenser | Yes (nano-brain) |

## 9. Decisions & Risks

- **Static over LLM** — deterministic, exact, no token cost; the trade-off is no natural-language paraphrase of conditions (we show the raw predicate text). Accepted per product direction.
- **JS/TS first for express-app** — express-app workspace is the main project that must be supported first. Go comes later in Phase 2.
- **Location-based keying** — anonymous functions have no symbol; `file::startLine-endLine` is universal.
- **New endpoint over format polymorphism** — avoids response-shape collision; clean separation of concerns.
- **New MCP tool over modifying memory_flow** — MCP contracts are stricter; breaking changes cascade to all clients.
- **Layout is the dominant risk**, not extraction. Mitigation: start with the constrained spine+gutter layout that fits guard-clause handlers (the majority), fall back to Graph view otherwise.
- **Condition-label quality**: raw predicates can be long/ugly; Phase 2 adds optional LLM condensation. Phase 1 accepts raw text.
- **Accuracy ceiling**: shows possible paths, not the executed one (no runtime trace). Inherent to static flowcharts; documented in the UI.
- **JS/TS pattern variation**: `if (err) return;` pattern varies across JS/TS styles. Document supported patterns; Phase 2 handles more idioms.
