# RRI-T Phase 3: STRUCTURE — basic-commands-server-proxy

**Feature:** basic-commands-server-proxy
**Generated from:** 02-discover.md (2026-05-14)
**Total Test Cases:** 20

## Priority Distribution
| Priority | Count | Description |
|----------|-------|-------------|
| P0 | 6 | Critical — blocks release |
| P1 | 10 | Major — fix before release |
| P2 | 4 | Minor — next sprint |

## Dimension Distribution
| Dimension | Count |
|-----------|-------|
| D2: API | 4 |
| D3: Performance | 1 |
| D4: Security | 3 |
| D5: Data Integrity | 4 |
| D6: Infrastructure | 5 |
| D7: Edge Cases | 3 |

---

## Test Cases

### TC-RRI-PROXY-001
- **Q:** As an AI agent in a container, what happens when I run `npx nano-brain tags` and the host daemon is running?
- **A:** CLI detects container, proxies to `http://host.docker.internal:3100/api/tags`, prints tag list to stdout.
- **R:** REQ-PROXY-001: All CLI commands in container must route through HTTP server.
- **P:** P0
- **T:**
  - **Preconditions:**
    - Running inside Docker container (`/.dockerenv` exists)
    - Host daemon running on port 3100
    - At least 3 documents indexed with tags `["memory", "code", "session"]`
    - `NANO_BRAIN_DIRECT` not set
  - **Steps:**
    1. `cd /workspace && npx nano-brain tags`
    2. Observe stdout output
    3. Verify no SQLite file opened (use `strace -e openat` or check server access log)
  - **Expected Result:**
    - Exit code 0
    - Stdout lists tags with counts (e.g. `memory (12)`, `code (8)`, `session (3)`)
    - Server access log shows `GET /api/tags` with 200
    - No direct SQLite access
  - **Dimension:** D2: API
  - **Source Persona:** End User
- **Result:** ☐ MISSING
- **Notes:** Not yet implemented. Currently opens SQLite directly.

---

### TC-RRI-PROXY-002
- **Q:** As an AI agent, what happens when I run `npx nano-brain tags` and the host daemon is NOT running?
- **A:** CLI should fail fast with a clear, actionable error message. Must NOT silently fall back to direct SQLite in container mode.
- **R:** REQ-PROXY-002: Container mode must not fall back to local SQLite; fail loudly.
- **P:** P0
- **T:**
  - **Preconditions:**
    - Running inside Docker container
    - Host daemon NOT running (port 3100 closed)
    - `NANO_BRAIN_DIRECT` not set
  - **Steps:**
    1. Confirm `curl -sf http://host.docker.internal:3100/health` fails
    2. Run `npx nano-brain tags`
    3. Observe stderr and exit code
  - **Expected Result:**
    - Exit code non-zero (1 or 2)
    - Stderr: `Error: nano-brain server not reachable at host.docker.internal:3100. Run: nano-brain docker start`
    - NO tag output printed
    - No SQLite opened directly
  - **Dimension:** D6: Infrastructure
  - **Source Persona:** End User
- **Result:** ☐ MISSING
- **Notes:** Currently falls through to direct SQLite which appears to "work" but violates architecture.

---

### TC-RRI-PROXY-003
- **Q:** As an AI agent, what happens when I run `npx nano-brain update` inside the container to trigger reindex?
- **A:** CLI proxies `POST /api/update` with workspace context to the server; server triggers reindex for that workspace.
- **R:** REQ-PROXY-003: `update` command must proxy to server in container mode.
- **P:** P0
- **T:**
  - **Preconditions:**
    - Running inside Docker container
    - Host daemon running on port 3100
    - Workspace `/workspace/myproject` has indexed documents
  - **Steps:**
    1. `cd /workspace/myproject && npx nano-brain update`
    2. Observe stdout and server logs
    3. Verify server logs show reindex job triggered for correct workspace
  - **Expected Result:**
    - Exit code 0
    - Stdout: confirmation message (e.g. `Reindex triggered for /workspace/myproject`)
    - Server access log: `POST /api/update 200`
    - Server logs: reindex job enqueued for `/workspace/myproject`
  - **Dimension:** D2: API
  - **Source Persona:** End User
