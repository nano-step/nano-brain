-- name: UpsertDocument :one
INSERT INTO documents (workspace_hash, content_hash, title, content, source_path, collection, tags, metadata, supersedes_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (source_path, workspace_hash) WHERE source_path != '' DO UPDATE SET
    content_hash = EXCLUDED.content_hash,
    title = EXCLUDED.title,
    content = EXCLUDED.content,
    collection = EXCLUDED.collection,
    tags = EXCLUDED.tags,
    metadata = EXCLUDED.metadata,
    supersedes_id = COALESCE(EXCLUDED.supersedes_id, documents.supersedes_id),
    updated_at = now()
RETURNING id, content_hash, collection, workspace_hash;

-- name: UpsertDocumentBySourcePath :one
INSERT INTO documents (workspace_hash, content_hash, title, content, source_path, collection, tags, metadata, supersedes_id, mod_time, file_size)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
ON CONFLICT (source_path, workspace_hash) WHERE source_path != '' DO UPDATE SET
    content_hash = EXCLUDED.content_hash,
    title = EXCLUDED.title,
    content = EXCLUDED.content,
    collection = EXCLUDED.collection,
    tags = EXCLUDED.tags,
    metadata = EXCLUDED.metadata,
    supersedes_id = COALESCE(EXCLUDED.supersedes_id, documents.supersedes_id),
    mod_time = EXCLUDED.mod_time,
    file_size = EXCLUDED.file_size,
    updated_at = now()
RETURNING id, content_hash, collection, workspace_hash;

-- name: GetDocumentByHash :one
SELECT * FROM documents WHERE content_hash = $1 AND workspace_hash = $2;

-- name: GetDocumentByID :one
SELECT * FROM documents WHERE id = $1 AND workspace_hash = $2;

-- name: GetDocumentBySourcePath :one
SELECT * FROM documents WHERE source_path = $1 AND workspace_hash = $2;

-- name: ListDocumentsByWorkspace :many
SELECT d.id, d.workspace_hash, d.content_hash, d.title, d.source_path, d.collection, d.tags, d.created_at, d.updated_at,
       d.supersedes_id,
       (SELECT s.id FROM documents s WHERE s.supersedes_id = d.id LIMIT 1) AS superseded_by_id
FROM documents d
WHERE d.workspace_hash = $1
ORDER BY d.updated_at DESC;

-- name: DeleteDocumentByIDAndWorkspace :execrows
DELETE FROM documents WHERE id = $1 AND workspace_hash = $2;

-- name: UpdateDocumentsCollection :exec
UPDATE documents SET collection = $2 WHERE collection = $1 AND workspace_hash = $3;

-- name: ListSymbolsByWorkspace :many
SELECT id, workspace_hash, content_hash, title, source_path, collection, tags, metadata, created_at, updated_at
FROM documents
WHERE workspace_hash = $1
  AND metadata->>'source_type' = 'symbol'
  AND ($2::text = '' OR title ILIKE '%' || $2::text || '%')
  AND ($3::text = '' OR metadata->>'kind' = $3::text)
ORDER BY title
LIMIT $4;

-- name: DeleteSymbolDocumentsByCollection :exec
DELETE FROM documents
WHERE workspace_hash = $1
  AND collection = $2
  AND metadata->>'source_type' = 'symbol';

-- name: ListTagsByWorkspace :many
SELECT unnest(tags) AS tag, COUNT(*) AS count
FROM documents
WHERE workspace_hash = $1 AND tags IS NOT NULL AND array_length(tags, 1) > 0
GROUP BY tag
ORDER BY count DESC, tag;

-- name: ListSessionDocumentsByWorkspace :many
SELECT id, workspace_hash, content_hash, title, source_path, collection, tags, content, created_at, updated_at
FROM documents
WHERE workspace_hash = @workspace_hash
  AND collection = 'sessions'
  AND (@tag_filter::text = '' OR @tag_filter::text = ANY(tags))
ORDER BY created_at DESC
LIMIT @lim;

-- name: DeleteDocumentsByWorkspace :exec
DELETE FROM documents WHERE workspace_hash = $1;

-- name: CountStaleRawOpenCodeDocs :one
-- Post-unification (phase 8), raw and summary session docs both live in
-- collection 'sessions' and are distinguished by source_path scheme:
-- raw = 'opencode://session/%', summary = 'summary://%'.
SELECT COUNT(*)::int AS n
FROM documents d_raw
WHERE d_raw.source_path LIKE 'opencode://session/%'
  AND d_raw.collection = 'sessions'
  AND EXISTS (
    SELECT 1 FROM documents d_summary
    WHERE d_summary.source_path = 'summary://opencode/' || split_part(d_raw.source_path, '/', 4)
      AND d_summary.workspace_hash = d_raw.workspace_hash
      AND d_summary.collection = 'sessions'
  );

-- name: DeleteStaleRawOpenCodeDocs :execrows
DELETE FROM documents d_raw
WHERE d_raw.source_path LIKE 'opencode://session/%'
  AND d_raw.collection = 'sessions'
  AND EXISTS (
    SELECT 1 FROM documents d_summary
    WHERE d_summary.source_path = 'summary://opencode/' || split_part(d_raw.source_path, '/', 4)
      AND d_summary.workspace_hash = d_raw.workspace_hash
      AND d_summary.collection = 'sessions'
  );

-- name: ListSummaryDocumentsForBackfill :many
-- Post-unification (phase 8), summary docs live in collection 'sessions' and
-- are identified by the 'summary://%' source_path scheme (raw session docs in
-- the same collection use 'opencode://%' / 'claude://%' schemes).
SELECT id, workspace_hash, content_hash, title, content, source_path, tags, metadata, created_at
FROM documents
WHERE collection = 'sessions'
  AND source_path LIKE 'summary://%'
  AND ($1::text = '' OR workspace_hash = $1)
  AND ($2::timestamptz IS NULL OR created_at >= $2)
ORDER BY created_at ASC;

-- name: ListDocumentSourcePathsAndHashes :many
SELECT id, source_path, content_hash
FROM documents
WHERE workspace_hash = $1
  AND collection = $2
  AND source_path != ''
ORDER BY source_path;

-- name: DeleteFlowDocumentsByWorkspace :exec
DELETE FROM documents
WHERE workspace_hash = $1
  AND collection = 'flows';

-- name: ListDocumentsByTag :many
-- Cross-workspace tag query: returns documents matching a single tag value
-- across ALL workspaces. No workspace_hash filter — intentionally global.
SELECT id, workspace_hash, title, content, source_path, collection, tags, created_at, updated_at
FROM documents
WHERE $1::text = ANY(tags)
  AND collection = $2
ORDER BY updated_at DESC
LIMIT $3;

-- name: PreloadFileStateByWorkspace :many
-- Returns the mtime+size fingerprint for all fully-fingerprinted documents in a
-- workspace+collection. Used at watcher startup to warm the in-memory fast-path
-- cache (plan 03). Rows with NULL mod_time or file_size are excluded — they were
-- indexed before migration 00029 and must fall through to normal processing.
SELECT source_path, mod_time, file_size, content_hash
FROM documents
WHERE workspace_hash = $1
  AND collection = $2
  AND source_path != ''
  AND mod_time IS NOT NULL
  AND file_size IS NOT NULL;
