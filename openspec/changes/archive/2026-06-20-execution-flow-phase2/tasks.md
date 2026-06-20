# Tasks: Execution Flow Visualization — Phase 2

Ordered for incremental, independently-verifiable delivery. Each numbered group should build (`CGO_ENABLED=0 go build ./...`) and pass `go test -race -short ./...` before moving on.

## 1. CFG branch edges — if/else
- [ ] 1.1 Verify tree-sitter-javascript node type names for `if_statement` fields (`condition`, `consequence`, `alternative`) — confirm they match current `buildIf` implementation.
- [ ] 1.2 In `buildIf`: add `Branch: "yes"` edge from decision node to first node of then-block.
- [ ] 1.3 In `buildIf`: add `Branch: "no"` edge from decision node to first node of else-block (or merge node if no else).
- [ ] 1.4 Add/update tests: if-with-else, if-without-else, if-elseif-else (chained decisions), nested if.
- [ ] 1.5 Verify existing `js_cflow_test.go` tests still pass (no regression).

## 2. CFG branch edges — switch/case
- [ ] 2.1 Verify tree-sitter-javascript node type names for `switch_statement` children — confirm actual grammar types (AGENTS.md warns they may be `switch_case`/`switch_default` instead of `"case"`/`"default"`).
- [ ] 2.2 Fix node type matching in `buildSwitch` (update switch statement at lines 415-434).
- [ ] 2.3 In `buildSwitch`: add `Branch: "case:<value>"` edge from decision node to each case body's first node.
- [ ] 2.4 In `buildSwitch`: add `Branch: "default"` edge for default case (or implicit default if absent).
- [ ] 2.5 Add/update tests: switch with cases, switch with default, switch without default, switch with fallthrough (break missing).

## 3. CFG branch edges — ternary and try/catch
- [ ] 3.1 In ternary handling (lines 614-624): add `Branch: "yes"` / `Branch: "no"` edges from decision to true/false branches.
- [ ] 3.2 In `buildTry` (lines 560-569): add `Branch: "try"` edge to try body, `Branch: "catch"` edge to catch body.
- [ ] 3.3 Add tests: ternary expression, try/catch, try/catch/finally.

## 4. CFG branch edges — verification
- [ ] 4.1 Run `go test -race -short ./internal/graph/...` — all tests pass.
- [ ] 4.2 Run full `go test -race -short ./...` — no regressions.
- [ ] 4.3 Update any Mermaid golden-file tests if branch edges change output.

## 5. Verification & docs
- [ ] 5.1 Full build and test suite green.
- [ ] 5.2 Dogfood: re-extract zengamingx CFGs, verify branch edges appear.
- [ ] 5.3 Update AGENTS.md docs for `internal/graph` (Known limitations section).
