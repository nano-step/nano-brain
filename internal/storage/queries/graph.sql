-- name: UpsertGraphEdge :exec
INSERT INTO graph_edges (workspace_hash, source_node, target_node, edge_type, source_file, metadata)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (workspace_hash, source_node, target_node, edge_type) DO UPDATE
    SET metadata = EXCLUDED.metadata, created_at = now();

-- name: DeleteGraphEdgesByFile :exec
DELETE FROM graph_edges WHERE workspace_hash = $1 AND source_file = $2;

-- name: GetOutgoingEdges :many
SELECT id, workspace_hash, source_node, target_node, edge_type, source_file, metadata, created_at
FROM graph_edges
WHERE workspace_hash = $1 AND source_node = $2
  AND ($3::text = '' OR edge_type = $3)
ORDER BY edge_type, target_node;

-- name: GetIncomingEdges :many
SELECT id, workspace_hash, source_node, target_node, edge_type, source_file, metadata, created_at
FROM graph_edges
WHERE workspace_hash = $1 AND target_node = $2
  AND ($3::text = '' OR edge_type = $3)
ORDER BY edge_type, source_node;

-- name: GraphStats :one
SELECT
    COUNT(*) FILTER (WHERE edge_type = 'contains') AS contains_count,
    COUNT(*) FILTER (WHERE edge_type = 'imports')  AS imports_count,
    COUNT(*) FILTER (WHERE edge_type = 'calls')    AS calls_count
FROM graph_edges WHERE workspace_hash = $1;
