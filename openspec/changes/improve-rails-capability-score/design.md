## Context

Research for issue #489 identified three high-confidence causes for the poor Rails capability benchmark score:

1. **Traversal mismatch:** `memory_flow` builds an in-memory symbol index and can reconcile bare names to file-qualified nodes. `memory_trace` and `memory_impact` mostly query SQL directly and therefore dead-end on realistic Rails names.
2. **Impact exact matching:** `GetImpactorsByTargets` matches `target_node = ANY(...)` exactly, unlike `GetIncomingEdges`, which already has a `split_part(target_node, '::', 2)` fallback.
3. **Ruby symbol gaps:** the Ruby symbol extractor captures methods/classes/modules but not constant assignments such as `STATUS_ORDER_PAID`, which support/debug tasks expect to find.

The benchmark should remain hard. The goal is to make Rails graph tools handle realistic user input, not to rewrite expectations around current weaknesses.

## Architecture Decisions

### Decision 1: Use SQL fallbacks for direct graph lookups and a Go helper for BFS expansion

Use a two-layer reconciliation approach rather than forcing all behavior into one abstraction:

- **SQL layer:** direct graph queries that already operate on one node or frontier should support the same symbol fallback pattern as `GetIncomingEdges`, including `split_part(..., '::', 2)` where appropriate. This includes both `GetImpactorsByTargets` and `GetImpactors`.
- **Go layer:** BFS tools should use a small shared helper in the graph/server traversal layer that normalizes and expands a requested node into candidate graph node IDs with fanout guards.

The Go helper should support:

- exact node IDs (`app/models/story.rb::Story#create_print_orders`),
- class/method IDs (`Story#create_print_orders`),
- bare class IDs (`DropboxUploadManager`),
- bare method IDs (`create_print_orders`).

The helper should reuse existing graph edges and symbol-part logic rather than inventing Rails-specific state in each handler. It should include fanout guards equivalent to flow builder's `maxReconcileFiles = 8` behavior to prevent generic names like `save` or `where` from exploding traversal.

### Decision 2: Align trace and impact with flow's reconciliation behavior

`memory_trace` and the HTTP trace endpoint already have partial symbol-aware source matching through `GetOutgoingEdgesBySymbol`. Implementation must first diagnose whether the Rails benchmark trace failures are due to missing graph edges, edge type filtering, target naming, or missing multi-hop reconciliation. If graph edges exist, trace should reconcile bare callee targets into matching file-qualified source nodes before later hops.

`memory_impact` and the HTTP impact endpoint should use symbol-aware target expansion before reading incoming edges. The implementation can either add SQL helpers mirroring `GetIncomingEdges` behavior or perform a pre-resolution step that converts bare nodes into exact frontier candidates.

### Decision 3: Keep flow HTTP-first, but support Rails job/service entries

`memory_flow` should continue treating HTTP routes as the primary flow entry type. If no HTTP edge matches and the entry is not an HTTP-looking string, it should fall back to class/job/service entry resolution and start BFS from matching contains/calls/integration nodes.

This makes `DropboxFolderUpdateJob`-style support questions valid without changing the meaning of existing HTTP flows.

### Decision 4: Extract Ruby constants as first-class symbols

Extend `internal/symbol/ruby_extractor.go` so constant assignments produce `KindConst` symbols. This should include status constants in models/concerns/services and should not require Rails-specific logic beyond standard Ruby parsing.

### Decision 5: Benchmark improvements must be measured, not frozen prematurely

The Rails capability benchmark should be run before and after implementation, but `results_current.json` and any private-workspace baseline artifacts must remain uncommitted unless explicitly sanitized and approved. Freezing a baseline should happen only after score improvements are meaningful.

## Phasing

### Phase 1: Traversal reconciliation MVP

- Diagnose actual stored source/target edge names for the benchmark's failing Rails nodes before changing traversal behavior.
- Add SQL fallback consistency for impact queries that lack symbol matching.
- Add symbol-aware node expansion shared by trace and impact where diagnostics confirm traversal names are the blocker.
- Add SQL/query support where needed for source/target symbol matching.
- Add unit tests covering Ruby-style `Class#method` and bare class inputs.
- Target lift: `trace > 0`, `impact > 0`.

### Phase 2: Rails entry and symbol coverage

- Add non-HTTP class/job/service entry fallback for flow.
- Extract Ruby constants as symbols.
- Add focused Ruby symbol tests for `STATUS_*` constants and concern files.
- Target lift: `flow`, `symbol-lookup`, `search-qa`, and `state-transition` categories improve.

### Phase 3: Benchmark and regression hardening

- Run Rails capability benchmark locally against a real runtime workspace supplied through environment variables.
- Record sanitized score-only evidence.
- Add fixture-backed tests that do not depend on private workspace names, hashes, or paths.

## Risks and Mitigations

### Risk: Over-reconciliation creates noisy traversal

Mitigation: cap candidate counts, prefer same-file definitions, and avoid expanding generic names when candidate count is above the guard threshold.

### Risk: SQL changes affect Go/TypeScript graph behavior

Mitigation: keep exact matching first, add fallback only for unresolved/bare names, and run existing graph/trace/impact tests plus quick validation.

### Risk: Benchmark overfitting

Mitigation: do not edit expectations to match current output. Each change must be explainable as a general Rails/Ruby code-intelligence improvement.

### Risk: Private workspace leakage

Mitigation: committed artifacts use placeholders (`rails-app`, generic app paths) and score-only evidence. Do not commit runtime workspace hashes or raw result JSON from private apps.

## Validation Plan

- `go test ./internal/graph ./internal/server/handlers ./internal/mcp` or narrower package tests for changed areas.
- `go build ./... && go test -race -short ./...`.
- Rails capability benchmark with runtime-only `NANO_BRAIN_WORKSPACE`, reporting only category/overall scores.
- Privacy grep before commit/PR for known private names, hashes, and filesystem paths.
