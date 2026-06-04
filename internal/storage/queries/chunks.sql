-- name: UpsertChunk :one
INSERT INTO chunks (document_id, workspace_hash, content_hash, content, chunk_index, start_line, end_line, metadata, symbol_name, symbol_kind, language, line_start, line_end, chunk_type, embedding_strategy)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
ON CONFLICT (content_hash, workspace_hash, document_id) DO UPDATE SET
    document_id = EXCLUDED.document_id,
    content = EXCLUDED.content,
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
    embed_status = 'pending'
RETURNING id;

-- name: DeleteChunksByDocumentID :exec
DELETE FROM chunks WHERE document_id = $1 AND workspace_hash = $2;

-- name: CountChunksByDocumentID :one
SELECT count(*) FROM chunks WHERE document_id = $1 AND workspace_hash = $2;

-- name: GetChunkByID :one
SELECT c.id, c.document_id, c.workspace_hash, c.content_hash, c.content, c.chunk_index, c.start_line, c.end_line, c.metadata, c.embed_status, c.created_at,
       COALESCE(d.source_path, '') AS source_path
FROM chunks c
LEFT JOIN documents d ON d.id = c.document_id
WHERE c.id = $1;

-- name: ListChunksByDocumentID :many
SELECT id, document_id, workspace_hash, content_hash, content, chunk_index, start_line, end_line, metadata, created_at
FROM chunks WHERE document_id = $1 AND workspace_hash = $2 ORDER BY chunk_index;
