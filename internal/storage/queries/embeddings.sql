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
UPDATE chunks SET embed_status = 'embedded' WHERE id = $1;

-- name: MarkChunkEmbedFailed :exec
UPDATE chunks SET embed_status = 'embed_failed' WHERE id = $1;

-- name: CountPendingChunks :one
SELECT count(*) FROM chunks WHERE workspace_hash = $1 AND embed_status = 'pending';

-- name: CountEmbedFailedChunks :one
SELECT count(*) FROM chunks WHERE workspace_hash = $1 AND embed_status = 'embed_failed';

-- name: ResetEmbedStatus :exec
UPDATE chunks SET embed_status = 'pending' WHERE workspace_hash = $1;

-- name: VectorSearch :many
SELECT e.id, e.chunk_id, e.workspace_hash, e.embedding,
       c.content, c.metadata, c.document_id,
       d.source_path, d.collection, d.tags,
       1 - (e.embedding <=> $1::vector) AS score
FROM embeddings e
JOIN chunks c ON e.chunk_id = c.id
JOIN documents d ON c.document_id = d.id
WHERE e.workspace_hash = $2
ORDER BY e.embedding <=> $1::vector
LIMIT $3;
