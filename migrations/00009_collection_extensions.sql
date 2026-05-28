-- +goose Up
ALTER TABLE collections ADD COLUMN IF NOT EXISTS allowed_extensions TEXT[] DEFAULT '{}';

-- +goose Down
ALTER TABLE collections DROP COLUMN IF EXISTS allowed_extensions;
