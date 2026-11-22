-- name: UpsertGraphEdge :exec
INSERT INTO graph_edges (workspace_id, source_node_id, target_node_id, edge_type, source_file_id, metadata)
VALUES (
    ensure_workspace_short_id(@workspace_hash::text),
    intern_graph_node(@source_node::text),
    intern_graph_node(@target_node::text),
    @edge_type,
    intern_graph_node(@source_file::text),
    @metadata
)
ON CONFLICT (workspace_id, source_node_id, target_node_id, edge_type) DO UPDATE
    SET metadata = EXCLUDED.metadata, created_at = now();

-- name: DeleteGraphEdgesByFile :exec
DELETE FROM graph_edges ge
USING graph_nodes sf
WHERE ge.workspace_id = resolve_workspace_short_id(@workspace_hash::text)
  AND ge.source_file_id = sf.id
  AND sf.node_key = @source_file::text;

-- name: GetOutgoingEdges :many
SELECT *
FROM graph_edge_views
WHERE workspace_hash = $1 AND source_node = $2
  AND ($3::text = '' OR edge_type = $3)
ORDER BY edge_type, target_node;

-- name: GetOutgoingEdgesBySymbol :many
SELECT *
FROM graph_edge_views
WHERE workspace_hash = $1
  AND source_node <> '<unresolved>'
  AND target_node <> '<unresolved>'
  AND (source_node = $2
       OR (NOT ($2 ~ '^(?:(?:[^/]+/)*[^/]+\.(?:js|jsx|mjs|cjs|ts|tsx|mts|cts))::')
           AND split_part(source_node, '::', 2) = $2))
  AND ($3::text = '' OR edge_type = $3)
ORDER BY edge_type, target_node;

-- name: GetIncomingEdges :many
SELECT *
FROM graph_edge_views
WHERE workspace_hash = $1
  AND target_node <> '<unresolved>'
  AND (target_node = $2
       OR (NOT ($2 ~ '^(?:(?:[^/]+/)*[^/]+\.(?:js|jsx|mjs|cjs|ts|tsx|mts|cts))::')
           AND NOT (target_node ~ '^(?:(?:[^/]+/)*[^/]+\.(?:js|jsx|mjs|cjs|ts|tsx|mts|cts))::')
           AND split_part(target_node, '::', 2) = $2)
       OR (strpos($2::text, '::') > 0
           AND NOT ($2 ~ '^(?:(?:[^/]+/)*[^/]+\.(?:js|jsx|mjs|cjs|ts|tsx|mts|cts))::')
           AND target_node = split_part($2::text, '::', 2)))
  AND ($3::text = '' OR edge_type = $3)
ORDER BY edge_type, source_node;

-- name: GraphStats :one
SELECT
    COUNT(*) FILTER (WHERE edge_type = 'contains') AS contains_count,
    COUNT(*) FILTER (WHERE edge_type = 'imports')  AS imports_count,
    COUNT(*) FILTER (WHERE edge_type = 'calls')    AS calls_count
FROM graph_edge_views
WHERE workspace_hash = $1
  AND (edge_type <> 'calls' OR (source_node <> '<unresolved>' AND target_node <> '<unresolved>'));

-- name: GetImpactors :many
SELECT DISTINCT sn.node_key AS source_node, ge.edge_type
FROM graph_edges ge
JOIN graph_nodes sn ON sn.id = ge.source_node_id
JOIN graph_nodes tn ON tn.id = ge.target_node_id
WHERE ge.workspace_id = resolve_workspace_short_id(@workspace_hash::text)
  AND tn.node_key <> '<unresolved>'
  AND (
      tn.node_key = @target_node::text
      OR (NOT (@target_node::text ~ '^(?:(?:[^/]+/)*[^/]+\.(?:js|jsx|mjs|cjs|ts|tsx|mts|cts))::')
          AND NOT tn.is_js_ts_qualified
          AND tn.bare_symbol = @target_node::text)
  )
  AND (@column_3::text = '' OR ge.edge_type = @column_3::text)
ORDER BY ge.edge_type, source_node;

-- name: GetImpactorsByTargets :many
SELECT DISTINCT sn.node_key AS source_node, tn.node_key AS target_node, ge.edge_type
FROM graph_edges ge
JOIN graph_nodes sn ON sn.id = ge.source_node_id
JOIN graph_nodes tn ON tn.id = ge.target_node_id
WHERE ge.workspace_id = resolve_workspace_short_id(@workspace_hash::text)
  AND tn.node_key <> '<unresolved>'
  AND (
      tn.node_key = ANY(@column_2::text[])
      OR (NOT tn.is_js_ts_qualified
          AND tn.bare_symbol <> ''
          AND tn.bare_symbol = ANY(
              ARRAY(SELECT n FROM unnest(@column_2::text[]) AS n
                    WHERE NOT (n ~ '^(?:(?:[^/]+/)*[^/]+\.(?:js|jsx|mjs|cjs|ts|tsx|mts|cts))::'))
          ))
  )
  AND (@column_3::text = '' OR ge.edge_type = @column_3::text)