- **Result:** ☐ MISSING
- **Notes:** Not yet implemented.

---

### TC-RRI-PROXY-004
- **Q:** As a Business Analyst, does the `tags` proxy return exactly the same data as direct SQLite access did?
- **A:** Tag names and counts must be identical. Order may differ but all tags present.
- **R:** REQ-PROXY-004: Proxy must not alter data shape or content vs direct access.
- **P:** P0
- **T:**
  - **Preconditions:**
    - SQLite DB with 5 known tags: `["memory:5", "code:3", "session:8", "debug:1", "daily:12"]`
    - Server running and pointing to same DB
  - **Steps:**
    1. Call `GET /api/tags` directly via curl, record response
    2. Run `npx nano-brain tags` on host (non-container, direct SQLite path), record output
    3. Compare tag names and counts
  - **Expected Result:**
    - All 5 tags present in both outputs
    - Counts identical
    - No extra tags from other workspaces
  - **Dimension:** D5: Data Integrity
  - **Source Persona:** Business Analyst
- **Result:** ☐ MISSING
- **Notes:** Requires both endpoint and CLI conversion to exist.

---

### TC-RRI-PROXY-005
- **Q:** As a Business Analyst, does `status` after the fix return complete information without local DB fallback?
- **A:** `status` must return all health data from server only. Removing local DB reads must not drop any fields agents depend on.
- **R:** REQ-PROXY-005: `status` output must be backward-compatible after removing local DB fallback.
- **P:** P0
- **T:**
  - **Preconditions:**
    - Baseline: record `npx nano-brain status` output on host (pre-fix)
    - Server running with same data
  - **Steps:**
    1. Record baseline `status` output fields (pre-fix)
    2. Apply fix (remove local DB fallback from `status.ts`)
    3. Run `npx nano-brain status` (container mode, via proxy)
    4. Diff the output fields
  - **Expected Result:**
    - All fields present in baseline are also present in proxy output
    - No fields missing or renamed
    - Values reflect server-side state (may differ from stale local DB reads)
  - **Dimension:** D5: Data Integrity
  - **Source Persona:** Business Analyst
- **Result:** ☐ MISSING
- **Notes:** Must audit what fields `status.ts` reads from local DB vs what `/api/status` returns.

---

### TC-RRI-PROXY-006
- **Q:** As an AI agent, what happens when I run `npx nano-brain tags` OUTSIDE a container (local dev)?
- **A:** Command must behave exactly as before — direct SQLite, no proxy. The fix must not break non-container usage.
- **R:** REQ-PROXY-006: Non-container mode must be unchanged.
- **P:** P0
- **T:**
  - **Preconditions:**
    - Running on macOS host (no `/.dockerenv`)
    - Local SQLite DB with known tags
    - `NANO_BRAIN_DIRECT` not set
  - **Steps:**
    1. Confirm `isInsideContainer()` returns false (check `node -e "..."`)
    2. Run `npx nano-brain tags`
    3. Verify output matches direct DB query
  - **Expected Result:**
    - Exit code 0
    - Tags listed from local SQLite directly
    - No HTTP request made to port 3100
  - **Dimension:** D6: Infrastructure
  - **Source Persona:** End User
- **Result:** ☐ MISSING
- **Notes:** Regression test — must not break existing local workflow.

---

### TC-RRI-PROXY-007
- **Q:** As a QA Destroyer, what happens when `GET /api/tags` returns HTTP 500?
- **A:** CLI must exit with non-zero code and show a clear error. Must not crash with unhandled exception.
- **R:** REQ-PROXY-007: HTTP error responses must be handled gracefully.
- **P:** P1
- **T:**
  - **Preconditions:**
    - Mock server responding with `HTTP 500` for `GET /api/tags`
    - CLI in container mode pointing at mock server
  - **Steps:**
    1. Start mock server on port 3100 that returns `500 Internal Server Error`
    2. Run `npx nano-brain tags` in container mode
    3. Check exit code and stderr
  - **Expected Result:**
    - Exit code 1
    - Stderr: `Error: Server returned 500 for /api/tags`
    - No stack trace exposed to user
    - No SQLite fallback attempted
  - **Dimension:** D7: Edge Cases
  - **Source Persona:** QA Destroyer
