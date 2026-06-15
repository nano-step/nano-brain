-- +goose Up
-- Backfill search_vector for all existing chunks where it is NULL.
-- This fixes BM25 returning 0 results after migration 00023 replaced the
-- trigger function without backfilling existing data.

UPDATE chunks c
SET search_vector =
    setweight(to_tsvector(get_tsvector_config(), coalesce(d.title, '')), 'A') ||
    setweight(to_tsvector(get_tsvector_config(), coalesce(c.content, '')), 'B')
FROM documents d
WHERE c.document_id = d.id
  AND c.search_vector IS NULL;

-- +goose Down
-- No-op: backfill is idempotent and safe to re-run.