ORDER BY ge.edge_type, source_node;

-- name: GetOutgoingEdgesBySources :many
SELECT DISTINCT source_node, target_node, edge_type
FROM graph_edge_views
WHERE workspace_hash = $1
  AND source_node = ANY($2::text[])
  AND source_node <> '<unresolved>'
  AND target_node <> '<unresolved>'
  AND ($3::text = '' OR edge_type = $3)
ORDER BY edge_type, target_node;

-- name: ListDocIDsByTitle :many
SELECT id FROM documents
WHERE workspace_hash = $1 AND lower(title) = lower($2);

-- name: ExistsDocByID :one
SELECT EXISTS(SELECT 1 FROM documents WHERE workspace_hash = $1 AND id = $2);

-- name: ListReferenceEdgesBySource :many
SELECT * FROM graph_edge_views
WHERE workspace_hash = $1 AND source_node = $2 AND edge_type = 'references';

-- name: UpsertReferenceEdge :exec
INSERT INTO graph_edges (workspace_id, source_node_id, target_node_id, edge_type, source_file_id, metadata)
VALUES (
    ensure_workspace_short_id(@workspace_hash::text),
    intern_graph_node(@source_node::text),
    intern_graph_node(@target_node::text),
    'references',
    intern_graph_node(@source_file::text),
    @metadata
)
ON CONFLICT (workspace_id, source_node_id, target_node_id, edge_type) DO UPDATE
SET source_file_id = EXCLUDED.source_file_id, metadata = EXCLUDED.metadata;

-- name: DeleteReferenceEdgesBySource :exec
DELETE FROM graph_edges ge
USING graph_nodes sn
WHERE ge.workspace_id = resolve_workspace_short_id(@workspace_hash::text)
  AND ge.source_node_id = sn.id
  AND sn.node_key = @source_node::text
  AND ge.edge_type = 'references';

-- name: ListTopGraphNodesByDegree :many
WITH degrees AS (
    SELECT ge.source_node AS node
    FROM graph_edge_views ge
    WHERE ge.workspace_hash = $1 AND ge.edge_type = ANY($2::text[])
      AND ge.target_node <> '<unresolved>' AND ge.source_node <> '<unresolved>'
    UNION ALL
    SELECT ge.target_node AS node
    FROM graph_edge_views ge
    WHERE ge.workspace_hash = $1 AND ge.edge_type = ANY($2::text[])
      AND ge.target_node <> '<unresolved>' AND ge.source_node <> '<unresolved>'
)
SELECT degrees.node, count(*)::bigint AS degree
FROM degrees
GROUP BY degrees.node
ORDER BY degree DESC, degrees.node ASC
LIMIT $3;

-- name: CountDistinctGraphNodes :one
WITH degrees AS (
    SELECT ge.source_node AS node
    FROM graph_edge_views ge
    WHERE ge.workspace_hash = $1 AND ge.edge_type = ANY($2::text[])
      AND ge.target_node <> '<unresolved>' AND ge.source_node <> '<unresolved>'
    UNION
    SELECT ge.target_node AS node
    FROM graph_edge_views ge
    WHERE ge.workspace_hash = $1 AND ge.edge_type = ANY($2::text[])
      AND ge.target_node <> '<unresolved>' AND ge.source_node <> '<unresolved>'
)
SELECT count(*)::bigint FROM degrees;

-- name: ListEdgesBetweenNodes :many
SELECT *
FROM graph_edge_views
WHERE workspace_hash = $1
  AND target_node <> '<unresolved>' AND source_node <> '<unresolved>'
  AND edge_type = ANY($2::text[])
  AND source_node = ANY($3::text[])
  AND target_node = ANY($3::text[]);

-- name: ListEdgesTouchingNodes :many
SELECT *
FROM graph_edge_views
WHERE workspace_hash = $1
  AND target_node <> '<unresolved>' AND source_node <> '<unresolved>'
  AND edge_type = ANY($2::text[])
  AND (source_node = ANY($3::text[]) OR target_node = ANY($3::text[]))
LIMIT $4;

-- name: ListAllEdgesByWorkspace :many
SELECT *
FROM graph_edge_views
WHERE workspace_hash = $1
ORDER BY edge_type, source_node, target_node;

-- name: ListConsumerEntryNodesByWorkspace :many
SELECT *
FROM graph_edge_views
WHERE workspace_hash = $1
  AND edge_type = 'integration'
  AND (source_node LIKE 'CONSUME %' OR source_node LIKE 'ON %')
ORDER BY source_node;

-- name: ListHTTPEndpointsByWorkspace :many
SELECT DISTINCT source_node, target_node
FROM graph_edge_views
WHERE workspace_hash = $1
  AND edge_type = 'http'
ORDER BY source_node;
