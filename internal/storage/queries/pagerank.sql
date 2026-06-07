-- name: ListCallEdges :many
SELECT source_node, target_node
FROM graph_edges
WHERE workspace_hash = $1 AND edge_type = 'calls';

-- name: UpsertPageRankScore :exec
INSERT INTO pagerank_scores (workspace_hash, node_name, score, computed_at)
VALUES ($1, $2, $3, now())
ON CONFLICT (workspace_hash, node_name) DO UPDATE
    SET score = EXCLUDED.score, computed_at = now();

-- name: GetPageRankScores :many
SELECT node_name, score
FROM pagerank_scores
WHERE workspace_hash = $1;

-- name: DeletePageRankScores :exec
DELETE FROM pagerank_scores WHERE workspace_hash = $1;

-- name: CountCallEdges :one
SELECT count(*)::bigint FROM graph_edges
WHERE workspace_hash = $1 AND edge_type = 'calls';
