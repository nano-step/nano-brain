## MODIFIED Requirements

### Requirement: Sequence diagrams render Ruby request flows
The sequence diagram renderer SHALL visualize Rails request flows, showing the chain from HTTP request through controller actions to service and model calls.

#### Scenario: Render Rails controller sequence
- **WHEN** a sequence diagram is requested for a Rails controller action
- **THEN** the renderer SHALL show the HTTP request → controller action → service calls → model queries as a sequence of messages

#### Scenario: Show Ruby class names in sequence
- **WHEN** rendering a Ruby sequence diagram
- **THEN** participant names SHALL be formatted as "ClassName" (e.g., "UsersController", "OrderService")

#### Scenario: Show method names in messages
- **WHEN** rendering method calls in a Ruby sequence diagram
- **THEN** message labels SHALL show the method name (e.g., "process(order)", "find(id)")

### Requirement: Sequence diagrams support Ruby control flow
The sequence diagram renderer SHALL visualize Ruby control flow within methods, showing conditional branches and loops.

#### Scenario: Show if/else branches
- **WHEN** a Ruby method contains an if/else block
- **THEN** the sequence diagram SHALL show alt/opt fragments for the conditional branches

#### Scenario: Show loop iterations
- **WHEN** a Ruby method contains a loop
- **THEN** the sequence diagram SHALL show a loop fragment for the repeated operations
