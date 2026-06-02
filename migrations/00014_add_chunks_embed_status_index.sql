-- +goose Up
-- +goose NO TRANSACTION
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_chunks_embed_status
  ON chunks (embed_status, created_at)
  WHERE embed_status IN ('pending', 'embed_failed');

-- +goose Down
-- +goose NO TRANSACTION
DROP INDEX CONCURRENTLY IF EXISTS idx_chunks_embed_status;
