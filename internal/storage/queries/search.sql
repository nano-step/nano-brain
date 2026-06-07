-- name: BM25Search :many
SELECT c.id, c.document_id, c.workspace_hash, c.content, c.chunk_index, c.metadata,
       d.source_path, d.title, d.collection, d.tags,
       d.created_at, d.updated_at,
       CAST(ts_rank_cd(c.search_vector, websearch_to_tsquery(current_setting('nanobrain.tsvector_config', true)::regconfig, sqlc.arg(query)::text)) AS double precision) AS score
FROM chunks c
JOIN documents d ON c.document_id = d.id
WHERE c.workspace_hash = sqlc.arg(workspace_hash)
  AND c.search_vector @@ websearch_to_tsquery(current_setting('nanobrain.tsvector_config', true)::regconfig, sqlc.arg(query)::text)
  AND (sqlc.narg('chunk_type')::text IS NULL OR c.chunk_type = sqlc.narg('chunk_type'))
  AND (sqlc.narg('updated_after')::timestamptz IS NULL OR d.updated_at >= sqlc.narg('updated_after'))
  AND (sqlc.narg('updated_before')::timestamptz IS NULL OR d.updated_at <= sqlc.narg('updated_before'))
  AND (sqlc.narg('created_after')::timestamptz IS NULL OR d.created_at >= sqlc.narg('created_after'))
  AND (sqlc.narg('created_before')::timestamptz IS NULL OR d.created_at <= sqlc.narg('created_before'))
ORDER BY score DESC, c.id ASC
LIMIT sqlc.arg(max_results);

-- name: BM25SearchAll :many
SELECT c.id, c.document_id, c.workspace_hash, c.content, c.chunk_index, c.metadata,
       d.source_path, d.title, d.collection, d.tags,
       d.created_at, d.updated_at,
       CAST(ts_rank_cd(c.search_vector, websearch_to_tsquery(current_setting('nanobrain.tsvector_config', true)::regconfig, sqlc.arg(query)::text)) AS double precision) AS score
FROM chunks c
JOIN documents d ON c.document_id = d.id
WHERE c.search_vector @@ websearch_to_tsquery(current_setting('nanobrain.tsvector_config', true)::regconfig, sqlc.arg(query)::text)
  AND (sqlc.narg('chunk_type')::text IS NULL OR c.chunk_type = sqlc.narg('chunk_type'))
  AND (sqlc.narg('updated_after')::timestamptz IS NULL OR d.updated_at >= sqlc.narg('updated_after'))
  AND (sqlc.narg('updated_before')::timestamptz IS NULL OR d.updated_at <= sqlc.narg('updated_before'))
  AND (sqlc.narg('created_after')::timestamptz IS NULL OR d.created_at >= sqlc.narg('created_after'))
  AND (sqlc.narg('created_before')::timestamptz IS NULL OR d.created_at <= sqlc.narg('created_before'))
ORDER BY score DESC, c.id ASC
LIMIT sqlc.arg(max_results);

-- name: BM25SearchWithTags :many
SELECT c.id, c.document_id, c.workspace_hash, c.content, c.chunk_index, c.metadata,
       d.source_path, d.title, d.collection, d.tags,
       d.created_at, d.updated_at,
       CAST(ts_rank_cd(c.search_vector, websearch_to_tsquery(current_setting('nanobrain.tsvector_config', true)::regconfig, sqlc.arg(query)::text)) AS double precision) AS score
FROM chunks c
JOIN documents d ON c.document_id = d.id
WHERE c.workspace_hash = sqlc.arg(workspace_hash)
  AND c.search_vector @@ websearch_to_tsquery(current_setting('nanobrain.tsvector_config', true)::regconfig, sqlc.arg(query)::text)
  AND d.tags && sqlc.arg(tags)::text[]
  AND (sqlc.narg('chunk_type')::text IS NULL OR c.chunk_type = sqlc.narg('chunk_type'))
  AND (sqlc.narg('updated_after')::timestamptz IS NULL OR d.updated_at >= sqlc.narg('updated_after'))
  AND (sqlc.narg('updated_before')::timestamptz IS NULL OR d.updated_at <= sqlc.narg('updated_before'))
  AND (sqlc.narg('created_after')::timestamptz IS NULL OR d.created_at >= sqlc.narg('created_after'))
  AND (sqlc.narg('created_before')::timestamptz IS NULL OR d.created_at <= sqlc.narg('created_before'))
ORDER BY score DESC, c.id ASC
LIMIT sqlc.arg(max_results);

-- name: BM25SearchAllWithTags :many
SELECT c.id, c.document_id, c.workspace_hash, c.content, c.chunk_index, c.metadata,
       d.source_path, d.title, d.collection, d.tags,
       d.created_at, d.updated_at,
       CAST(ts_rank_cd(c.search_vector, websearch_to_tsquery(current_setting('nanobrain.tsvector_config', true)::regconfig, sqlc.arg(query)::text)) AS double precision) AS score
FROM chunks c
JOIN documents d ON c.document_id = d.id
WHERE c.search_vector @@ websearch_to_tsquery(current_setting('nanobrain.tsvector_config', true)::regconfig, sqlc.arg(query)::text)
  AND d.tags && sqlc.arg(tags)::text[]
  AND (sqlc.narg('chunk_type')::text IS NULL OR c.chunk_type = sqlc.narg('chunk_type'))
  AND (sqlc.narg('updated_after')::timestamptz IS NULL OR d.updated_at >= sqlc.narg('updated_after'))
  AND (sqlc.narg('updated_before')::timestamptz IS NULL OR d.updated_at <= sqlc.narg('updated_before'))
  AND (sqlc.narg('created_after')::timestamptz IS NULL OR d.created_at >= sqlc.narg('created_after'))
  AND (sqlc.narg('created_before')::timestamptz IS NULL OR d.created_at <= sqlc.narg('created_before'))
ORDER BY score DESC, c.id ASC
LIMIT sqlc.arg(max_results);
