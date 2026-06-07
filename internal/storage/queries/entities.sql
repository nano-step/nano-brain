-- name: InsertChunkEntity :exec
INSERT INTO chunk_entities (chunk_id, entity_name, entity_type, workspace_hash)
VALUES ($1, $2, $3, $4)
ON CONFLICT (chunk_id, entity_name) DO NOTHING;

-- name: GetChunkIDsByEntityNames :many
SELECT DISTINCT chunk_id
FROM chunk_entities
WHERE workspace_hash = $1 AND entity_name = ANY($2::text[]);

-- name: DeleteEntitiesByChunkIDs :exec
DELETE FROM chunk_entities WHERE chunk_id = ANY($1::uuid[]);
