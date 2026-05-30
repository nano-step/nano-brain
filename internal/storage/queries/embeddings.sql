-- name: InsertEmbedding :one
INSERT INTO embeddings (chunk_id, workspace_hash, provider, model, embedding)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (chunk_id) DO UPDATE SET
    embedding = EXCLUDED.embedding,
    provider = EXCLUDED.provider,
    model = EXCLUDED.model,
    updated_at = now()
RETURNING *;

-- name: GetPendingChunks :many
SELECT c.* FROM chunks c
WHERE c.workspace_hash = $1
AND c.embed_status = 'pending'
ORDER BY c.created_at ASC
LIMIT $2;

-- name: MarkChunkEmbedded :exec
UPDATE chunks SET embed_status = 'embedded' WHERE id = $1 AND workspace_hash = $2;

-- name: MarkChunkEmbedFailed :exec
UPDATE chunks SET embed_status = 'embed_failed' WHERE id = $1 AND workspace_hash = $2;

-- name: MarkChunkEmbedPermanentlyFailed :exec
UPDATE chunks SET embed_status = 'embed_permanently_failed' WHERE id = $1 AND workspace_hash = $2;

-- name: CountPendingChunks :one
SELECT count(*) FROM chunks WHERE workspace_hash = $1 AND embed_status = 'pending';

-- name: CountEmbedFailedChunks :one
SELECT count(*) FROM chunks WHERE workspace_hash = $1 AND embed_status = 'embed_failed';

-- name: ResetEmbedStatus :execrows
UPDATE chunks SET embed_status = 'pending' WHERE workspace_hash = $1;

-- name: GetAllPendingChunks :many
SELECT id FROM chunks
WHERE embed_status = 'pending'
ORDER BY created_at ASC
LIMIT $1;

-- name: GetAllFailedChunks :many
SELECT id FROM chunks
WHERE embed_status = 'embed_failed'
ORDER BY created_at ASC
LIMIT $1;

-- name: GetPendingChunksAllWorkspaces :many
SELECT c.id FROM chunks c
WHERE c.embed_status = 'pending'
  AND EXISTS (
    SELECT 1 FROM workspaces w
    WHERE w.hash = c.workspace_hash
  )
ORDER BY c.created_at ASC
LIMIT $1;

-- name: GetFailedChunksAllWorkspaces :many
SELECT c.id FROM chunks c
WHERE c.embed_status = 'embed_failed'
  AND EXISTS (
    SELECT 1 FROM workspaces w
    WHERE w.hash = c.workspace_hash
  )
ORDER BY c.created_at ASC
LIMIT $1;

-- name: DeleteEmbeddingsByWorkspace :execrows
DELETE FROM embeddings WHERE workspace_hash = $1;

-- name: VectorSearch :many
SELECT e.id, e.chunk_id, e.workspace_hash,
       c.content, c.metadata, c.document_id,
       d.source_path, d.title, d.collection, d.tags,
       d.created_at, d.updated_at,
       CAST(1 - (e.embedding <=> sqlc.arg(query_embedding)::vector) AS double precision) AS score
FROM embeddings e
JOIN chunks c ON e.chunk_id = c.id
JOIN documents d ON c.document_id = d.id
WHERE e.workspace_hash = sqlc.arg(workspace_hash)
ORDER BY e.embedding <=> sqlc.arg(query_embedding)::vector
LIMIT sqlc.arg(max_results);

-- name: ResetEmbedStatusByCollection :execrows
UPDATE chunks
SET embed_status = 'pending'
FROM documents
WHERE chunks.document_id = documents.id
  AND chunks.workspace_hash = @workspace_hash
  AND documents.collection = @collection;

-- name: ResetAndReturnChunkIDsByCollection :many
UPDATE chunks
SET embed_status = 'pending'
FROM documents
WHERE chunks.document_id = documents.id
  AND chunks.workspace_hash = @workspace_hash
  AND documents.collection = @collection
RETURNING chunks.id;

-- name: VectorSearchAll :many
SELECT e.id, e.chunk_id, e.workspace_hash,
       c.content, c.metadata, c.document_id,
       d.source_path, d.title, d.collection, d.tags,
       d.created_at, d.updated_at,
       CAST(1 - (e.embedding <=> sqlc.arg(query_embedding)::vector) AS double precision) AS score
FROM embeddings e
JOIN chunks c ON e.chunk_id = c.id
JOIN documents d ON c.document_id = d.id
ORDER BY e.embedding <=> sqlc.arg(query_embedding)::vector
LIMIT sqlc.arg(max_results);
