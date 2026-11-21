-- name: UpsertChunk :one
INSERT INTO chunks (
    document_id, workspace_hash, content_hash, blob_id, chunk_index,
    start_line, end_line, metadata, symbol_name, symbol_kind, language,
    line_start, line_end, chunk_type, embedding_strategy
)
VALUES (
    @document_id,
    @workspace_hash,
    @content_hash,
    intern_content_blob(@content::text),
    @chunk_index,
    @start_line,
    @end_line,
    @metadata,
    @symbol_name,
    @symbol_kind,
    @language,
    @line_start,
    @line_end,
    @chunk_type,
    @embedding_strategy
)
ON CONFLICT (document_id, content_hash) DO UPDATE SET
    workspace_hash = EXCLUDED.workspace_hash,
    blob_id = EXCLUDED.blob_id,
    chunk_index = EXCLUDED.chunk_index,
    start_line = EXCLUDED.start_line,
    end_line = EXCLUDED.end_line,
    metadata = EXCLUDED.metadata,
    symbol_name = EXCLUDED.symbol_name,
    symbol_kind = EXCLUDED.symbol_kind,
    language = EXCLUDED.language,
    line_start = EXCLUDED.line_start,
    line_end = EXCLUDED.line_end,
    chunk_type = EXCLUDED.chunk_type,
    embedding_strategy = EXCLUDED.embedding_strategy,
    embed_status = CASE
        WHEN EXCLUDED.blob_id != chunks.blob_id THEN 'pending'
        ELSE chunks.embed_status
    END
RETURNING id;

-- name: DeleteChunksByDocumentID :exec
DELETE FROM chunks WHERE document_id = $1 AND workspace_hash = $2;

-- name: DeleteChunksByIDs :exec
DELETE FROM chunks WHERE id = ANY($1::uuid[]);

-- name: CountChunksByDocumentID :one
SELECT count(*) FROM chunks WHERE document_id = $1 AND workspace_hash = $2;

-- name: GetChunkByID :one
SELECT c.id, c.document_id, c.workspace_hash, c.content_hash, cb.content, c.chunk_index, c.start_line, c.end_line, c.metadata, c.embed_status, c.created_at,
       COALESCE(d.source_path, '') AS source_path
FROM chunks c
JOIN content_blobs cb ON cb.id = c.blob_id
LEFT JOIN documents d ON d.id = c.document_id
WHERE c.id = $1;

-- name: ListChunksByDocumentID :many
SELECT c.id, c.document_id, c.workspace_hash, c.content_hash, cb.content, c.chunk_index, c.start_line, c.end_line, c.metadata, c.created_at
FROM chunks c
JOIN content_blobs cb ON cb.id = c.blob_id
WHERE c.document_id = $1 AND c.workspace_hash = $2
ORDER BY c.chunk_index;
