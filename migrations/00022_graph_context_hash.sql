-- +goose Up
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS graph_context_hash TEXT;

-- +goose Down
ALTER TABLE chunks DROP COLUMN IF EXISTS graph_context_hash;
