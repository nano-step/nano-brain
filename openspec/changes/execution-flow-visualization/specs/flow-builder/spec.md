## ADDED Requirements

### Requirement: Build an in-process flow with bare-name reconciliation
The system SHALL build a flow tree from an HTTP entry node by traversing graph edges in the order `http` → `middleware` → `calls`, producing plain-data nodes and edges with no database or rendering dependency. Because `calls` edges target **bare callee names** while `calls` sources are `<file>::<func>`, the builder SHALL continue multi-hop traversal by **reconciling** a bare target name to source nodes whose symbol part equals that name (`source_node` symbol-part match), rather than by exact node-id match.

#### Scenario: Multi-hop chain via reconciliation
- **WHEN** the graph has `"POST /api/topup"` →(http) `HandleTopup`, and `handlers/x.go::HandleTopup` →(calls) `Create`, and `service/s.go::Create` →(calls) `Save`
- **THEN** the built flow connects `"POST /api/topup"` → `HandleTopup` → `Create` → `Save` by reconciling each bare target to its defining source node before continuing

#### Scenario: Exact-match traversal would dead-end
- **WHEN** traversal relies only on exact `source_node = <bare target>` matching (as `memory_trace` does)
- **THEN** the chain stops after one hop; the builder MUST therefore use symbol-part reconciliation to reach service/repo depth

#### Scenario: Ambiguous reconciliation
- **WHEN** a bare target name (e.g. `Save`) is defined in more than one file
- **THEN** the builder includes each distinct candidate, de-duplicates by node, caps fan-out per node, and marks the join as ambiguous in flow metadata

#### Scenario: External leaf
- **WHEN** a bare target name reconciles to no source node (e.g. stdlib `Fprintf`)
- **THEN** that node is a terminal node classified `external`

#### Scenario: Middleware attached to entry
- **WHEN** a `middleware` edge `AuthMW` → `HandleTopup` exists
- **THEN** `AuthMW` appears in the flow as a guard on the handler and does not consume traversal depth

### Requirement: Depth cap and cycle safety
The flow builder SHALL accept a maximum depth and SHALL terminate on cyclic graphs without revisiting nodes.

#### Scenario: Depth cap honored
- **WHEN** a flow is built with `maxDepth = 2`
- **THEN** no `calls` node deeper than 2 hops from the handler appears in the flow

#### Scenario: Recursive / mutual calls
- **WHEN** the call graph contains a cycle reachable from the entry
- **THEN** the builder terminates and each node appears at most once

### Requirement: Role classification of nodes
The flow builder SHALL classify each node with a role (`entry`, `middleware`, `handler`, `service`, `repo`, `external`) using isolated heuristics, where classification is advisory and never required for correctness of the traversal.

#### Scenario: Handler and external leaf
- **WHEN** the direct `http` target has downstream `calls` and a leaf node has no in-workspace outgoing edges
- **THEN** the `http` target is classified `handler` and the leaf is classified `external`

#### Scenario: Unresolved downstream
- **WHEN** the handler node could not be resolved to a `calls` chain (unresolved handler)
- **THEN** the flow still returns with the handler as a terminal node, and the result is marked degraded rather than failing
