-- name: RecentDocuments :many
SELECT id, title, tags, updated_at,
       LEFT(content, 200) AS snippet
FROM documents
WHERE workspace_hash = $1
ORDER BY updated_at DESC
LIMIT $2;

-- name: WorkspaceDocStats :one
SELECT count(*)::bigint AS total_documents,
       max(updated_at) AS last_updated
FROM documents
WHERE workspace_hash = $1;

-- name: WorkspaceChunkCount :one
SELECT count(*)::bigint AS total_chunks
FROM chunks
WHERE workspace_hash = $1;

-- name: ListCollectionsWithLastUpdated :many
SELECT c.name,
       COALESCE(d.cnt, 0)::bigint AS document_count,
       d.last_updated
FROM collections c
LEFT JOIN (
    SELECT collection, count(*) AS cnt, max(updated_at) AS last_updated
    FROM documents doc
    WHERE doc.workspace_hash = sqlc.arg(workspace_hash)
    GROUP BY collection
) d ON c.name = d.collection
WHERE c.workspace_hash = sqlc.arg(workspace_hash)
ORDER BY c.name;
