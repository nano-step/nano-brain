# Spec: REST API Endpoints

## ADDED Requirements

### Requirement: System status endpoint
The system SHALL expose GET /api/v1/status returning system health information.

#### Scenario: Request system status
- **WHEN** client sends GET request to /api/v1/status
- **THEN** system returns JSON with version, uptime, document count, embedding count, workspace list, model status

### Requirement: Workspace list endpoint
The system SHALL expose GET /api/v1/workspaces returning all indexed workspaces.

#### Scenario: Request workspace list
- **WHEN** client sends GET request to /api/v1/workspaces
- **THEN** system returns JSON array of { hash, path, documentCount, lastIndexed }

### Requirement: Entity graph endpoint
The system SHALL expose GET /api/v1/graph/entities returning the knowledge graph.

#### Scenario: Request full entity graph
- **WHEN** client sends GET request to /api/v1/graph/entities
- **THEN** system returns JSON with nodes array [{id, name, type, firstLearnedAt, lastConfirmedAt}] and edges array [{sourceId, targetId, edgeType}]

#### Scenario: Request filtered entity graph
- **WHEN** client sends GET request to /api/v1/graph/entities?workspace=<hash>
- **THEN** system returns JSON filtered to entities from that workspace only

### Requirement: Graph statistics endpoint
The system SHALL expose GET /api/v1/graph/stats returning graph metrics.

#### Scenario: Request graph statistics
- **WHEN** client sends GET request to /api/v1/graph/stats
- **THEN** system returns JSON with entityCount, edgeCount, clusterCount, topEntitiesByConnections

### Requirement: Code dependencies endpoint
The system SHALL expose GET /api/v1/code/dependencies returning file dependency graph.

#### Scenario: Request code dependencies
- **WHEN** client sends GET request to /api/v1/code/dependencies
- **THEN** system returns JSON with files array [{path, centrality, clusterId}] and edges array [{source, target, type}]

### Requirement: Search endpoint
The system SHALL expose GET /api/v1/search for hybrid search queries.

#### Scenario: Execute search query
- **WHEN** client sends GET request to /api/v1/search?q=<query>
- **THEN** system executes hybrid search and returns JSON array of results with scores

### Requirement: Telemetry endpoint
The system SHALL expose GET /api/v1/telemetry returning learning system metrics.

#### Scenario: Request telemetry data
- **WHEN** client sends GET request to /api/v1/telemetry
- **THEN** system returns JSON with queryCount, expandRate, banditStats, preferenceWeights, importanceDistribution
