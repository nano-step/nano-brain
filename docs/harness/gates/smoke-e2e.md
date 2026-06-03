# Gate: smoke-e2e

End-to-end smoke test verifying the server starts and changed endpoints work.

## When Required

- Change type = **user-feature** or **bug-fix** → MUST run
- Change type = infrastructure / refactor / docs / deps → SKIP (document reason)

## Hard Rules

1. **Binary builds** — `go build -o ./bin/nano-brain ./cmd/nano-brain/` must succeed
2. **Server starts on port 3199** — Health endpoint responds within 15 seconds
3. **Changed endpoints return correct status** — HTTP status codes match spec
4. **Response JSON has required fields** — Spot-check key fields in responses
5. **No panics** — Server must not panic on any tested request
6. **Evidence captured** — Output saved to `docs/evidence/<slug>/smoke-e2e-output.log`

## Step-by-Step Procedure

```bash
# 1. Build binary
go build -o ./bin/nano-brain ./cmd/nano-brain/

# 2. Start server on non-default port (3199 = test port)
NANO_BRAIN_DATABASE_URL="postgres://nanobrain:nanobrain@host.docker.internal:5432/nanobrain_dev?sslmode=disable" \
NANO_BRAIN_SERVER_PORT=3199 \
NANO_BRAIN_EMBEDDING_PROVIDER="" \
./bin/nano-brain &
SERVER_PID=$!

# 3. Wait for health (15s timeout)
for i in $(seq 1 15); do curl -sf http://localhost:3199/health >/dev/null && break; sleep 1; done

# 4. Exercise changed endpoints
# Example for /api/v1/write:
curl -sf -X POST http://localhost:3199/api/v1/write \
  -H "Content-Type: application/json" \
  -d '{"workspace":"test","source_path":"test.md","content":"test"}'

# For each changed endpoint, verify:
#   - HTTP status code matches spec (200, 201, 400, etc.)
#   - Response JSON structure has required fields
#   - If bug fix: confirm broken case now works

# 5. Kill server
kill $SERVER_PID; wait $SERVER_PID 2>/dev/null
```

## Evidence Requirements

Capture all curl output to the evidence file:

```bash
mkdir -p docs/evidence/<slug>
{
  echo "=== smoke:e2e run at $(date) ==="
  echo "Binary: $(./bin/nano-brain --version 2>&1 || echo 'no version')"
  # ... curl commands and responses ...
  echo "=== RESULT: smoke:e2e PASS ==="
} | tee docs/evidence/<slug>/smoke-e2e-output.log
```

## FAIL Conditions

- Binary doesn't build → fix compilation errors
- Server doesn't start → fix startup errors, check logs
- Health endpoint not responding within 15s → fix startup timeout or config issues
- Changed endpoint returns wrong status → fix the handler
- Response JSON missing required fields → fix response serialization
- Server panics on any request → fix the panic (critical bug, blocks merge)
- No evidence file → run smoke:e2e and save output

Cross-reference: [docs/HARNESS_GATES.md](../../HARNESS_GATES.md#pre-merge)
