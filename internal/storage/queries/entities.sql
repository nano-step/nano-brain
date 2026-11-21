-- name: InsertChunkEntity :exec
INSERT INTO chunk_entities (chunk_id, workspace_id, entity_id)
SELECT @chunk_id, wi.short_id, intern_entity(@entity_name::text, @entity_type::text)
FROM workspace_ids wi
WHERE wi.workspace_hash = @workspace_hash::text
ON CONFLICT (chunk_id, entity_id) DO NOTHING;

-- name: GetChunkIDsByEntityNames :many
SELECT DISTINCT ce.chunk_id
FROM chunk_entities ce
JOIN entity_dictionary ed ON ed.id = ce.entity_id
WHERE ce.workspace_id = resolve_workspace_short_id(@workspace_hash::text)
  AND ed.entity_name = ANY(@column_2::text[]);

-- name: DeleteEntitiesByChunkIDs :exec
DELETE FROM chunk_entities WHERE chunk_id = ANY($1::uuid[]);
