-- name: InsertEmbedding :one
WITH live_chunk AS (
    SELECT c.id, c.blob_id, wi.short_id AS workspace_id
    FROM chunks c
    JOIN workspace_ids wi ON wi.workspace_hash = c.workspace_hash
    WHERE c.id = sqlc.arg(chunk_id) AND c.workspace_hash = sqlc.arg(workspace_hash)
    FOR KEY SHARE OF c
),
vec_row AS (
    INSERT INTO embedding_vectors (blob_id, provider, model, embedding)
    SELECT live_chunk.blob_id,
           sqlc.arg(provider)::text,
           sqlc.arg(model)::text,
           sqlc.arg(embedding)::vector::halfvec(768)
    FROM live_chunk
    ON CONFLICT (blob_id, provider, model) DO UPDATE SET
        embedding = EXCLUDED.embedding
    RETURNING id
)
INSERT INTO embeddings (chunk_id, workspace_id, vector_id)
SELECT live_chunk.id,
       live_chunk.workspace_id,
       vec_row.id
FROM live_chunk
CROSS JOIN vec_row
ON CONFLICT (chunk_id) DO UPDATE SET
    vector_id = EXCLUDED.vector_id,
    updated_at = now()
RETURNING *;

-- name: ReuseEmbeddingIfCached :one
WITH live_chunk AS (
    SELECT c.id, c.blob_id, wi.short_id AS workspace_id
    FROM chunks c
    JOIN workspace_ids wi ON wi.workspace_hash = c.workspace_hash
    WHERE c.id = sqlc.arg(chunk_id) AND c.workspace_hash = sqlc.arg(workspace_hash)
    FOR KEY SHARE OF c
),
cached_vec AS (
    SELECT ev.id
    FROM live_chunk
    JOIN embedding_vectors ev
      ON ev.blob_id = live_chunk.blob_id
     AND ev.provider = sqlc.arg(provider)::text
     AND ev.model = sqlc.arg(model)::text
),
linked AS (
    INSERT INTO embeddings (chunk_id, workspace_id, vector_id)
    SELECT live_chunk.id, live_chunk.workspace_id, cached_vec.id
    FROM live_chunk
    CROSS JOIN cached_vec
    ON CONFLICT (chunk_id) DO UPDATE SET
        vector_id = EXCLUDED.vector_id,
        updated_at = now()
    RETURNING chunk_id
)
UPDATE chunks
SET embed_status = 'embedded'
FROM linked
WHERE chunks.id = linked.chunk_id
RETURNING chunks.id;

-- name: GetPendingChunks :many
SELECT * FROM chunk_views
WHERE workspace_hash = $1
AND embed_status = 'pending'
ORDER BY created_at ASC
LIMIT $2;

-- name: MarkChunkEmbedded :exec
UPDATE chunks SET embed_status = 'embedded' WHERE id = $1 AND workspace_hash = $2;

-- name: MarkChunkEmbedFailed :exec
UPDATE chunks SET embed_status = 'embed_failed' WHERE id = $1 AND workspace_hash = $2;

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
DELETE FROM embeddings WHERE workspace_id = resolve_workspace_short_id($1);

-- name: VectorSearch :many
SELECT e.id, e.chunk_id, c.workspace_hash,
       cb.content, c.metadata, c.document_id,
       d.source_path, d.title, d.collection, d.tags,
       d.created_at, d.updated_at,
       CAST(1 - (ev.embedding <=> sqlc.arg(query_embedding)::vector::halfvec(768)) AS double precision) AS score
FROM embeddings e
JOIN embedding_vectors ev ON ev.id = e.vector_id
JOIN chunks c ON e.chunk_id = c.id
JOIN content_blobs cb ON cb.id = c.blob_id
JOIN documents d ON c.document_id = d.id
WHERE e.workspace_id = resolve_workspace_short_id(sqlc.arg(workspace_hash))
  AND (sqlc.narg('chunk_type')::text IS NULL OR c.chunk_type = sqlc.narg('chunk_type'))
  AND (sqlc.narg('updated_after')::timestamptz IS NULL OR d.updated_at >= sqlc.narg('updated_after'))
  AND (sqlc.narg('updated_before')::timestamptz IS NULL OR d.updated_at <= sqlc.narg('updated_before'))
  AND (sqlc.narg('created_after')::timestamptz IS NULL OR d.created_at >= sqlc.narg('created_after'))
  AND (sqlc.narg('created_before')::timestamptz IS NULL OR d.created_at <= sqlc.narg('created_before'))
