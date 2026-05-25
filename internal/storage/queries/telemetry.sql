-- name: InsertSearchTelemetry :exec
INSERT INTO telemetry_logs (workspace_hash, event_type, query_text, result_count, latency_ms, collection)
VALUES ($1, 'search', $2, $3, $4, $5);

-- name: CleanupTelemetryLogs :execresult
DELETE FROM telemetry_logs WHERE created_at < now() - make_interval(days => $1::int);
