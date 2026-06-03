-- +goose Up
-- +goose NO TRANSACTION
-- Issue #360: Add btree indexes on documents timestamp columns
-- to enable efficient filtering by created_at and updated_at in time-range
-- filter queries (see design.md §D4). These indexes were missing across
-- all 14 prior migrations and are critical for query planner selectivity
-- once WHERE clauses on timestamps are introduced in Task 3.
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_documents_created_at
  ON documents (created_at);

CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_documents_updated_at
  ON documents (updated_at);

-- +goose Down
-- +goose NO TRANSACTION
DROP INDEX CONCURRENTLY IF EXISTS idx_documents_updated_at;
DROP INDEX CONCURRENTLY IF EXISTS idx_documents_created_at;
