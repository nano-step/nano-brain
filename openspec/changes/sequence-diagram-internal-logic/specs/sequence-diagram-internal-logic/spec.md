## ADDED Requirements

### Requirement: Render internal CFG nodes as self-messages

The sequence diagram SHALL render internal function logic within the service actor as self-messages. When a function's CFG is available, step nodes SHALL be rendered as `tradeit-backend->>tradeit-backend: <step label>`.

#### Scenario: Simple function with steps
- **WHEN** the entry function has CFG nodes `start → validate → process → terminal`
- **THEN** the sequence diagram SHALL show self-messages for `validate` and `process` within the service actor

#### Scenario: No CFG available
- **WHEN** the entry function has no CFG in `function_flowcharts`
- **THEN** the sequence diagram SHALL fall back to current behavior (cross-actor messages only)

### Requirement: Render conditionals as alt/loop blocks

The sequence diagram SHALL render decision nodes as Mermaid `alt`/`opt` blocks. Branch edges with `yes`/`no` labels SHALL map to `alt yes/no` blocks. Single outgoing edges SHALL map to `opt` blocks.

#### Scenario: If/else with yes/no branches
- **WHEN** a decision node has `yes` and `no` outgoing edges
- **THEN** the sequence diagram SHALL render `alt yes\n<truthy path>\nelse\n<falsy path>\nend`

#### Scenario: Loop with loop edge
- **WHEN** a decision node has a `loop` outgoing edge
- **THEN** the sequence diagram SHALL render `loop condition\n<body>\nend`

### Requirement: Diagram size limits

The sequence diagram SHALL NOT exceed 50 total messages or 3 levels of decision nesting. When exceeded, the diagram SHALL truncate with a note: "Internal logic too complex — see full CFG".

#### Scenario: Deeply nested function
- **WHEN** the CFG has more than 3 levels of nested decisions
- **THEN** the sequence diagram SHALL stop rendering internal logic at depth 3 and emit a truncation note

#### Scenario: Exceeds message limit
- **WHEN** total messages (cross-actor + internal) would exceed 50
- **THEN** the sequence diagram SHALL emit the first 45 messages and a truncation note
