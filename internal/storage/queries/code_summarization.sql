-- name: IncrementCodeSummarizationUsage :exec
INSERT INTO code_summarization_usage (workspace_hash, usage_date, request_count)
VALUES ($1, CURRENT_DATE, 1)
ON CONFLICT (workspace_hash, usage_date) DO UPDATE
SET request_count = code_summarization_usage.request_count + 1;

-- name: GetCodeSummarizationUsage :one
SELECT request_count FROM code_summarization_usage
WHERE workspace_hash = $1 AND usage_date = CURRENT_DATE;

-- name: GetUnsummarizedSymbols :many
SELECT 
    c.id,
    c.content,
    c.symbol_name,
    c.symbol_kind,
    c.language,
    c.content_hash,
    COALESCE(d.source_path, '') AS source_path
FROM chunks c
LEFT JOIN documents d ON d.id = c.document_id
WHERE c.workspace_hash = $1
  AND c.chunk_type = 'symbol'
  AND c.symbol_name IS NOT NULL
  AND NOT EXISTS (
      SELECT 1
      FROM documents doc
      WHERE doc.workspace_hash = c.workspace_hash
        AND doc.tags @> ARRAY['symbol-summary']::text[]
        AND doc.metadata->>'source_content_hash' = c.content_hash
  )
LIMIT $2;
