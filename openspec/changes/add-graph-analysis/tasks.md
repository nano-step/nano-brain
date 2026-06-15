## 1. Database Migrations

- [ ] 1.1 Create goose migration to add `confidence TEXT NOT NULL DEFAULT 'EXTRACTED'` to `edges` table
- [ ] 1.2 Backfill existing edges as `EXTRACTED` (they originate from AST extraction)
- [ ] 1.3 Create goose migration to add `community_id INTEGER` to `symbols` table

## 2. Core: Leiden Integration

- [ ] 2.1 Add `gonum.org/v1/gonum` dependency (`go get gonum.org/v1/gonum/graph/community`)
- [ ] 2.2 Create `internal/graph/community.go` with Leiden wrapper function
- [ ] 2.3 Implement hub exclusion: filter nodes above 90th percentile degree before Leiden
- [ ] 2.4 Implement majority-vote reattachment for excluded hubs
- [ ] 2.5 Implement oversized community splitting (>25% of graph, min 10 nodes)
- [ ] 2.6 Implement cohesion scoring (`actual_edges / (N * (N-1) / 2)`)
- [ ] 2.7 Implement stable ID remapping across runs (maximize overlap with previous assignment)

## 3. Analysis: God Nodes

- [ ] 3.1 Create `internal/graph/analysis.go` with god node ranking function
- [ ] 3.2 Implement exclusion filters: file-level hubs, concept nodes, built-in noise
- [ ] 3.3 Implement degree-based ranking with metadata (id, label, degree)

## 4. Analysis: Surprise Scoring

- [ ] 4.1 Implement composite surprise score function (5 factors)
- [ ] 4.2 Implement cross-file-type detection (code↔paper, code↔image)
- [ ] 4.3 Implement cross-repo detection (different top-level directory)
- [ ] 4.4 Implement cross-community detection (different Leiden community)
- [ ] 4.5 Implement peripheral→hub detection (low-degree → high-degree)
- [ ] 4.6 Implement false positive suppression for language boundaries

## 5. Analysis: Suggested Questions

- [ ] 5.1 Implement question generation from AMBIGUOUS edges
- [ ] 5.2 Implement question generation from bridge nodes (high betweenness)
- [ ] 5.3 Implement question generation from god nodes with INFERRED edges
- [ ] 5.4 Implement question generation from isolated nodes (degree <= 1)
- [ ] 5.5 Implement question generation from low-cohesion communities

## 6. MCP Tools

- [ ] 6.1 Add `memory_communities` MCP tool (returns all communities with members)
- [ ] 6.2 Add `memory_surprises` MCP tool (returns top N surprising connections)
- [ ] 6.3 Add `memory_god_nodes` MCP tool (returns top N god nodes)
- [ ] 6.4 Update existing MCP tools (`memory_graph`, `memory_impact`, `memory_trace`) to include `confidence` field
- [ ] 6.5 Add `confidence` filter parameter to `memory_impact` and `memory_trace`

## 7. Batch Analysis

- [ ] 7.1 Create batch analysis function that runs community detection + god nodes + surprises
- [ ] 7.2 Integrate batch analysis with graph rebuild workflow
- [ ] 7.3 Add CLI command or API endpoint to trigger batch analysis

## 8. Testing & Validation

- [ ] 8.1 Unit tests for community detection (Leiden, hub exclusion, cohesion)
- [ ] 8.2 Unit tests for surprise scoring
- [ ] 8.3 Unit tests for suggested questions
- [ ] 8.4 Integration tests for MCP tools
- [ ] 8.5 Validate against Graphify's approach: compare community detection results
