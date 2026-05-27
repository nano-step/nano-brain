-- name: UpsertDocument :one
INSERT INTO documents (workspace_hash, content_hash, title, content, source_path, collection, tags, metadata, supersedes_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (source_path, workspace_hash) WHERE source_path != '' DO UPDATE SET
    content_hash = EXCLUDED.content_hash,
    title = EXCLUDED.title,
    content = EXCLUDED.content,
    collection = EXCLUDED.collection,
    tags = EXCLUDED.tags,
    metadata = EXCLUDED.metadata,
    supersedes_id = COALESCE(EXCLUDED.supersedes_id, documents.supersedes_id),
    updated_at = now()
RETURNING id, content_hash, collection, workspace_hash;

-- name: UpsertDocumentBySourcePath :one
INSERT INTO documents (workspace_hash, content_hash, title, content, source_path, collection, tags, metadata, supersedes_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (source_path, workspace_hash) WHERE source_path != '' DO UPDATE SET
    content_hash = EXCLUDED.content_hash,
    title = EXCLUDED.title,
    content = EXCLUDED.content,
    collection = EXCLUDED.collection,
    tags = EXCLUDED.tags,
    metadata = EXCLUDED.metadata,
    supersedes_id = COALESCE(EXCLUDED.supersedes_id, documents.supersedes_id),
    updated_at = now()
RETURNING id, content_hash, collection, workspace_hash;

-- name: GetDocumentByHash :one
SELECT * FROM documents WHERE content_hash = $1 AND workspace_hash = $2;

-- name: GetDocumentByID :one
SELECT * FROM documents WHERE id = $1 AND workspace_hash = $2;

-- name: GetDocumentBySourcePath :one
SELECT * FROM documents WHERE source_path = $1 AND workspace_hash = $2;

-- name: ListDocumentsByWorkspace :many
SELECT id, workspace_hash, content_hash, title, source_path, collection, tags, created_at, updated_at
FROM documents WHERE workspace_hash = $1 ORDER BY updated_at DESC;

-- name: UpdateDocumentsCollection :exec
UPDATE documents SET collection = $2 WHERE collection = $1 AND workspace_hash = $3;

-- name: ListSymbolsByWorkspace :many
SELECT id, workspace_hash, content_hash, title, source_path, collection, tags, metadata, created_at, updated_at
FROM documents
WHERE workspace_hash = $1
  AND metadata->>'source_type' = 'symbol'
  AND ($2::text = '' OR title ILIKE '%' || $2::text || '%')
  AND ($3::text = '' OR metadata->>'kind' = $3::text)
ORDER BY title
LIMIT $4;

-- name: DeleteSymbolDocumentsByCollection :exec
DELETE FROM documents
WHERE workspace_hash = $1
  AND collection = $2
  AND metadata->>'source_type' = 'symbol';

-- name: ListTagsByWorkspace :many
SELECT unnest(tags) AS tag, COUNT(*) AS count
FROM documents
WHERE workspace_hash = $1 AND tags IS NOT NULL AND array_length(tags, 1) > 0
GROUP BY tag
ORDER BY count DESC, tag;

-- name: ListSessionDocumentsByWorkspace :many
SELECT id, workspace_hash, content_hash, title, source_path, collection, tags, content, created_at, updated_at
FROM documents
WHERE workspace_hash = @workspace_hash
  AND collection = 'sessions'
  AND (@tag_filter::text = '' OR @tag_filter::text = ANY(tags))
ORDER BY created_at DESC
LIMIT @lim;

-- name: DeleteDocumentsByWorkspace :exec
DELETE FROM documents WHERE workspace_hash = $1;
