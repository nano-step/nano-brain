-- +goose Up
ALTER TABLE telemetry_logs ADD COLUMN IF NOT EXISTS query_text TEXT NOT NULL DEFAULT '';
ALTER TABLE telemetry_logs ADD COLUMN IF NOT EXISTS result_count INTEGER NOT NULL DEFAULT 0;
ALTER TABLE telemetry_logs ADD COLUMN IF NOT EXISTS latency_ms INTEGER NOT NULL DEFAULT 0;
ALTER TABLE telemetry_logs ADD COLUMN IF NOT EXISTS collection TEXT NOT NULL DEFAULT '';
CREATE INDEX IF NOT EXISTS idx_telemetry_logs_created_at ON telemetry_logs(created_at);

-- +goose Down
DROP INDEX IF EXISTS idx_telemetry_logs_created_at;
ALTER TABLE telemetry_logs DROP COLUMN IF EXISTS collection;
ALTER TABLE telemetry_logs DROP COLUMN IF EXISTS latency_ms;
ALTER TABLE telemetry_logs DROP COLUMN IF EXISTS result_count;
ALTER TABLE telemetry_logs DROP COLUMN IF EXISTS query_text;
