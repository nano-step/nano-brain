## ADDED Requirements

### Requirement: Leiden community detection
The system SHALL detect communities in the code graph using the Leiden algorithm via `gonum.org/v1/gonum/graph.community.Leiden`. Each symbol SHALL be assigned a community ID (integer) representing its natural grouping.

#### Scenario: Detect communities from code graph
- **WHEN** the system runs community detection on a code graph
- **THEN** each symbol with at least one edge SHALL have a `community_id` assigned
- **AND** isolated symbols (no edges) SHALL have `community_id = NULL`

#### Scenario: Deterministic results
- **WHEN** community detection runs on the same graph twice
- **THEN** the same community assignments SHALL be produced (using seed=42)

### Requirement: Hub exclusion
The system SHALL exclude high-degree "hub" nodes from community partitioning. Hub nodes SHALL be reattached to their majority-vote neighbor community after partitioning.

#### Scenario: Exclude utility hubs
- **WHEN** a node's degree exceeds the 90th percentile of all node degrees
- **THEN** that node SHALL be excluded from Leiden partitioning
- **AND** the node SHALL be assigned to the community of its most-connected neighbor

#### Scenario: Hub threshold configurable
- **WHEN** the system runs community detection with a custom `exclude_hubs_percentile` parameter
- **THEN** nodes above that percentile threshold SHALL be excluded

### Requirement: Oversized community splitting
The system SHALL split communities larger than 25% of the graph (minimum 10 nodes) by running a second Leiden pass on the subgraph.

#### Scenario: Split oversized community
- **WHEN** a community contains more than 25% of all graph nodes (minimum 10)
- **THEN** the system SHALL run a second Leiden pass on that community's subgraph
- **AND** the resulting sub-communities SHALL replace the original oversized community

### Requirement: Cohesion scoring
The system SHALL compute a cohesion score for each community as the ratio of actual intra-community edges to maximum possible edges.

#### Scenario: Compute cohesion
- **WHEN** the system computes cohesion for a community with N nodes
- **THEN** the cohesion score SHALL be `actual_edges / (N * (N-1) / 2)`
- **AND** communities with cohesion < 0.05 (minimum 50 nodes) SHALL be re-split

### Requirement: Stable community IDs across runs
The system SHALL remap community IDs to maximize overlap with a previous assignment, ensuring stable IDs across incremental updates.

#### Scenario: Stable IDs after graph update
- **WHEN** the graph is updated and community detection re-runs
- **THEN** community IDs SHALL be remapped to maximize overlap with the previous assignment
- **AND** unmatched communities SHALL get fresh IDs in deterministic order (size desc, lexical tie-break)
