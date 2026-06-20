## Context

`RenderSequenceDiagram(f Flow)` currently does DFS traversal of all 182 nodes, treating each function as a Mermaid participant. Result: 181 participants with 432 flat arrows — unreadable.

The `Flow` model already has the data needed for actor grouping:
- `FlowNode.Role` (entry/handler/func/repo/integration/external/middleware)
- `FlowEdge.Kind` (http/calls/integration/middleware/cross_service)
- `FlowEdge.CrossServiceWorkspace` (for cross-service actor naming)

## Goals / Non-Goals

**Goals:**
- Group functions into system-level actors (Client, Backend, External Systems)
- Only show cross-actor messages
- Synthesize return arrows from DFS backtrack
- Render middleware as notes

**Non-Goals:**
- WebSocket/async paths (needs new edge types)
- Activation boxes (complex, deferred)
- Database driver detection as integrations (Phase 2)
- Loop/par blocks
- Schema changes to Flow model

## Decisions

### D1: Actor mapping by role

| Role | Actor | Notes |
|------|-------|-------|
| entry | Client | Already handled |
| handler, func, repo, service | Backend | All internal code collapses |
| middleware | (hidden) | Rendered as Note, not participant |
| integration | Extract system name | From FlowNode.Name |
| external | Extract system name | From FlowNode.Name |

Cross-service: `FlowEdge.Kind == "cross_service"` → actor `"Service:<workspace[:8]>"`

### D2: Return arrows from DFS backtrack

When DFS returns from a deeper node to a shallower one, and the call crossed an actor boundary, emit `-->>`. This works because DFS naturally follows the call stack — backtrack = return.

### D3: Hide internal calls

Only emit arrows when `actorForNode[from] != actorForNode[to]`. Internal calls (handler→service→repo) are invisible — they're implementation details.

## Risks

| Risk | Mitigation |
|------|------------|
| `cross_service` is edge kind, not role | Handle via edge analysis, not node role |
| All 13 tests break | Rewrite alongside renderer |
| Cycles/diamonds | `seen` set prevents infinite recursion |