- **Result:** ☐ MISSING

---

### TC-RRI-PROXY-008
- **Q:** As a QA Destroyer, what happens when the server returns malformed JSON for `tags`?
- **A:** CLI must catch JSON parse error and exit cleanly, not crash with unhandled exception.
- **R:** REQ-PROXY-007: HTTP error responses must be handled gracefully.
- **P:** P1
- **T:**
  - **Preconditions:**
    - Mock server returning `HTTP 200` with body `{"tags": [{"name": "memo`  (truncated)
  - **Steps:**
    1. Start mock returning truncated JSON on `GET /api/tags`
    2. `npx nano-brain tags`
    3. Check exit code and stderr
  - **Expected Result:**
    - Exit code 1
    - Stderr: `Error: Invalid response from server (JSON parse error)`
    - Process exits cleanly
  - **Dimension:** D7: Edge Cases
  - **Source Persona:** QA Destroyer
- **Result:** ☐ MISSING

---

### TC-RRI-PROXY-009
- **Q:** As a QA Destroyer, what happens when the HTTP request to `tags` times out (server hangs)?
- **A:** CLI must time out after a bounded period and exit with a clear message. Must not hang indefinitely.
- **R:** REQ-PROXY-008: Proxy requests must have a timeout (max 30s, consistent with existing proxyPost pattern).
- **P:** P1
- **T:**
  - **Preconditions:**
    - Mock server that accepts TCP connection but never sends response
  - **Steps:**
    1. Start hanging mock server on port 3100
    2. `npx nano-brain tags`
    3. Measure time to exit
  - **Expected Result:**
    - CLI exits within 30 seconds (matching existing proxy timeout)
    - Stderr: `Error: Request timed out — server at host.docker.internal:3100 did not respond`
    - Exit code 1
  - **Dimension:** D3: Performance
  - **Source Persona:** QA Destroyer
- **Result:** ☐ MISSING

---

### TC-RRI-PROXY-010
- **Q:** As a DevOps Tester, what happens when `nano-brain docker start` just ran and `tags` is called immediately?
- **A:** Server may still be warming up. CLI should give a retry or a clear "server starting" message, not a hard error.
- **R:** REQ-PROXY-009: CLI must handle server warm-up gracefully.
- **P:** P1
- **T:**
  - **Preconditions:**
    - Host daemon just started (within 3 seconds)
    - `/health` endpoint may not be ready yet
  - **Steps:**
    1. `nano-brain docker start` on host
    2. Immediately (< 3s) run `npx nano-brain tags` from container
    3. Observe behaviour
  - **Expected Result:**
    - Either: waits briefly and retries (1-2 retries with 1s backoff)
    - Or: clear message `Server is starting — retry in a moment`
    - Does NOT silently fall back to SQLite
  - **Dimension:** D6: Infrastructure
  - **Source Persona:** DevOps Tester
- **Result:** ☐ MISSING

---

### TC-RRI-PROXY-011
- **Q:** As a Security Auditor, does `GET /api/tags` return only tags for the requesting workspace?
- **A:** Server must scope tag results to the workspace derived from the request context. Tags from other workspaces must not leak.
- **R:** REQ-SEC-001: Workspace isolation must be maintained across HTTP proxy boundary.
- **P:** P1
- **T:**
  - **Preconditions:**
    - Server has two workspaces: `projectA` (tags: `["alpha", "beta"]`) and `projectB` (tags: `["gamma", "delta"]`)
    - Request is made in context of `projectA`
  - **Steps:**
    1. `GET /api/tags` with workspace header/param set to `projectA`
    2. Inspect response
  - **Expected Result:**
    - Response contains only `["alpha", "beta"]`
    - `gamma` and `delta` not present
    - No 500 error if workspace has no tags (returns empty array)
  - **Dimension:** D4: Security
  - **Source Persona:** Security Auditor
- **Result:** ☐ MISSING

---

