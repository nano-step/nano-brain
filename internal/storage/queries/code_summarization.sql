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

-- name: UpsertCodeSummarizationFailure :exec
INSERT INTO code_summarization_failures (workspace_hash, symbol_name, symbol_kind, source_file, content_hash, error_reason, error_type, attempts, last_attempt_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
ON CONFLICT (workspace_hash, content_hash) WHERE resolved_at IS NULL
DO UPDATE SET attempts = EXCLUDED.attempts, error_reason = EXCLUDED.error_reason, error_type = EXCLUDED.error_type, last_attempt_at = NOW();

-- name: UpdateCodeSummarizationFailure :exec
UPDATE code_summarization_failures
SET attempts = $2, error_reason = $3, last_attempt_at = NOW()
WHERE id = $1;

-- name: GetUnresolvedFailures :many
SELECT id, workspace_hash, symbol_name, symbol_kind, source_file, content_hash, error_reason, error_type, attempts, last_attempt_at, created_at
FROM code_summarization_failures
WHERE workspace_hash = $1 AND resolved_at IS NULL
ORDER BY last_attempt_at DESC;

-- name: ResolveFailure :exec
UPDATE code_summarization_failures
SET resolved_at = NOW()
WHERE id = $1;

-- name: GetSummarizationStatus :one
SELECT
    (SELECT COUNT(*) FROM chunks c WHERE c.workspace_hash = $1 AND c.chunk_type = 'symbol' AND c.symbol_name IS NOT NULL)::int AS total_symbols,
    (SELECT COUNT(*) FROM documents d WHERE d.workspace_hash = $1 AND d.tags @> ARRAY['symbol-summary'])::int AS summarized,
    (SELECT COUNT(*) FROM code_summarization_failures csf WHERE csf.workspace_hash = $1 AND csf.resolved_at IS NULL)::int AS failed;

-- name: GetFailureBySymbol :one
SELECT id, attempts FROM code_summarization_failures
WHERE workspace_hash = $1 AND content_hash = $2 AND resolved_at IS NULL
LIMIT 1;
