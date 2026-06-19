## Why

The sequence diagram currently only renders cross-actor messages (tradeit-backend → MySQL, tradeit-backend → Steam API). All internal logic is collapsed into a single "tradeit-backend" actor, so the diagram shows WHAT external systems are called but not HOW the code flows internally — no conditionals, no loops, no error handling, no function calls within the service.

This makes the diagram useful for infrastructure overview but useless for understanding business logic. A real sequence diagram should show the flow of control, including branching (if/else), iteration (for-each), and error paths (try/catch).

## What Changes

- Render CFG nodes within the main service actor as self-messages (internal calls)
- Show branching as `alt`/`opt` blocks in Mermaid (if/else → alt, try/catch → alt try/success/catch)
- Show loops as `loop` blocks in Mermaid (for-each → loop over items)
- Collapse sequential internal steps into grouped notes (e.g., "validate balance", "check inventory")
- Keep external integrations as cross-actor messages
- Limit total diagram size to prevent overwhelming output (max 50 messages)

## Capabilities

### New Capabilities

- `sequence-diagram-internal-logic`: Sequence diagrams render internal flow logic (conditionals, loops, error handling) within the service actor using Mermaid `alt`/`opt`/`loop` blocks

### Modified Capabilities

_(none — this extends existing sequence diagram rendering)_

## Impact

- **Files**: `internal/flow/sequence.go` (significant rewrite of `RenderSequenceDiagram`)
- **API**: No contract change — `format=sequence` response improves (shows internal logic)
- **Breaking**: No — diagrams get richer, no format change
- **Dependencies**: Requires CFG data (from `function_flowcharts` table) to be populated for the entry function
