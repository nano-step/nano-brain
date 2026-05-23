-- name: UpsertChunk :one
INSERT INTO chunks (document_id, workspace_hash, content_hash, content, chunk_index, start_line, end_line, metadata)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (content_hash, workspace_hash, document_id) DO UPDATE SET
    document_id = EXCLUDED.document_id,
    content = EXCLUDED.content,
    chunk_index = EXCLUDED.chunk_index,
    start_line = EXCLUDED.start_line,
    end_line = EXCLUDED.end_line,
    metadata = EXCLUDED.metadata
RETURNING id;

-- name: DeleteChunksByDocumentID :exec
DELETE FROM chunks WHERE document_id = $1 AND workspace_hash = $2;

-- name: CountChunksByDocumentID :one
SELECT count(*) FROM chunks WHERE document_id = $1 AND workspace_hash = $2;

-- name: ListChunksByDocumentID :many
SELECT id, document_id, workspace_hash, content_hash, content, chunk_index, start_line, end_line, metadata, created_at
FROM chunks WHERE document_id = $1 AND workspace_hash = $2 ORDER BY chunk_index;
