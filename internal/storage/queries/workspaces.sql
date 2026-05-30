-- name: UpsertWorkspace :one
INSERT INTO workspaces (hash, name, path)
VALUES ($1, $2, $3)
ON CONFLICT (hash) DO UPDATE SET
    name = EXCLUDED.name,
    path = EXCLUDED.path,
    updated_at = now()
RETURNING *;

-- name: GetWorkspaceByHash :one
SELECT * FROM workspaces WHERE hash = $1;

-- name: ListWorkspaces :many
SELECT * FROM workspaces ORDER BY name;

-- name: CountDocumentsByWorkspace :one
SELECT COUNT(*) FROM documents WHERE workspace_hash = $1;

-- name: CountWorkspaces :one
SELECT COUNT(*) FROM workspaces;

-- name: ListWorkspacesWithStats :many
SELECT w.*,
    (SELECT COUNT(*) FROM documents d WHERE d.workspace_hash = w.hash) AS document_count,
    (SELECT MAX(d.updated_at) FROM documents d WHERE d.workspace_hash = w.hash) AS last_document_updated
FROM workspaces w
ORDER BY w.name;

-- name: DeleteWorkspace :exec
DELETE FROM workspaces WHERE hash = $1;

-- name: ListOrphanDocumentWorkspaces :many
SELECT d.workspace_hash AS workspace_hash, COUNT(*) AS doc_count
FROM documents d
LEFT JOIN workspaces w ON d.workspace_hash = w.hash
WHERE w.hash IS NULL
GROUP BY d.workspace_hash
ORDER BY doc_count DESC;

-- name: CountOrphanDocuments :one
SELECT COUNT(*) FROM documents d
LEFT JOIN workspaces w ON d.workspace_hash = w.hash
WHERE w.hash IS NULL;

-- name: CountOrphanChunks :one
SELECT COUNT(*) FROM chunks c
LEFT JOIN workspaces w ON c.workspace_hash = w.hash
WHERE w.hash IS NULL;

-- name: DeleteOrphanDocuments :execrows
DELETE FROM documents
WHERE workspace_hash IN (
    SELECT d.workspace_hash FROM documents d
    LEFT JOIN workspaces w ON d.workspace_hash = w.hash
    WHERE w.hash IS NULL
);

-- name: DeleteOrphanChunks :execrows
DELETE FROM chunks
WHERE workspace_hash IN (
    SELECT c.workspace_hash FROM chunks c
    LEFT JOIN workspaces w ON c.workspace_hash = w.hash
    WHERE w.hash IS NULL
);
