# real-sequence Delta — Real sequence diagram with system-level actors

## ADDED Requirements

### Requirement: System-level actor grouping

The sequence diagram SHALL group functions into system-level actors based on their role. Internal functions (handler, func, repo, service) collapse into a single "Backend" actor. Integration and external functions become separate actors.

#### Scenario: HTTP entry with DB and external API
- **GIVEN** a flow with entry→handler→repo→integration(MySQL) and entry→handler→external(Steam API)
- **WHEN** `RenderSequenceDiagram` is called
- **THEN** the output contains exactly 3 participants: Client, Backend, MySQL, Steam API
- **AND** only cross-actor messages appear (no internal handler→repo arrows)

### Requirement: Return arrows on DFS backtrack

When the DFS traversal returns from a deeper node to a shallower one across an actor boundary, the renderer SHALL emit a return arrow (`-->>`).

#### Scenario: Backend calls MySQL and gets response
- **GIVEN** a flow with Backend→MySQL (integration edge) followed by other Backend calls
- **WHEN** DFS backtracks from MySQL back to Backend
- **THEN** a return arrow `MySQL -->> Backend` is emitted

### Requirement: Middleware as notes

Middleware nodes SHALL NOT appear as separate participants. Instead, they SHALL be rendered as `Note over Backend: guarded by <name>` before the first cross-actor call from the guarded handler.

#### Scenario: Handler with auth middleware
- **GIVEN** a flow with middleware→handler via middleware edge
- **WHEN** `RenderSequenceDiagram` is called
- **THEN** no middleware participant exists
- **AND** a note `Note over Backend: guarded by AuthMiddleware` appears

### Requirement: Cross-service actors

Cross-service edges SHALL create separate actors named `Service:<workspace[:8]>`.

#### Scenario: Cross-service call
- **GIVEN** a flow with `cross_service` edge to workspace `abc1234567890`
- **WHEN** `RenderSequenceDiagram` is called
- **THEN** a participant `Service:abc12345` appears with the cross-service message

### Requirement: Maximum 15 participants

The output SHALL contain no more than 15 participants. If more actors are detected, the least-connected actors are collapsed into "Other".

#### Scenario: Many external systems
- **GIVEN** a flow calling 20 different external APIs
- **WHEN** `RenderSequenceDiagram` is called
- **THEN** output has ≤15 participants
