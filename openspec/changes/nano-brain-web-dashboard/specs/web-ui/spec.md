# Spec: Web UI Views

## ADDED Requirements

### Requirement: Status dashboard view
The system SHALL provide a status dashboard at /web/ showing system health and learning metrics.

#### Scenario: View status dashboard
- **WHEN** user navigates to /web/
- **THEN** system displays version, uptime, document count, embedding status

#### Scenario: View learning metrics
- **WHEN** user views status dashboard
- **THEN** system displays bandit variant stats, preference weights chart, expand rate trend

#### Scenario: Select workspace
- **WHEN** user clicks workspace selector
- **THEN** system shows list of available workspaces and filters data to selected workspace

### Requirement: Knowledge graph explorer view
The system SHALL provide a knowledge graph explorer at /web/graph using Sigma.js WebGL.

#### Scenario: View knowledge graph
- **WHEN** user navigates to /web/graph
- **THEN** system renders entity knowledge graph with nodes colored by type (tool=blue, service=green, concept=purple)

#### Scenario: Interact with graph node
- **WHEN** user clicks a node in the graph
- **THEN** system highlights connected nodes and shows detail panel with entity information

#### Scenario: View large graph
- **WHEN** graph has more than 500 nodes
- **THEN** system shows cluster-first view with expand-on-click behavior

#### Scenario: Hover over edge
- **WHEN** user hovers over an edge
- **THEN** system shows edge label with relationship type

### Requirement: Code dependency graph view
The system SHALL provide a code dependency graph at /web/code.

#### Scenario: View code dependencies
- **WHEN** user navigates to /web/code
- **THEN** system renders file dependency graph with nodes sized by centrality score and colored by Louvain cluster

#### Scenario: Inspect file node
- **WHEN** user clicks a file node
- **THEN** system shows imports and dependents list for that file

### Requirement: Search interface view
The system SHALL provide a search interface at /web/search.

#### Scenario: Execute search
- **WHEN** user types a query in the search input
- **THEN** system executes hybrid search and displays results with scores, snippets, and tags

#### Scenario: View search result
- **WHEN** user clicks a search result
- **THEN** system shows full document content
