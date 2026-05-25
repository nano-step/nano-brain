## ADDED Requirements

### Requirement: Symbol graph view renders at /web/symbols

The system SHALL render an interactive symbol call graph at the `/web/symbols` route using Sigma.js WebGL.

#### Scenario: Route accessible

- **WHEN** user navigates to `/web/symbols`
- **THEN** system displays the symbol graph view with navigation item highlighted

#### Scenario: Graph renders symbols as nodes

- **WHEN** workspace has indexed symbols
- **THEN** each symbol renders as a node with color based on kind (function=blue, class=green, method=cyan, interface=purple)

#### Scenario: Graph renders edges

- **WHEN** symbols have relationships
- **THEN** edges render with color based on type (CALLS=gray, INHERITS=orange, IMPLEMENTS=teal)

#### Scenario: Node size indicates connectivity

- **WHEN** graph renders
- **THEN** node size SHALL be proportional to edge count (hub detection)

### Requirement: Cluster-first rendering for large graphs

The system SHALL use cluster-first rendering when symbol count exceeds 500 to maintain performance.

#### Scenario: Large graph shows clusters

- **WHEN** workspace has more than 500 symbols
- **THEN** graph initially renders Louvain clusters as super-nodes instead of individual symbols

#### Scenario: Cluster super-node size

- **WHEN** clusters render as super-nodes
- **THEN** super-node size SHALL be proportional to memberCount

#### Scenario: Click expands cluster

- **WHEN** user clicks a cluster super-node
- **THEN** super-node expands to show individual symbols within that cluster

#### Scenario: Performance target

- **WHEN** graph renders with 5000+ symbols using cluster-first view
- **THEN** rendering SHALL maintain 30+ FPS

### Requirement: Symbol detail panel

The system SHALL display a detail panel when a symbol node is clicked.

#### Scenario: Click shows detail panel

- **WHEN** user clicks a symbol node
- **THEN** detail panel displays: name, kind, file path, line number

#### Scenario: Detail panel shows relationships

- **WHEN** detail panel is open
- **THEN** panel lists callers and callees of the selected symbol

### Requirement: Empty state handling

The system SHALL display appropriate messaging when no symbols exist.

#### Scenario: No symbols indexed

- **WHEN** workspace has no indexed symbols
- **THEN** view displays empty state message with guidance to index codebase
