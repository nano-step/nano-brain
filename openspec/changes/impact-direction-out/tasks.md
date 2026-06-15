# Tasks: Add direction="out" to memory_impact

**Tracking:** #419
**Status:** In Progress

## Phase 1: Core Implementation

- [ ] 1.1 Update `memory_graph` to support direction parameter
  - [ ] Add `direction` parameter (default: `"in"`)
  - [ ] Implement `"in"`, `"out"`, `"both"` logic
  - [ ] Handle edge cases (no edges, cycles)

- [ ] 1.2 Update `memory_impact` MCP handler
  - [ ] Add `direction` parameter
  - [ ] Pass to `memory_graph` call
  - [ ] Default to `"in"` for backward compatibility

- [ ] 1.3 Update internal storage queries
  - [ ] Verify graph queries support direction
  - [ ] Add test queries for outbound edges

## Phase 2: Testing

- [ ] 2.1 Add unit tests for `memory_impact`
  - [ ] Test `direction="in"` (backward compatibility)
  - [ ] Test `direction="out"`
  - [ ] Test `direction="both"`
  - [ ] Test no edges case

- [ ] 2.2 Add integration tests
  - [ ] Test end-to-end with real graph data
  - [ ] Test circular dependency handling
  - [ ] Test multi-hop traversal

- [ ] 2.3 Update test matrix
  - [ ] Add `memory_impact` direction scenarios
  - [ ] Document in `docs/TEST_MATRIX.md`

## Phase 3: Validation

- [ ] 3.1 Run `go build ./... && go test -race -short ./...`
- [ ] 3.2 Run integration tests
- [ ] 3.3 MCP tool call test (user-flow)

## Phase 4: Documentation

- [ ] 4.1 Update MCP docs
  - [ ] Document `direction` parameter
  - [ ] Add examples for each direction

- [ ] 4.2 Update README
  - [ ] Add usage example for outbound traversal
  - [ ] Document use case: reviewing new files

## Phase 5: Review & Merge

- [ ] 5.1 Self-review
- [ ] 5.2 PR + Bot Review Loop
- [ ] 5.3 Merge and archive
