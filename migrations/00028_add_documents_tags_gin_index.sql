-- +goose Up
-- +goose NO TRANSACTION
-- Phase 8 (08-03): the cross-workspace ticket query (ListDocumentsByTag) filters
-- with `$1 = ANY(tags)` over the documents.tags TEXT[] column with no
-- workspace_hash predicate, so without an index it forces a sequential scan of
-- the entire documents table. A GIN index on the array column makes the
-- membership test index-backed. CONCURRENTLY + NO TRANSACTION matches the
-- existing index-migration convention (see 00014/00015).
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_documents_tags
  ON documents USING GIN (tags);

-- +goose Down
-- +goose NO TRANSACTION
DROP INDEX CONCURRENTLY IF EXISTS idx_documents_tags;
