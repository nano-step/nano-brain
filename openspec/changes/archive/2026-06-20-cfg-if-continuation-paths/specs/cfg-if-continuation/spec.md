# cfg-if-continuation Delta — Guard clause continuation paths

## ADDED Requirements

### Requirement: No-else if preserves continuation path

When an `if` statement has no `else` block and the then-block ends with a terminal (return/throw), the code after the `if` SHALL be reachable from the decision node.

#### Scenario: Guard clause with early return
- **GIVEN** a function with `if (guard) { return; }` followed by happy-path code
- **WHEN** the CFG is extracted
- **THEN** the happy-path code is connected to the decision node via a `"next"` edge

#### Scenario: Nested guard clauses
- **GIVEN** `if (a) { return; } if (b) { return; } happy_path;`
- **WHEN** the CFG is extracted
- **THEN** both guard decisions connect to the continuation code

#### Scenario: If without else, then-block does NOT return
- **GIVEN** `if (cond) { x = 1; } y = 2;`
- **WHEN** the CFG is extracted
- **THEN** both the then-block exit AND the decision node connect to `y = 2`
