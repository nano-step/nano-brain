## ADDED Requirements

### Requirement: Condition labels on edges (Phase 1a)
The dashboard Flow view SHALL display condition labels on dotted (conditional) edges. Labels are shown as small text annotations near the edge.

#### Scenario: Conditional edge shows predicate
- **WHEN** a flow edge has `conditional: true` and `condition_label: "err !== null"`
- **THEN** the edge displays the label text near the edge midpoint
- **AND** the edge remains dotted (existing styling)

#### Scenario: Long labels are truncated in UI
- **WHEN** a `condition_label` exceeds 40 characters
- **THEN** the dashboard truncates with `…` and shows full text on hover

### Requirement: Flowchart view in the Flow panel (Phase 1b)
The dashboard SHALL render a CFG as a flowchart and expose it as a third Flow view alongside Graph and Sequence (`Graph · Sequence · Flowchart`). Decision nodes render as diamonds, steps as boxes, terminals as pills, and edges carry their branch label (`yes`/`no`/`case`).

#### Scenario: Switching to the Flowchart view
- **WHEN** a flow is selected and the user clicks the `Flowchart` toggle
- **THEN** the panel calls `POST /api/v1/graph/flowchart` and renders the CFG
- **AND** decision diamonds show the condition and yes/no branches lead to their nodes

#### Scenario: Error terminals are visually distinct
- **WHEN** a `terminal` node has `kind:"error"` (e.g. a 4xx/5xx response)
- **THEN** it is rendered in the error color and placed in the right-hand gutter
- **AND** the success path remains a readable vertical spine

#### Scenario: Fallback when no CFG
- **WHEN** the flowchart API returns `cfg: null`
- **THEN** the Flowchart toggle is disabled or the view falls back to the Graph view with a notice
- **AND** no error is thrown

### Requirement: Dependency-free rendering
The flowchart SHALL be rendered with the existing stack (SVG), without adding a diagramming dependency (e.g. mermaid), and SHALL use the shared design tokens.

#### Scenario: No new chart dependency
- **WHEN** the flowchart renders
- **THEN** it uses inline SVG and CSS tokens
- **AND** no Mermaid/diagram library is imported

### Requirement: Spine+gutter layout (Phase 1b)
The flowchart SHALL use a spine+gutter layout for guard-clause handlers: happy path as a vertical spine, error terminals in a right-hand gutter.

#### Scenario: Guard-clause handler layout
- **WHEN** the CFG has a linear happy path with early-exit error branches
- **THEN** the happy path renders as a vertical spine
- **AND** error terminals render in a right-hand gutter
- **AND** branch labels (`yes`/`no`) are shown on edges

#### Scenario: Complex CFG fallback
- **WHEN** the CFG has >30 nodes or depth >10
- **THEN** the Flowchart view falls back to the Graph view with a "CFG too complex" notice
