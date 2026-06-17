## ADDED Requirements

### Requirement: Renderer-agnostic GraphCanvas component
The dashboard SHALL provide a `GraphCanvas` React component with props `{ model: GraphModel, layout: 'force' | 'dagre', onNodeClick: (node: GNode) => void }` that renders graphs using the configured chart library (React Flow initially, G6 swappable later).

#### Scenario: GraphCanvas renders a force-directed graph
- **WHEN** `GraphCanvas` receives a `GraphModel` with `layout="force"`
- **THEN** nodes are positioned using a force-directed layout
- **AND** edges connect the correct source/target nodes

#### Scenario: GraphCanvas renders a directed flow graph
- **WHEN** `GraphCanvas` receives a `GraphModel` with `layout="dagre"`
- **THEN** nodes are positioned in a top-down directed layout
- **AND** edges flow from source to target without cycles

#### Scenario: Node click triggers callback
- **WHEN** user clicks a node in `GraphCanvas`
- **THEN** `onNodeClick` is called with the clicked `GNode` object

### Requirement: Graph model types
The dashboard SHALL define a `GraphModel` type: `{ nodes: GNode[], edges: GEdge[] }` where `GNode = { id: string, label: string, role: string, meta?: Record<string, unknown> }` and `GEdge = { id: string, source: string, target: string, kind: string, dashed?: boolean }`.

#### Scenario: Model accepts empty graph
- **WHEN** `GraphModel` has `nodes: []` and `edges: []`
- **THEN** `GraphCanvas` renders an empty canvas without errors

#### Scenario: Model handles duplicate edges
- **WHEN** `GraphModel` contains two edges with the same source and target
- **THEN** both edges are rendered (no deduplication at model level)

### Requirement: Flow adapter
The dashboard SHALL provide a `flowToModel` adapter that converts the API flow response (`nodes[]`/`edges[]` from `/api/v1/graph/flow`) into a `GraphModel`. Edges with `kind: "conditional"` or `kind: "middleware"` SHALL be rendered as dashed lines.

#### Scenario: Flow response converts to GraphModel
- **WHEN** `flowToModel` receives a flow response with 3 nodes and 2 edges
- **THEN** the returned `GraphModel` has 3 `GNode` items and 2 `GEdge` items
- **AND** node roles are preserved in `GNode.role`

#### Scenario: Conditional edges are dashed
- **WHEN** a flow edge has `kind: "conditional"`
- **THEN** the corresponding `GEdge` has `dashed: true`

### Requirement: Graph adapter
The dashboard SHALL provide an `apiGraphToModel` adapter that converts the API graph response (`nodes[]`/`edges[]` from `/api/v1/graph/overview` or `/api/v1/graph/neighborhood`) into a `GraphModel`.

#### Scenario: Graph overview converts to GraphModel
- **WHEN** `apiGraphToModel` receives a graph response with symbol nodes and call edges
- **THEN** the returned `GraphModel` has nodes with `role` derived from `node.kind`
- **AND** edges have `kind` derived from `edge.edge_type`

### Requirement: Graph panel integration
The Graph panel SHALL use `GraphCanvas` with the `apiGraphToModel` adapter, supporting legend display, node coloring by role, neighborhood expansion on click, and position caching across renders.

#### Scenario: Graph panel renders overview
- **WHEN** user navigates to Graph panel
- **THEN** the knowledge graph is rendered using `GraphCanvas`
- **AND** a legend shows node role colors

#### Scenario: Neighborhood expansion
- **WHEN** user clicks a node in the graph
- **THEN** the neighborhood around that node is fetched via `/api/v1/graph/neighborhood`
- **AND** the graph expands to show the neighborhood nodes

### Requirement: Flow panel integration
The Flow panel SHALL use `GraphCanvas` with the `flowToModel` adapter, consuming `nodes[]/edges[]` from the API (not the Mermaid string). A "Copy Mermaid" button SHALL copy the `mermaid` field for pasting into docs.

#### Scenario: Flow panel renders endpoint flow
- **WHEN** user selects an endpoint in Flow panel
- **THEN** the flow is rendered using `GraphCanvas` with dagre layout
- **AND** nodes show roles (handler, middleware, service)
- **AND** edges show kind (calls, conditional, middleware)

#### Scenario: Copy Mermaid button works
- **WHEN** user clicks "Copy Mermaid"
- **THEN** the Mermaid text from the API response is copied to clipboard
