-- +goose Up
-- +goose NO TRANSACTION
CREATE INDEX CONCURRENTLY IF NOT EXISTS idx_documents_workspace_symbol_title
    ON documents (workspace_hash, title)
    WHERE metadata->>'source_type' = 'symbol';

-- +goose Down
-- +goose NO TRANSACTION
DROP INDEX CONCURRENTLY IF EXISTS idx_documents_workspace_symbol_title;
