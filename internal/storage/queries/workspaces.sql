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
