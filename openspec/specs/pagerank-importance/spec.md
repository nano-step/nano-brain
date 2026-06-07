# pagerank-importance Specification

## Purpose
TBD - created by archiving change enhanced-code-understanding. Update Purpose after archive.
## Requirements
### Requirement: PageRank computation
The system SHALL implement the following behavior:
WHEN graph_edges table has >=100 new/updated edges since last computation OR daily cron triggers THEN PageRank is recomputed for all symbols in the workspace (damping=0.85, max_iter=100, tol=1e-6). Scores stored in `symbol_documents.importance_score` (FLOAT, 0.0-1.0).

#### Scenario: Threshold trigger
- **GIVEN** last PageRank computed 2 hours ago
- **AND** 150 new graph edges added since then (> threshold of 100)
- **WHEN** background ticker checks edge count
- **THEN** PageRank recomputes for all symbols in workspace
- **AND** importance_scores updated in symbol_documents

### Requirement: Search importance boost
The system SHALL implement the following behavior:
WHEN hybrid search returns results AND `pagerank_enabled = true` THEN result scores are boosted: `score *= (1 + importance_score * pagerank_weight)` where default weight = 0.2.

#### Scenario: Important symbol ranks higher
- **GIVEN** symbol "ProcessFile" has importance_score = 0.9 (highly referenced)
- **AND** symbol "helperInternal" has importance_score = 0.1 (rarely referenced)
- **AND** both match a query with equal RRF score = 1.0
- **WHEN** pagerank_enabled = true AND pagerank_weight = 0.2
- **THEN** ProcessFile boosted score = 1.0 * (1 + 0.9 * 0.2) = 1.18
- **AND** helperInternal boosted score = 1.0 * (1 + 0.1 * 0.2) = 1.02
- **AND** ProcessFile ranks higher

### Requirement: Cold start handling
The system SHALL implement the following behavior:
WHEN a workspace has no graph edges THEN all `importance_score` = 0.5 (neutral, uniform boost, no ranking change).

#### Scenario: New workspace with no edges
- **GIVEN** new workspace with 0 graph edges
- **WHEN** search runs with pagerank_enabled = true
- **THEN** all symbols have importance_score = 0.5
- **AND** boost = score * (1 + 0.5 * 0.2) = score * 1.1 (uniform, no ranking change)

### Requirement: Feature flag
The system SHALL implement the following behavior:
WHEN `pagerank_enabled = false` THEN boost step is skipped entirely (zero overhead).

#### Scenario: PageRank disabled
- **GIVEN** pagerank_enabled = false
- **WHEN** search query executes
- **THEN** importance_score not read from DB
- **AND** results identical to pre-PageRank behavior

