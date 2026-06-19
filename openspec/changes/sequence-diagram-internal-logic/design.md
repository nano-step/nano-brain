## Context

The sequence diagram renderer (`internal/flow/sequence.go`) currently:
1. Groups nodes into actors via `groupActors` (Client, serviceName, external systems)
2. Does DFS traversal, emitting only cross-actor messages (line 259-277)
3. All internal function calls within the same actor are silently skipped

The CFG data (from `function_flowcharts`) contains rich control flow information per function: start, step, decision, terminal, merge nodes with branch edges (yes/no/loop/case). This data is already extracted and stored — but unused by the sequence diagram.

The goal is to render internal logic within the service actor using Mermaid sequence diagram features: self-messages for steps, `alt`/`opt` for conditionals, `loop` for iteration.

## Goals / Non-Goals

**Goals:**
- Render internal CFG nodes as self-messages within the service actor
- Show conditionals as `alt`/`opt` blocks (if/else → alt yes/no)
- Show loops as `loop` blocks
- Show error handling (try/catch) as alt blocks
- Limit diagram to ~50 messages max to prevent overwhelming output
- Keep external integrations as cross-actor messages

**Non-Goals:**
- Rendering every single CFG node (500-node CFGs would be unreadable)
- Showing function-level decomposition (each function as separate lifeline)
- Replacing the existing cross-actor message rendering
- CFG-to-UML exact mapping

## Decisions

### Decision 1: Render decision nodes as alt/loop blocks

**Choice**: Map CFG `decision` nodes to Mermaid `alt`/`opt`/`loop` blocks based on edge labels:
- `yes`/`no` edges → `alt` block (if/else)
- `loop` edge → `loop` block
- `case:X` edges → `alt` block with case labels
- Single `next` edge → `opt` block (try/catch or conditional)

**Rationale**: Mermaid's `alt`/`loop` syntax directly maps to if/else and iteration patterns. No custom rendering needed.

### Decision 2: Depth limiting with truncation

**Choice**: Limit internal logic rendering to max 3 decision depth levels and 50 total messages. If exceeded, emit a single note: "Internal logic too complex — see full CFG at /api/v1/graph/flowchart".

**Rationale**: Deeply nested conditionals produce unreadable diagrams. The flowchart endpoint already provides the complete internal view.

### Decision 3: Use existing CFG data, not re-extract

**Choice**: Load CFG from `function_flowcharts` table (already stored) and render it in the sequence diagram context. Do NOT re-parse source code.

**Rationale**: CFG extraction is already done and stored. The sequence diagram just needs to render it differently.

## Risks / Trade-offs

- **[Risk] Large diagrams** → Mitigation: Hard limit of 50 messages + depth 3. Excess truncated with note.
- **[Risk] CFG not available for all functions** → Mitigation: Fall back to current behavior (cross-actor only) when CFG is missing.
- **[Risk] Mermaid rendering issues** → Mitigation: Test with multiple diagram sizes; validate Mermaid syntax.
