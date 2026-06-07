-- +goose Up
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS summary TEXT;
ALTER TABLE chunks ADD COLUMN IF NOT EXISTS summary_hash TEXT;

-- +goose Down
ALTER TABLE chunks DROP COLUMN IF EXISTS summary_hash;
ALTER TABLE chunks DROP COLUMN IF EXISTS summary;