ORDER BY ev.embedding <=> sqlc.arg(query_embedding)::vector::halfvec(768)
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
SELECT e.id, e.chunk_id, c.workspace_hash,
       cb.content, c.metadata, c.document_id,
       d.source_path, d.title, d.collection, d.tags,
       d.created_at, d.updated_at,
       CAST(1 - (ev.embedding <=> sqlc.arg(query_embedding)::vector::halfvec(768)) AS double precision) AS score
FROM embeddings e
JOIN embedding_vectors ev ON ev.id = e.vector_id
JOIN chunks c ON e.chunk_id = c.id
JOIN content_blobs cb ON cb.id = c.blob_id
JOIN documents d ON c.document_id = d.id
WHERE (sqlc.narg('chunk_type')::text IS NULL OR c.chunk_type = sqlc.narg('chunk_type'))
  AND (sqlc.narg('updated_after')::timestamptz IS NULL OR d.updated_at >= sqlc.narg('updated_after'))
  AND (sqlc.narg('updated_before')::timestamptz IS NULL OR d.updated_at <= sqlc.narg('updated_before'))
  AND (sqlc.narg('created_after')::timestamptz IS NULL OR d.created_at >= sqlc.narg('created_after'))
  AND (sqlc.narg('created_before')::timestamptz IS NULL OR d.created_at <= sqlc.narg('created_before'))
ORDER BY ev.embedding <=> sqlc.arg(query_embedding)::vector::halfvec(768)
LIMIT sqlc.arg(max_results);

-- name: VectorSearchWithTags :many
SELECT e.id, e.chunk_id, c.workspace_hash,
       cb.content, c.metadata, c.document_id,
       d.source_path, d.title, d.collection, d.tags,
       d.created_at, d.updated_at,
       CAST(1 - (ev.embedding <=> sqlc.arg(query_embedding)::vector::halfvec(768)) AS double precision) AS score
FROM embeddings e
JOIN embedding_vectors ev ON ev.id = e.vector_id
JOIN chunks c ON e.chunk_id = c.id
JOIN content_blobs cb ON cb.id = c.blob_id
JOIN documents d ON c.document_id = d.id
WHERE e.workspace_id = resolve_workspace_short_id(sqlc.arg(workspace_hash))
  AND d.tags && sqlc.arg(tags)::text[]
  AND (sqlc.narg('chunk_type')::text IS NULL OR c.chunk_type = sqlc.narg('chunk_type'))
  AND (sqlc.narg('updated_after')::timestamptz IS NULL OR d.updated_at >= sqlc.narg('updated_after'))
  AND (sqlc.narg('updated_before')::timestamptz IS NULL OR d.updated_at <= sqlc.narg('updated_before'))
  AND (sqlc.narg('created_after')::timestamptz IS NULL OR d.created_at >= sqlc.narg('created_after'))
  AND (sqlc.narg('created_before')::timestamptz IS NULL OR d.created_at <= sqlc.narg('created_before'))
ORDER BY ev.embedding <=> sqlc.arg(query_embedding)::vector::halfvec(768)
LIMIT sqlc.arg(max_results);

-- name: VectorSearchAllWithTags :many
SELECT e.id, e.chunk_id, c.workspace_hash,
       cb.content, c.metadata, c.document_id,
       d.source_path, d.title, d.collection, d.tags,
       d.created_at, d.updated_at,
       CAST(1 - (ev.embedding <=> sqlc.arg(query_embedding)::vector::halfvec(768)) AS double precision) AS score
FROM embeddings e
JOIN embedding_vectors ev ON ev.id = e.vector_id
JOIN chunks c ON e.chunk_id = c.id
JOIN content_blobs cb ON cb.id = c.blob_id
JOIN documents d ON c.document_id = d.id
WHERE d.tags && sqlc.arg(tags)::text[]
  AND (sqlc.narg('chunk_type')::text IS NULL OR c.chunk_type = sqlc.narg('chunk_type'))
  AND (sqlc.narg('updated_after')::timestamptz IS NULL OR d.updated_at >= sqlc.narg('updated_after'))
  AND (sqlc.narg('updated_before')::timestamptz IS NULL OR d.updated_at <= sqlc.narg('updated_before'))
  AND (sqlc.narg('created_after')::timestamptz IS NULL OR d.created_at >= sqlc.narg('created_after'))
  AND (sqlc.narg('created_before')::timestamptz IS NULL OR d.created_at <= sqlc.narg('created_before'))
ORDER BY ev.embedding <=> sqlc.arg(query_embedding)::vector::halfvec(768)
LIMIT sqlc.arg(max_results);

-- name: CountEmbeddingsByWorkspace :one
SELECT count(*) FROM embeddings WHERE workspace_id = resolve_workspace_short_id($1);
