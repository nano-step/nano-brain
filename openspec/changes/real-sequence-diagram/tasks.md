# Tasks: real-sequence-diagram

## 1. Actor grouping

- [ ] 1.1 Implement `groupActors(f Flow) map[string]string` — maps node ID → actor alias based on FlowNode.Role
- [ ] 1.2 Implement `actorName(name string) string` — extracts clean system name from integration/external node names
- [ ] 1.3 Handle `cross_service` edges — assign target nodes to `Service:<workspace[:8]>` actor

## 2. Rewrite RenderSequenceDiagram

- [ ] 2.1 Replace current DFS with actor-aware traversal — only emit arrows when actors differ
- [ ] 2.2 Add return arrow synthesis — emit `-->>` on DFS backtrack across actor boundary
- [ ] 2.3 Add middleware rendering as `Note over Backend: guarded by <name>`
- [ ] 2.4 Cap participants at 15 — collapse least-connected actors into "Other"
- [ ] 2.5 Preserve existing `alt`/`opt` conditional block behavior (adapt to actor-level)

## 3. Tests

- [ ] 3.1 Rewrite all existing tests in `sequence_test.go` to match new output format
- [ ] 3.2 Add test: flow with integration edge → external actor appears
- [ ] 3.3 Add test: return arrows on backtrack
- [ ] 3.4 Add test: internal-only flow → Client→Backend minimal diagram
- [ ] 3.5 Add test: cross-service edge → Service actor appears

## 4. Verify and test

- [ ] 4.1 Build: `go build ./...`
- [ ] 4.2 Test: `go test -race -short ./internal/flow/...`
- [ ] 4.3 Manual: render `POST /purchase` and verify ≤10 participants with return arrows
