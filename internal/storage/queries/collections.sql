-- name: UpsertCollection :one
INSERT INTO collections (workspace_hash, name, path, glob_pattern, update_mode)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (name, workspace_hash) DO UPDATE SET
    path = EXCLUDED.path,
    glob_pattern = EXCLUDED.glob_pattern,
    update_mode = EXCLUDED.update_mode,
    updated_at = now()
RETURNING *;

-- name: ListCollections :many
SELECT * FROM collections WHERE workspace_hash = $1 ORDER BY name;

-- name: GetCollectionByName :one
SELECT * FROM collections WHERE name = $1 AND workspace_hash = $2;

-- name: RenameCollection :one
UPDATE collections SET name = $2, updated_at = now()
WHERE name = $1 AND workspace_hash = $3
RETURNING *;

-- name: DeleteCollection :exec
DELETE FROM collections WHERE name = $1 AND workspace_hash = $2;

-- name: CountDocumentsByCollection :one
SELECT count(*) FROM documents WHERE collection = $1 AND workspace_hash = $2;
