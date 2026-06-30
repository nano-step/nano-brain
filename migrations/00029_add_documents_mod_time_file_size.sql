-- Phase 999.1 / D-06b: persist the watcher fast-path fingerprint (mtime + byte size)
-- so the in-memory cache survives process restart and new-worktree registration.
-- Both columns are nullable; existing rows receive values lazily on next index run.

-- +goose Up
ALTER TABLE documents
    ADD COLUMN IF NOT EXISTS mod_time  TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS file_size BIGINT;

-- +goose Down
ALTER TABLE documents
    DROP COLUMN IF EXISTS file_size,
    DROP COLUMN IF EXISTS mod_time;
