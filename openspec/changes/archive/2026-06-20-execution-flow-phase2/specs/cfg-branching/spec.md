## MODIFIED Requirements

### Requirement: Branch-aware CFG edges for JS/TS
The JS/TS CFG extractor SHALL emit typed branch edges (`yes`/`no`/`case:<v>`/`default`/`try`/`catch`) for conditional control flow instead of flat `next` edges. The `CFGEdge.Branch` field already defines the value set; the extractor must populate it correctly.

#### Scenario: if/else produces yes/no edges
- **WHEN** extracting `if (condition) { thenBody(); } else { elseBody(); }`
- **THEN** the CFG contains a `decision` node for the condition
- **AND** an edge labeled `Branch: "yes"` from decision to the first node of the then-block
- **AND** an edge labeled `Branch: "no"` from decision to the first node of the else-block
- **AND** internal then/else block edges remain `Branch: "next"`
- **AND** then/else exits merge to a common successor

#### Scenario: if without else produces yes edge and fallthrough
- **WHEN** extracting `if (condition) { thenBody(); }` (no else)
- **THEN** the CFG contains `Branch: "yes"` from decision to then-block
- **AND** `Branch: "no"` from decision to the merge/next-statement node

#### Scenario: switch/case produces case-labeled edges
- **WHEN** extracting `switch (expr) { case "A": handleA(); break; case "B": handleB(); break; default: handleDefault(); }`
- **THEN** the CFG contains a `decision` node for the switch
- **AND** edges labeled `Branch: "case:A"`, `Branch: "case:B"`, `Branch: "default"` from decision to each case body's first node
- **AND** each case body's internal edges remain `Branch: "next"`
- **AND** all case exits merge to a common successor

#### Scenario: switch without explicit default
- **WHEN** extracting a switch with no `default` case
- **THEN** an implicit `Branch: "default"` edge goes from decision to the merge/next-statement node

#### Scenario: ternary expression produces yes/no edges
- **WHEN** extracting `const x = condition ? a() : b();`
- **THEN** the CFG contains a `decision` node for the condition
- **AND** `Branch: "yes"` from decision to the true-branch call
- **AND** `Branch: "no"` from decision to the false-branch call
- **AND** both merge to successor

#### Scenario: try/catch produces try/catch edges
- **WHEN** extracting `try { riskyOperation(); } catch (e) { handleError(); }`
- **THEN** the CFG contains edges labeled `Branch: "try"` to the try body
- **AND** `Branch: "catch"` to the catch body
- **AND** both merge to successor (or finally block if present)

### Requirement: Tree-sitter node type correctness
The extractor SHALL match the correct tree-sitter node type names emitted by the grammar. Known mismatch: `buildSwitch` currently matches `"case"` and `"default"` but tree-sitter-javascript emits `"switch_case"` and `"switch_default"`.

#### Scenario: switch case node types match grammar
- **WHEN** the extractor processes a `switch_statement` node
- **THEN** child node types are matched against the actual grammar types (`switch_case`, `switch_default`) — NOT the currently hardcoded `"case"` / `"default"`

### Requirement: Existing behavior preserved
The extractor SHALL continue to produce correct CFGs for all currently-supported patterns. Branch edges are additive — existing `Branch: "next"` edges for sequential statements remain unchanged.

#### Scenario: Sequential statements still produce next edges
- **WHEN** extracting `a(); b(); c();`
- **THEN** all edges are `Branch: "next"` (no branch labels added)

#### Scenario: Existing tests pass without regression
- **WHEN** the extractor changes are applied
- **THEN** all existing `js_cflow_test.go` tests continue to pass

### Requirement: Loop edges remain unchanged
Loop back-edges are NOT part of this change. Loops continue to produce a single `step` node with `Branch: "next"` edges.

#### Scenario: For loop produces step node
- **WHEN** extracting `for (let i = 0; i < n; i++) { body(); }`
- **THEN** the CFG contains a `step` node labeled "for loop"
- **AND** all edges are `Branch: "next"`

### Requirement: CFG size limit
The extractor SHALL continue to cap CFGs at 500 nodes. Branch edges increase edge count but do not change the node limit.

#### Scenario: Large function with many branches is truncated
- **WHEN** a function contains many nested if/else blocks producing >500 nodes
- **THEN** the CFG is truncated at 500 nodes
- **AND** `status` is set to `"truncated"`

## Scope

- **File:** `internal/graph/js_cflow.go` — extend `buildIf`, `buildSwitch`, ternary handling, `buildTry`
- **Test:** `internal/graph/js_cflow_test.go` — add/update tests for branch edges
- **No schema change:** `CFGEdge.Branch` already supports all values
- **No API change:** `memory_flowchart` and `POST /api/v1/graph/flowchart` return richer data (additive)

## Test Cases

| Input | Expected branch edges |
|-------|----------------------|
| `if (x) { a(); } else { b(); }` | `decision --yes--> a`, `decision --no--> b` |
| `if (x) { a(); }` | `decision --yes--> a`, `decision --no--> <merge>` |
| `switch (v) { case 1: a(); case 2: b(); }` | `decision --case:1--> a`, `decision --case:2--> b`, `decision --default--> <merge>` |
| `x ? a() : b()` | `decision --yes--> a`, `decision --no--> b` |
| `try { a(); } catch (e) { b(); }` | `tryNode --try--> a`, `tryNode --catch--> b` |
| Nested: `if (x) { if (y) { a(); } else { b(); } }` | Outer `--yes--> inner`, inner `--yes--> a`, inner `--no--> b`, outer `--no--> <merge>` |

## Edge Cases

- Empty then/else blocks: emit `yes`/`no` edge to merge node directly
- Single-statement then/else (no braces): wrap in synthetic block, same behavior
- Switch with only default: `decision --default--> body`
- Ternary without else (invalid syntax, but handle gracefully): `--yes--> a`, `--no--> <merge>`

## Acceptance Criteria

1. `go test -race -short ./internal/graph/...` passes with branch-edge assertions
2. All existing tests continue to pass (no regression)
3. `go build ./...` and `go test -race -short ./...` pass
4. Live verification: re-extract CFGs for express-app workspace, confirm branch edges appear in `memory_flowchart` response
