## MODIFIED Requirements

### Requirement: Flow builder supports Ruby edges
The flow builder SHALL construct call chains from Ruby edges stored in the graph_edges table, enabling visualization of Rails controller → service → model flows.

#### Scenario: Build flow from Rails controller action
- **WHEN** a flow is requested for a Rails controller action (e.g., "POST /users")
- **THEN** the flow builder SHALL follow calls edges from the controller action to downstream services and models

#### Scenario: Handle Ruby-specific node naming
- **WHEN** Ruby edges are loaded into the flow builder
- **THEN** node names SHALL be formatted as "ClassName#method_name" for Ruby methods

#### Scenario: Classify Ruby node roles
- **WHEN** a Ruby node is added to the flow
- **THEN** the role classifier SHALL assign RoleService for nodes containing "service", RoleRepo for nodes containing "repository" or "model", and RoleFunc for others

### Requirement: Flow materializer processes Ruby edges
The flow materializer SHALL process Ruby edges the same way as edges from other languages, creating searchable flow documents for Rails routes.

#### Scenario: Materialize Ruby flow document
- **WHEN** the materializer runs for a workspace with Ruby edges
- **THEN** it SHALL create flow documents in the "flows" collection for each HTTP entry point

#### Scenario: Ruby flow document includes metadata
- **WHEN** a Ruby flow document is created
- **THEN** it SHALL include the controller name, action name, and HTTP method in the document metadata
