# Specs: memory_impact direction parameter

**Tracking:** #419

## ADDED Requirements

### Requirement: memory_impact direction parameter

#### Scenario: Default behavior (backward compatible)

**Given:** `memory_impact(workspace="abc", node="server/main.go")`
**When:** `direction` parameter not specified
**Then:** Returns nodes that import/call `server/main.go` (inbound only, default)
**Then:** Return schema unchanged from current behavior

#### Scenario: Outbound traversal

**Given:** `memory_impact(workspace="abc", node="server/newService.js", direction="out")`
**When:** New file added to repository
**Then:** Returns nodes that `newService.js` imports/calls (outbound)
**Then:** Used for reviewing new files - agent sees its dependencies

#### Scenario: Both directions

**Given:** `memory_impact(workspace="abc", node="server/db.go", direction="both")`
**When:** Full graph exploration needed
**Then:** Returns both inbound and outbound neighbors
**Then:** Results are deduplicated

#### Scenario: No edges in one direction

**Given:** `memory_impact(workspace="abc", node="server/utils.go", direction="out")`
**When:** `utils.go` has no imports/calls
**Then:** Returns empty array `[]`

#### Scenario: Circular dependencies

**Given:** `memory_impact(workspace="abc", node="A", max_depth=2, direction="out")`
**When:** `A` → `B` → `C` → `A` (cycle exists)
**Then:** Returns `["B", "C"]` (stops at max_depth, doesn't revisit)
**Then:** Circular references handled by existing depth logic

## MODIFIED Requirements

### Requirement: memory_impact parameter signature

**Given:** Current `memory_impact(workspace, node, max_depth)`
**Then:** Add optional `direction` parameter after `max_depth`
**Then:** Default value is `"in"` for backward compatibility

### Requirement: memory_graph internal call

**Given:** `memory_graph` is called by `memory_impact`
**Then:** Add `direction` parameter to `memory_graph` call
**Then:** Pass through `direction` value from `memory_impact` request
