## Why

The current sequence diagram renderer treats every function (182 nodes) as a participant with 432 flat arrows. It's a DFS traversal of the call graph, not a real sequence diagram. Users want system-level actors (Client, Backend, MySQL, Redis) with request/response cycles and return arrows.

## What Changes

- Rewrite `RenderSequenceDiagram` in `internal/flow/sequence.go` to group functions into system-level actors
- Only show cross-actor messages (hide internal handler→service→repo calls)
- Synthesize return arrows (`-->>`) from DFS backtrack
- Render middleware as notes, not separate participants
- Update all tests in `internal/flow/sequence_test.go`

## Impact

- `internal/flow/sequence.go` — full rewrite of `RenderSequenceDiagram`
- `internal/flow/sequence_test.go` — rewrite all tests to match new output
