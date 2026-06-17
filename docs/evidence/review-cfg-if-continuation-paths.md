# Self-Review: cfg-if-continuation-paths

## Story: CFG extractor drops continuation paths for if-without-else (Issue #445)

### Review Verdict: PASS

### What changed
- `internal/graph/js_cflow.go`: Fixed `buildIf` to only call `relabelPreds` when the block produced real exits beyond the decision placeholder
- `internal/graph/js_cflow_test.go`: Added 4 new tests (guard clause continuation, nested guards, if-continues, empty else block)

### Verification
- `go build ./...` ✅
- `go test -race -short ./internal/graph/...` ✅ (24 tests)
- Dogfood: `setUserEmail` in zengamingx now shows full flow (21 nodes) including DB update, email check, commit
- Branch edges: yes/no for if-else, try/catch for try blocks — all working

### Pre-existing failures (NOT from this change)
- `TestExpressExtractor_Integration`: expects 2 middleware edges, gets 1 — pre-existing
- `golangci-lint`: unused func `addMergeToStep`, ineffassign in `buildSwitch` — pre-existing
