## ADDED Requirements

### Requirement: God node ranking
The system SHALL identify the top N most-connected "real" entities in the graph, excluding file-level hubs, concept nodes, and built-in noise.

#### Scenario: Rank god nodes
- **WHEN** an agent queries god nodes with `top_n = 10`
- **THEN** the system SHALL return the 10 symbols with highest degree
- **AND** file-level hubs (label matches source filename) SHALL be excluded
- **AND** built-in noise (str, int, Path, etc.) SHALL be excluded
- **AND** concept nodes (empty source_file) SHALL be excluded

#### Scenario: God node includes metadata
- **WHEN** a god node is returned
- **THEN** the response SHALL include `id`, `label`, and `degree`

### Requirement: Surprise scoring
The system SHALL compute a composite surprise score for cross-file edges, combining confidence weight, cross-file-type, cross-repo, cross-community, and peripheral→hub factors.

#### Scenario: Score cross-file edge
- **WHEN** an edge connects symbols from different files
- **THEN** the surprise score SHALL be computed as:
  - Confidence weight: AMBIGUOUS=3, INFERRED=2, EXTRACTED=1
  - Cross file-type bonus: +2 (code↔paper, code↔image)
  - Cross-repo bonus: +2 (different top-level directory)
  - Cross-community bonus: +1 (different Leiden community)
  - Peripheral→hub bonus: +1 (low-degree node reaching high-degree node)

#### Scenario: Suppress false positives
- **WHEN** an INFERRED edge crosses language boundaries or connects code to doc
- **THEN** the structural bonuses SHALL be suppressed (confidence weight only)

### Requirement: Surprising connections
The system SHALL return the top N most surprising cross-file edges, ranked by surprise score.

#### Scenario: Return surprising connections
- **WHEN** an agent queries surprising connections with `top_n = 5`
- **THEN** the system SHALL return the 5 highest-scored cross-file edges
- **AND** each result SHALL include `source`, `target`, `confidence`, `relation`, and `why`

### Requirement: Suggested questions
The system SHALL auto-generate questions from graph structure: AMBIGUOUS edges, bridge nodes, god nodes with INFERRED edges, isolated nodes, and low-cohesion communities.

#### Scenario: Generate questions from AMBIGUOUS edges
- **WHEN** the graph contains AMBIGUOUS edges
- **THEN** the system SHALL generate questions like "What is the exact relationship between X and Y?"

#### Scenario: Generate questions from bridge nodes
- **WHEN** the graph contains nodes with high betweenness centrality
- **THEN** the system SHALL generate questions like "Why does X connect community A to community B?"

#### Scenario: Generate questions from isolated nodes
- **WHEN** the graph contains nodes with degree <= 1
- **THEN** the system SHALL generate questions like "What connects X to the rest of the system?"

### Requirement: MCP tools for graph analysis
The system SHALL expose `memory_communities`, `memory_surprises`, and `memory_god_nodes` via MCP.

#### Scenario: memory_communities returns communities
- **WHEN** an agent calls `memory_communities`
- **THEN** the system SHALL return all communities with their member symbols

#### Scenario: memory_surprises returns surprises
- **WHEN** an agent calls `memory_surprises` with `top_n`
- **THEN** the system SHALL return the top N surprising connections

#### Scenario: memory_god_nodes returns god nodes
- **WHEN** an agent calls `memory_god_nodes` with `top_n`
- **THEN** the system SHALL return the top N god nodes