### TC-RRI-PROXY-012
- **Q:** As a Security Auditor, does `POST /api/update` validate the workspace path to prevent path traversal?
- **A:** Server must reject workspace paths containing `..` or absolute paths outside allowed roots.
- **R:** REQ-SEC-002: Server must validate workspace paths in all write/trigger endpoints.
- **P:** P1
- **T:**
  - **Preconditions:**
    - Server running
  - **Steps:**
    1. `POST /api/update` with body `{"workspace": "../../etc/passwd"}`
    2. `POST /api/update` with body `{"workspace": "/root/secret"}`
    3. Check server response and logs
  - **Expected Result:**
    - Both requests return HTTP 400
    - No reindex triggered
    - Error: `Invalid workspace path`
    - No path exposed in error response
  - **Dimension:** D4: Security
  - **Source Persona:** Security Auditor
- **Result:** ☐ MISSING

---

### TC-RRI-PROXY-013
- **Q:** As an AI Agent, what happens when `tags` returns an empty list (no tags indexed yet)?
- **A:** CLI should print a friendly message like `(no tags)` or empty output, not an error.
- **R:** REQ-PROXY-010: Empty result sets must not be treated as errors.
- **P:** P1
- **T:**
  - **Preconditions:**
    - Server running, workspace exists but has no tagged documents
    - `GET /api/tags` returns `{"tags": []}`
  - **Steps:**
    1. `npx nano-brain tags`
    2. Check stdout, stderr, exit code
  - **Expected Result:**
    - Exit code 0
    - Stdout: `(no tags)` or empty line (matching current behaviour for empty tag list)
    - No error message
  - **Dimension:** D7: Edge Cases
  - **Source Persona:** End User
- **Result:** ☐ MISSING

---

### TC-RRI-PROXY-014
- **Q:** As an AI Agent, do Vietnamese tag names survive the proxy round-trip intact?
- **A:** Tags with Vietnamese characters (e.g. `nhớ`, `ghi chú`, `phân tích`) must be preserved.
- **R:** REQ-PROXY-011: UTF-8 data must be preserved through HTTP proxy.
- **P:** P1
- **T:**
  - **Preconditions:**
    - Document indexed with tags: `["nhớ", "ghi chú", "phân tích-2026"]`
    - Server running
  - **Steps:**
    1. `GET /api/tags` via curl, check raw JSON
    2. `npx nano-brain tags`, check stdout
  - **Expected Result:**
    - Both show `nhớ`, `ghi chú`, `phân tích-2026` correctly
    - No mojibake or escaped unicode
    - `Content-Type: application/json; charset=utf-8` in response headers
  - **Dimension:** D5: Data Integrity
  - **Source Persona:** End User
- **Result:** ☐ MISSING

---

### TC-RRI-PROXY-015
- **Q:** As a Business Analyst, what happens when `update` completes successfully — does the agent get a clear confirmation?
- **A:** CLI must print a confirmation that the reindex was triggered, with enough detail for the agent to know it worked.
- **R:** REQ-PROXY-003: `update` must provide actionable feedback.
- **P:** P1
- **T:**
  - **Preconditions:**
    - Server running, workspace indexed
  - **Steps:**
    1. `npx nano-brain update` in container mode
    2. Check stdout message
  - **Expected Result:**
    - Stdout contains something like `Reindex triggered` or `Update queued for /workspace/...`
    - Exit code 0
    - Format matches or improves on pre-proxy `update` output
  - **Dimension:** D5: Data Integrity
  - **Source Persona:** Business Analyst
- **Result:** ☐ MISSING

---

### TC-RRI-PROXY-016
- **Q:** As a DevOps Tester, what happens when `NANO_BRAIN_DIRECT=1` is set inside the container?
- **A:** `NANO_BRAIN_DIRECT=1` must override container detection and use local SQLite (debugging escape hatch). Must not proxy.
- **R:** REQ-PROXY-012: `NANO_BRAIN_DIRECT=1` escape hatch must still work after the fix.
- **P:** P1
- **T:**
  - **Preconditions:**
    - `/.dockerenv` exists (container mode)
    - `NANO_BRAIN_DIRECT=1` set in environment
    - Local SQLite present at `~/.nano-brain/data/`
  - **Steps:**
    1. `NANO_BRAIN_DIRECT=1 npx nano-brain tags`
    2. Check which path is taken (proxy vs local)
  - **Expected Result:**
    - Local SQLite used (no HTTP request to port 3100)
    - `isInsideContainer()` returns false due to env var
    - Tags read from local DB
  - **Dimension:** D6: Infrastructure
  - **Source Persona:** DevOps Tester
