## Why

Phase 1 delivered in-process execution flows for Go/Echo endpoints and JS/TS function CFGs. One structural blind spot remains:

**JS/TS CFGs are flat** — the CFG extractor generates only sequential `next` edges for all control flow. Real code has if/else, switch, and try/catch branching, but the extractor collapses these into a linear sequence. An agent asking "what happens when topup fails?" gets the same flow regardless of the branch condition. The `CFGEdge.Branch` field already defines `yes`/`no`/`case:X`/`default` values but the extractor never populates them.

Additionally, `buildSwitch` has a latent bug: it matches tree-sitter node types `"case"` and `"default"` but the grammar emits `"switch_case"` and `"switch_default"`, so switch statements produce no branch edges at all.

The other items originally deferred from Phase 1 (non-Echo Go routes, integration-point detection, sequence diagrams, cross-workspace stitching) are already implemented.

## What Changes

- **CFG branch-aware edges** — the JS/TS CFG extractor emits typed branch edges (`yes`/`no`/`case:X`/`default`/`try`/`catch`) for if/else, switch/case, ternary, and try/catch instead of flat `next` edges. No schema change — `CFGEdge.Branch` already supports these values.
- **Fix switch node type mismatch** — `buildSwitch` updated to match `switch_case`/`switch_default` (actual grammar types) instead of `"case"`/`"default"`.

## Capabilities

### Modified Capabilities
- `control-flow-extraction` (Phase 1b baseline): JS/TS CFG extractor now produces branch-aware edges for conditional control flow. Existing sequential behavior preserved for non-conditional statements.

## Impact

- **Code affected**:
  - `internal/graph/js_cflow.go` — extend `buildIf`, `buildSwitch`, ternary, and `buildTry` to emit branch-labeled edges. Fix node type matching in `buildSwitch`.
  - `internal/graph/js_cflow_test.go` — add/update tests for branch edges.
- **Dependencies**: None new.
- **API changes**: `memory_flowchart` and `POST /api/v1/graph/flowchart` return branch-aware CFGs (additive, no breaking change).
- **Risk**: Switch case node type names in tree-sitter-javascript grammar need verification against the actual grammar. Branch edges increase edge count in existing CFGs — no functional impact but larger JSON responses.
