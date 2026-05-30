-- name: CountDocsByCollectionGrouped :many
SELECT collection, COUNT(*) AS doc_count FROM documents
WHERE workspace_hash = $1
GROUP BY collection ORDER BY doc_count DESC;

-- name: CountChunksByEmbedStatus :many
SELECT c.embed_status, COUNT(*) AS chunk_count FROM chunks c
INNER JOIN documents d ON c.document_id = d.id
WHERE d.workspace_hash = $1
GROUP BY c.embed_status;

-- name: CountGraphEdgesByType :many
SELECT edge_type, COUNT(*) AS edge_count FROM graph_edges
WHERE workspace_hash = $1
GROUP BY edge_type;

-- name: ListTopTags :many
SELECT tag::text, COUNT(*) AS doc_count FROM (
  SELECT unnest(tags) AS tag FROM documents WHERE workspace_hash = $1
) t
GROUP BY tag ORDER BY doc_count DESC LIMIT 20;

-- name: ListRecentDocuments :many
SELECT id, title, collection, updated_at, tags FROM documents
WHERE workspace_hash = $1
ORDER BY updated_at DESC LIMIT 10;

-- name: ListRecentQueries :many
SELECT query_text, created_at FROM telemetry_logs
WHERE workspace_hash = $1 AND event_type = 'search'
ORDER BY created_at DESC LIMIT 20;

-- name: GetEdgesByNodes :many
SELECT id, workspace_hash, source_node, target_node, edge_type, source_file, metadata, created_at
FROM graph_edges
WHERE workspace_hash = $1 AND source_node = ANY($2::text[])
  AND ($3::text[] IS NULL OR edge_type = ANY($3::text[]))
ORDER BY source_node, edge_type, target_node;

-- name: ListDocumentsByIDs :many
SELECT id, title, collection, updated_at, tags FROM documents
WHERE workspace_hash = $1 AND id = ANY($2::uuid[])
ORDER BY updated_at DESC;

-- name: ListBacklinksByTarget :many
SELECT
  d.id, d.title, d.collection, d.updated_at, d.tags, d.content
FROM graph_edges ge
INNER JOIN documents d ON d.id::text = ge.source_node AND d.workspace_hash = ge.workspace_hash
WHERE ge.workspace_hash = $1 AND ge.target_node = $2 AND ge.edge_type = 'references'
ORDER BY d.updated_at DESC
LIMIT $3 OFFSET $4;

-- name: CountBacklinksByTarget :one
SELECT COUNT(*) FROM graph_edges
WHERE workspace_hash = $1 AND target_node = $2 AND edge_type = 'references';
