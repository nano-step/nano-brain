## ADDED Requirements

### Requirement: Render a flow as a Mermaid flowchart
The system SHALL render a flow tree as a Mermaid `graph TD` flowchart string via a pure function with no side effects.

#### Scenario: Nodes and arrows
- **WHEN** a flow with entry → handler → service → repo is rendered
- **THEN** the output begins with `graph TD` and contains one node per flow node and one arrow per flow edge

#### Scenario: Middleware styling
- **WHEN** a flow contains a middleware guard edge
- **THEN** the middleware arrow is rendered with a distinct (dotted/guard) style from `calls` arrows

### Requirement: Deterministic output
The renderer SHALL produce identical output for identical input by using stable node ids and a deterministic ordering of nodes and edges.

#### Scenario: Stable golden output
- **WHEN** the same flow is rendered twice
- **THEN** the two output strings are byte-for-byte identical

#### Scenario: Sanitized node ids
- **WHEN** a node name contains characters invalid in a Mermaid id (e.g. `/`, spaces, `.`)
- **THEN** the rendered node id is sanitized while the human-readable label preserves the original name
