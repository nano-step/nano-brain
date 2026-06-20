# Design: Execution Flow Visualization — Phase 2

## Context

Phase 1 delivered:
- **Go/Echo flow extraction** — Echo routes → `http`/`middleware` edges → flow builder → Mermaid flowchart + searchable docs. Extended with Gin, net/http, and framework-agnostic route detection.
- **JS/TS CFG extraction** — per-function control-flow graphs stored in `function_flowcharts`, served by `memory_flowchart` API + MCP tool.
- **Integration-point detection** — Go, JS/TS, Python detect Publish/Consume/Emit/Listen/HTTP-client calls → `integration` edges in `graph_edges`.
- **Sequence diagram renderer** — `RenderSequenceDiagram(Flow)` in `sequence.go`.
- **Cross-workspace stitching** — `Stitch()` in `stitch.go` matches publish/consume across workspaces by topic. Request-driven via `stitch_workspaces` field. Wired into REST and MCP.

Phase 2 addresses the remaining gap: CFG branch edges.

## Goals / Non-Goals

**Goals:**
1. **CFG branch-aware edges** — if/else, switch/case, ternary, and try/catch produce typed branch edges (`yes`/`no`/`case:X`/`default`/`try`/`catch`) instead of flat `next` edges. This makes the CFG structurally accurate and enables branch-specific querying.
2. **Fix switch node type bug** — `buildSwitch` matches `"case"`/`"default"` but tree-sitter-javascript emits `switch_case`/`switch_default`. Fix before shipping.

**Non-Goals:**
- Cross-workspace stitching (already implemented in `stitch.go`)
- Non-Go/JS/TS language CFG extractors (Python, Ruby, etc.) — Phase 3.
- Runtime/distributed tracing (OpenTelemetry) — Phase 3.
- LLM/semantic flow summaries — Phase 3.
- Sequence diagram enhancements (activation boxes, loops, alternatives) — follow-up.
- Loop back-edges — deferred to a future phase.

## Decisions

### D1: CFG branch edges — extend `buildIf` and `buildSwitch`

**Current state:** `buildIf` creates a decision node and chains then/else blocks with `"next"` edges. `buildSwitch` creates a decision node but: (a) matches wrong node types (`"case"`/`"default"` vs actual `switch_case`/`switch_default`), and (b) doesn't create per-case edges at all (the `caseExits` map is populated but the edges from decision to case bodies are missing).

**Change:**
- `buildIf`: Add explicit `Branch: "yes"` edge from decision node to the first node of the then-block, and `Branch: "no"` edge to the first node of the else-block (or to the merge node if no else).
- `buildSwitch`: Fix node type matching to `switch_case`/`switch_default`. Add `Branch: "case:<value>"` edge from decision node to each case body's first node, and `Branch: "default"` for the default case.
- Ternary expressions: Add `Branch: "yes"` / `Branch: "no"` edges (currently ternary is a single `step` node).
- try/catch: Add `Branch: "try"` / `Branch: "catch"` edges for the try body vs catch body.

**Rationale:** The `CFGEdge.Branch` field already defines these values (`"yes" | "no" | "case:<v>" | "default" | "loop" | "next"`). The extractor just doesn't use them yet. This is a targeted fix, not a type change.

**Alternative considered:** Add a `ConditionLabel` field to `CFGEdge` (like `FlowEdge`). Rejected — `Branch` already encodes this semantically; adding a duplicate field creates confusion.

### D2: Switch node type verification

The `buildSwitch` method at `js_cflow.go:415-434` matches `child.Type(b.lang)` against `"case"` and `"default"`. The tree-sitter-javascript grammar emits `switch_case` and `switch_default` as the actual node type names. This means switch statements currently produce only the decision node with no case branches.

**Action:** Verify the actual grammar node types by inspecting a parsed AST, then update the match statements. This is a prerequisite for case-labeled edges to work.

## Risks / Trade-offs

- **[Switch case node type mismatch]** → The AGENTS.md notes that tree-sitter-javascript emits `switch_case`/`switch_default` but the extractor matches `"case"`/`"default"`. This is a real bug — switch branching doesn't work today. Fix before shipping.
- **[CFG builder regression]** → Adding branch edges changes the edge count and structure. All existing tests must pass with the new edges. Golden-file tests for Mermaid rendering may need updating.
- **[Edge count increase]** → Branch-aware edges are more numerous than flat `next` edges. For deeply nested conditionals, this could push functions toward the 500-node cap faster. Acceptable trade-off for accuracy.

## Migration & Rollback

- No schema migration needed — `CFGEdge.Branch` already exists in the JSONB column; the extractor just starts populating it with non-`"next"` values.
- No new edge types in `graph_edges` — CFGs are self-contained in `function_flowcharts.cfg`.
- Rollback: revert the code change. Existing CFGs in the DB retain their flat `"next"` edges until re-extracted.
