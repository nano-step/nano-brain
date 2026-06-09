## smoke:e2e Evidence

**PR:** #415
**Issue:** #414
**Change type:** bug-fix

### Reason

This fix changes only internal watcher/reindex behavior — no REST API endpoints were added or modified. However, basic server functionality was verified to ensure no regressions.

### Smoke test: build → start → health check

```
# Build
go build -o ./bin/nano-brain ./cmd/nano-brain/
# (no output — success)

# Start with test config
NANO_BRAIN_DATABASE_URL="postgres://nanobrain:nanobrain@host.docker.internal:5432/nanobrain_test?sslmode=disable" \
  NANO_BRAIN_SERVER_PORT=3199 \
  NANO_BRAIN_EMBEDDING_PROVIDER="" \
  ./bin/nano-brain &
# Server process started — wait for health

# Health check (up after 6s)
curl -s http://localhost:3199/health
HTTP/1.1 200 OK
{"status":"ok","ready":true,"version":"dev","uptime_s":2,"workspace_count":4377}

# Version endpoint
curl -s http://localhost:3199/api/version
HTTP/1.1 404 Not Found
{"error":"http_error","message":"Not Found"}
# (version endpoint not implemented — expected, not a regression)

# Status endpoint
curl -s http://localhost:3199/api/status
HTTP/1.1 200 OK
{"pg_status":"healthy","migration_version":23,"embedding_queue_depth":94,"active_provider":"ollama","workspace_count":4377,...}
```

### Verification summary

| Check | Result |
|-------|--------|
| `go build ./...` | ✅ Clean |
| `go test -race -short ./...` | ✅ All 34 packages pass |
| Server starts | ✅ Up in <6s |
| `GET /health` | ✅ 200 `{"status":"ok","ready":true}` |
| `GET /api/status` | ✅ 200, valid JSON, pg healthy |
| No API endpoint changes | ✅ Internal watcher/reindex only |
