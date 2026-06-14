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

-- name: GetOutgoingEdgesBySymbol :many
SELECT id, workspace_hash, source_node, target_node, edge_type, source_file, metadata, created_at
FROM graph_edges
WHERE workspace_hash = $1
  AND (source_node = $2 OR split_part(source_node, '::', 2) = $2)
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

-- name: GetImpactors :many
SELECT DISTINCT source_node, edge_type
FROM graph_edges
WHERE workspace_hash = $1 AND target_node = $2
  AND ($3::text = '' OR edge_type = $3)
ORDER BY edge_type, source_node;

-- name: GetImpactorsByTargets :many
SELECT DISTINCT source_node, target_node, edge_type
FROM graph_edges
WHERE workspace_hash = $1 AND target_node = ANY($2::text[])
  AND ($3::text = '' OR edge_type = $3)
ORDER BY edge_type, source_node;

-- name: GetOutgoingEdgesBySources :many
SELECT DISTINCT source_node, target_node, edge_type
FROM graph_edges
WHERE workspace_hash = $1 AND source_node = ANY($2::text[])
  AND ($3::text = '' OR edge_type = $3)
ORDER BY edge_type, target_node;

-- name: ListDocIDsByTitle :many
SELECT id FROM documents
WHERE workspace_hash = $1 AND lower(title) = lower($2);

-- name: ExistsDocByID :one
SELECT EXISTS(SELECT 1 FROM documents WHERE workspace_hash = $1 AND id = $2);

-- name: ListReferenceEdgesBySource :many
SELECT * FROM graph_edges
WHERE workspace_hash = $1 AND source_node = $2 AND edge_type = 'references';

-- name: UpsertReferenceEdge :exec
INSERT INTO graph_edges (workspace_hash, source_node, target_node, edge_type, source_file, metadata)
VALUES ($1, $2, $3, 'references', $4, $5)
ON CONFLICT (workspace_hash, source_node, target_node, edge_type) DO UPDATE
SET source_file = EXCLUDED.source_file, metadata = EXCLUDED.metadata;

-- name: DeleteReferenceEdgesBySource :exec
DELETE FROM graph_edges
WHERE workspace_hash = $1 AND source_node = $2 AND edge_type = 'references';

-- name: ListTopGraphNodesByDegree :many
WITH degrees AS (
    SELECT ge.source_node AS node
    FROM graph_edges ge
    WHERE ge.workspace_hash = $1 AND ge.edge_type = ANY($2::text[])
    UNION ALL
    SELECT ge.target_node AS node
    FROM graph_edges ge
    WHERE ge.workspace_hash = $1 AND ge.edge_type = ANY($2::text[])
)
SELECT degrees.node, count(*)::bigint AS degree
FROM degrees
GROUP BY degrees.node
ORDER BY degree DESC, degrees.node ASC
LIMIT $3;

-- name: CountDistinctGraphNodes :one
WITH degrees AS (
    SELECT ge.source_node AS node
    FROM graph_edges ge
    WHERE ge.workspace_hash = $1 AND ge.edge_type = ANY($2::text[])
    UNION
    SELECT ge.target_node AS node
    FROM graph_edges ge
    WHERE ge.workspace_hash = $1 AND ge.edge_type = ANY($2::text[])
)
SELECT count(*)::bigint FROM degrees;

-- name: ListEdgesBetweenNodes :many
SELECT id, workspace_hash, source_node, target_node, edge_type, source_file, metadata, created_at
FROM graph_edges
WHERE workspace_hash = $1
  AND edge_type = ANY($2::text[])
  AND source_node = ANY($3::text[])
  AND target_node = ANY($3::text[]);

-- name: ListEdgesTouchingNodes :many
SELECT id, workspace_hash, source_node, target_node, edge_type, source_file, metadata, created_at
FROM graph_edges
WHERE workspace_hash = $1
  AND edge_type = ANY($2::text[])
  AND (source_node = ANY($3::text[]) OR target_node = ANY($3::text[]))
LIMIT $4;

-- name: ListAllEdgesByWorkspace :many
SELECT id, workspace_hash, source_node, target_node, edge_type, source_file, metadata, created_at
FROM graph_edges
WHERE workspace_hash = $1
ORDER BY edge_type, source_node, target_node;
