-- name: BM25Search :many
SELECT c.id, c.document_id, c.workspace_hash, c.content, c.chunk_index, c.metadata,
       d.source_path, d.title, d.collection, d.tags,
       d.created_at, d.updated_at,
       CAST(ts_rank_cd(c.search_vector, websearch_to_tsquery('english', sqlc.arg(query)::text)) AS double precision) AS score
FROM chunks c
JOIN documents d ON c.document_id = d.id
WHERE c.workspace_hash = sqlc.arg(workspace_hash)
  AND c.search_vector @@ websearch_to_tsquery('english', sqlc.arg(query)::text)
ORDER BY score DESC, c.id ASC
LIMIT sqlc.arg(max_results);

-- name: BM25SearchWithTags :many
SELECT c.id, c.document_id, c.workspace_hash, c.content, c.chunk_index, c.metadata,
       d.source_path, d.title, d.collection, d.tags,
       d.created_at, d.updated_at,
       CAST(ts_rank_cd(c.search_vector, websearch_to_tsquery('english', sqlc.arg(query)::text)) AS double precision) AS score
FROM chunks c
JOIN documents d ON c.document_id = d.id
WHERE c.workspace_hash = sqlc.arg(workspace_hash)
  AND c.search_vector @@ websearch_to_tsquery('english', sqlc.arg(query)::text)
  AND d.tags && sqlc.arg(tags)::text[]
ORDER BY score DESC, c.id ASC
LIMIT sqlc.arg(max_results);
