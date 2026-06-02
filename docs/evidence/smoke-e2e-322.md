# smoke:e2e Evidence for #322

PR: https://github.com/nano-step/nano-brain/pull/323
Issue: https://github.com/nano-step/nano-brain/issues/322
Lane: normal | Change type: user-feature + index-schema
Date: 2026-06-02

## Verdict: PASS

## Scope

This PR adds:
1. A new partial composite index on `chunks(embed_status, created_at)` via migration 00014
2. An in-memory `sync.Map` dedup set inside the embed queue worker

Neither change touches any user-visible HTTP endpoint, MCP tool, or CLI command. The smoke test verifies:
- Server boots cleanly with migration 00014 applied (no schema regression)
- Existing endpoints `/health` and `/api/status` still respond correctly
- `/api/status` reports `migration_version: 14` confirming our migration ran

## Smoke test execution

```bash
# Build binary
go build -o /tmp/nano-brain-322 ./cmd/nano-brain/

# Start server on port 3299 against real dev PG
NANO_BRAIN_DATABASE_URL="postgres://nanobrain:nanobrain@host.docker.internal:5432/nanobrain_dev?sslmode=disable" \
NANO_BRAIN_SERVER_PORT=3299 \
NANO_BRAIN_EMBEDDING_PROVIDER="" \
/tmp/nano-brain-322 &

# Wait for health
for i in $(seq 1 15); do curl -sf http://localhost:3299/health >/dev/null && break; sleep 1; done
```

## Endpoint verification

### `/health` — 200 OK
```
curl -sv http://localhost:3299/health
> GET /health HTTP/1.1
> Host: localhost:3299
HTTP/1.1 200 OK
Content-Type: application/json
X-Nano-Brain-Version: dev
X-Request-Id: c1599b03
Date: Tue, 02 Jun 2026 09:02:57 GMT
```

### `/api/status` — 200 OK + migration_version=14 (this PR's migration applied)
```
curl -sv http://localhost:3299/api/status
HTTP/1.1 200 OK
Content-Type: application/json
```
```json
{
  "pg_status": "healthy",
  "migration_version": 14,
  "embedding_queue_depth": 0,
  "active_provider": "",
  "workspace_count": 23,
  "queue_depth": 0,
  "queue_capacity": 0,
  "queue_status": "",
  "queue_pending": 0,
  "harvester_status": {
    "poll_interval_seconds": 120,
    "opencode": {"enabled": true, "mode": "db_path"}
  }
}
```

`migration_version: 14` confirms goose ran our migration 00014 successfully (no error, no rollback).

## Index verification

`psql` binary is not available in this container. Index existence is confirmed three ways:

1. **API confirmation** (above): `migration_version: 14` → goose applied our migration cleanly
2. **Integration test**: `TestMigration_EmbedStatusIndex_Exists` queries `pg_indexes` catalog directly via Go and asserts `idx_chunks_embed_status` exists. PASS — see `self-review-zfeat-322-embed-status-index.md`
3. **EXPLAIN ANALYZE artifact**: `docs/evidence/322-explain-analyze.txt` — captures `Index Scan using idx_chunks_embed_status` from a populated dev DB

## Surface coverage matrix

| Surface | Changed? | Smoke result |
|---|:-:|---|
| `/health` | No | HTTP 200 ✓ |
| `/api/status` | Schema unchanged | HTTP 200 ✓; reports `migration_version: 14` |
| Server boot | No | Boots cleanly, embed worker starts |
| Migration apply | NEW | 00014 applied without error; idx_chunks_embed_status created |
| MCP tools | None | N/A |
| CLI commands | None | N/A |

## Why no curl test against embed pipeline

The in-flight dedup set is purely internal worker state. No public API to enqueue chunks or inspect `inflight` size (intentionally — D10 in design.md rejected metrics scope creep). Correctness verified exhaustively by:
- 9 unit tests in `internal/embed/queue_test.go`
- 2 integration tests in `internal/embed/queue_integration_test.go`
- 3 dedicated D12-BLOCKER coverage tests (`TestQueue_HandleRetry_*`)

All PASS — see `self-review-zfeat-322-embed-status-index.md` for test:integration output.

## Reference

- Design: `openspec/changes/embed-status-index-and-inflight-dedup/design.md` D10, D11
- Self-review: `docs/evidence/self-review-zfeat-322-embed-status-index.md`
- TRACE_SPEC: Tier 2 (lane:normal)
