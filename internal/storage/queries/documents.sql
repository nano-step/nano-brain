-- name: UpsertDocument :one
INSERT INTO documents (workspace_hash, content_hash, title, content, source_path, collection, tags, metadata)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (content_hash, workspace_hash) DO UPDATE SET
    title = EXCLUDED.title,
    content = EXCLUDED.content,
    source_path = EXCLUDED.source_path,
    collection = EXCLUDED.collection,
    tags = EXCLUDED.tags,
    metadata = EXCLUDED.metadata,
    updated_at = now()
RETURNING id, content_hash, collection, workspace_hash;

-- name: GetDocumentByHash :one
SELECT * FROM documents WHERE content_hash = $1 AND workspace_hash = $2;

-- name: ListDocumentsByWorkspace :many
SELECT id, workspace_hash, content_hash, title, source_path, collection, tags, created_at, updated_at
FROM documents WHERE workspace_hash = $1 ORDER BY updated_at DESC;
