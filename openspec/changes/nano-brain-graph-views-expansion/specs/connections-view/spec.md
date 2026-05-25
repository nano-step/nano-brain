## ADDED Requirements

### Requirement: Connections view renders at /web/connections

The system SHALL render a document connections graph at the `/web/connections` route using Sigma.js force-directed layout.

#### Scenario: Route accessible

- **WHEN** user navigates to `/web/connections`
- **THEN** system displays the connections view with navigation item highlighted

#### Scenario: Documents render as nodes

- **WHEN** workspace has document connections
- **THEN** each document renders as a node with title as label

### Requirement: Connection edge visualization

The system SHALL render connections as colored edges between document nodes.

#### Scenario: Edge color by relationship type

- **WHEN** connections render
- **THEN** edge color reflects relationship type: supports=green, contradicts=red, extends=blue, supersedes=orange, related=gray, caused_by=yellow, refines=purple, implements=teal

#### Scenario: Edge thickness by strength

- **WHEN** connections render
- **THEN** edge thickness SHALL be proportional to strength value (0.0-1.0 maps to 1-5px)

### Requirement: Connection detail panel

The system SHALL display a detail panel when a document node is clicked.

#### Scenario: Click shows detail panel

- **WHEN** user clicks a document node
- **THEN** detail panel displays: document title, path

#### Scenario: Detail panel shows connections

- **WHEN** detail panel is open
- **THEN** panel lists all connections to/from the selected document with relationship types

### Requirement: Large dataset handling

The system SHALL limit displayed nodes for performance.

#### Scenario: Node limit

- **WHEN** workspace has more than 500 documents with connections
- **THEN** graph displays first 500 nodes with pagination indicator

### Requirement: Empty state handling

The system SHALL display appropriate messaging when no connections exist.

#### Scenario: No connections

- **WHEN** workspace has no document connections
- **THEN** view displays empty state message
