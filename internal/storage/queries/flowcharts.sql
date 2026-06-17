-- name: UpsertFunctionFlowchart :exec
INSERT INTO function_flowcharts (workspace_hash, entry, source_file, start_line, end_line, status, cfg)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (workspace_hash, source_file, start_line, end_line) DO UPDATE
    SET entry = EXCLUDED.entry,
        status = EXCLUDED.status,
        cfg = EXCLUDED.cfg,
        updated_at = now();

-- name: GetFunctionFlowchart :one
SELECT id, workspace_hash, entry, source_file, start_line, end_line, status, cfg, created_at, updated_at
FROM function_flowcharts
WHERE workspace_hash = $1 AND source_file = $2 AND start_line = $3 AND end_line = $4;

-- name: GetFunctionFlowchartByHandler :one
SELECT id, workspace_hash, entry, source_file, start_line, end_line, status, cfg, created_at, updated_at
FROM function_flowcharts
WHERE workspace_hash = $1
  AND (entry = $2 OR split_part(entry, '::', 2) = $2)
ORDER BY start_line
LIMIT 1;

-- name: DeleteFunctionFlowchartsByFile :exec
DELETE FROM function_flowcharts WHERE workspace_hash = $1 AND source_file = $2;