- **Result:** ☐ MISSING

---

### TC-RRI-PROXY-017
- **Q:** As a DevOps Tester, what happens when the CLI and server are on different nano-brain versions?
- **A:** Basic commands (tags, update, status) must work across minor version differences. API shape must be stable.
- **R:** REQ-PROXY-013: `/api/tags` and `/api/update` must be versioned or backward-compatible.
- **P:** P2
- **T:**
  - **Preconditions:**
    - Server running `v2026.8.15`
    - CLI in container is `v2026.8.1-beta.4`
  - **Steps:**
    1. `npx nano-brain@2026.8.1-beta.4 tags` (older CLI vs newer server)
    2. Check if command works
  - **Expected Result:**
    - Works if API shape is unchanged
    - Clear error if API incompatible (not a silent wrong result)
  - **Dimension:** D6: Infrastructure
  - **Source Persona:** DevOps Tester
- **Result:** ☐ MISSING

---

### TC-RRI-PROXY-018
- **Q:** As a Security Auditor, does the server error response for `tags` expose internal file paths or stack traces?
- **A:** Error responses must only contain safe, user-facing messages. Internal paths must be stripped.
- **R:** REQ-SEC-003: Error responses must not leak internal implementation details.
- **P:** P2
- **T:**
  - **Preconditions:**
    - Trigger a server error by corrupting the DB temporarily
    - `GET /api/tags`
  - **Steps:**
    1. Force server error on `/api/tags`
    2. Inspect full HTTP response body
  - **Expected Result:**
    - Response body: `{"error": "Internal server error"}` or similar
    - No stack trace, no file paths like `/home/user/.nano-brain/data/...`
    - HTTP 500 with safe body
  - **Dimension:** D4: Security
  - **Source Persona:** Security Auditor
- **Result:** ☐ MISSING

---

### TC-RRI-PROXY-019
- **Q:** As a Security Auditor, can `POST /api/update` be used to trigger DoS via rapid repeated calls?
- **A:** Server must deduplicate or rate-limit concurrent reindex calls for the same workspace.
- **R:** REQ-SEC-004: Trigger endpoints must be idempotent or rate-limited.
- **P:** P2
- **T:**
  - **Preconditions:**
    - Server running with workspace indexed
  - **Steps:**
    1. Send 50 `POST /api/update` requests in 1 second for the same workspace
    2. Monitor server CPU and job queue
  - **Expected Result:**
    - Server does not start 50 parallel reindex jobs
    - Jobs deduplicated or queued (max 1 active per workspace)
    - Responses: first request 200, subsequent requests 202 (already queued) or 429
  - **Dimension:** D4: Security
  - **Source Persona:** Security Auditor
- **Result:** ☐ MISSING

---

### TC-RRI-PROXY-020
- **Q:** As a DevOps Tester, what happens when `/.dockerenv` exists but `host.docker.internal` DNS resolution fails?
- **A:** CLI must give a specific DNS failure error, not a generic "connection refused" that's hard to debug.
- **R:** REQ-PROXY-002: Errors must be actionable.
- **P:** P2
- **T:**
  - **Preconditions:**
    - `/.dockerenv` exists (container detected)
    - `host.docker.internal` not resolvable (misconfigured network)
  - **Steps:**
    1. Block DNS for `host.docker.internal` (`/etc/hosts` override)
    2. `npx nano-brain tags`
    3. Check error message
  - **Expected Result:**
    - Stderr: `Error: Cannot resolve host.docker.internal — check Docker network configuration`
    - Not: `ECONNREFUSED` (generic and unhelpful)
    - Exit code 1
  - **Dimension:** D6: Infrastructure
  - **Source Persona:** DevOps Tester
- **Result:** ☐ MISSING
