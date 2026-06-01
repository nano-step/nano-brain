# graph-overview-endpoint Specification

## Purpose
TBD - created by archiving change add-graph-overview-endpoint. Update Purpose after archive.
## Requirements
### Requirement: Graph overview endpoint
The `POST /api/v1/graph/overview` endpoint SHALL return a default subgraph view for a workspace, consisting of the top-N most-connected nodes and all edges between them.

#### Scenario: Code mode default
- **WHEN** `POST /api/v1/graph/overview` is called with `{workspace, mode: "code", limit: 50}`
- **THEN** the response is HTTP 200 with `{nodes, edges, truncated, frontier_count}`
- **AND** `nodes` is an array of up to 50 items ordered by descending degree
- **AND** the query implicitly uses `edge_types = ["calls", "imports", "contains"]` (overridable via explicit `edge_types`)
- **AND** `edges` contains only edges where both source and target node IDs are in the returned `nodes` set

#### Scenario: Knowledge mode default
- **WHEN** called with `{workspace, mode: "knowledge", limit: 50}`
- **THEN** implicitly uses `edge_types = ["references"]`
- **AND** returns top-N documents by reference degree

#### Scenario: Empty workspace
- **WHEN** the workspace has no edges
- **THEN** response is `{nodes: [], edges: [], truncated: false, frontier_count: 0}`
- **AND** is NOT null

#### Scenario: limit clamping
- **WHEN** `limit` is missing, zero, or negative
- **THEN** server uses default 50
- **WHEN** `limit` exceeds 200
- **THEN** server clamps to 200

#### Scenario: explicit edge_types override
- **WHEN** request includes `edge_types: ["calls"]`
- **THEN** the query uses only that edge type regardless of `mode`

#### Scenario: truncated flag
- **WHEN** more than `limit` distinct nodes exist in the workspace (matching edge_types)
- **THEN** `truncated: true`
- **WHEN** the workspace has fewer nodes than limit
- **THEN** `truncated: false`

### Requirement: GraphPanel auto-fetch
The `/ui/graph` frontend panel SHALL automatically fetch and display the graph overview on mount when no focus node is specified.

#### Scenario: Empty focus shows overview
- **WHEN** user navigates to /ui/graph with empty focus input
- **THEN** the panel calls `/api/v1/graph/overview` for the current mode
- **AND** displays the returned nodes + edges in the Sigma graph canvas
- **AND** the "Enter a symbol name above" empty state is NOT shown when overview returns nodes

#### Scenario: Focus input triggers neighborhood
- **WHEN** user types a non-empty focus value
- **THEN** the existing `/api/v1/graph/neighborhood` call fires for that focus
- **AND** the overview view is replaced with the neighborhood result

#### Scenario: Mode switch refreshes overview
- **WHEN** focus is empty AND user clicks Code or Knowledge tab
- **THEN** a new `/api/v1/graph/overview` call fires for the new mode
- **AND** edge_types update accordingly (calls/imports/contains for Code, references for Knowledge)

### Requirement: Regression test for shape
A handler test SHALL assert the response body has all expected top-level keys with correct nested types. The test SHALL fail if any field is renamed or returned as an unexpected type.

#### Scenario: Shape assertion
- **WHEN** the handler returns its response
- **THEN** the response body must contain keys `nodes`, `edges`, `truncated`, `frontier_count`
- **AND** `nodes` and `edges` MUST be arrays (never null)
- **AND** each node has `id`, `kind`, `label`, `metadata`
- **AND** each edge has `source`, `target`, `edge_type`

