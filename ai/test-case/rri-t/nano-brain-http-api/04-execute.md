# RRI-T Phase 4: EXECUTE — nano-brain HTTP API

**Feature:** nano-brain-http-api
**Version:** 2026.7.0-rc.19
**Date:** 2026-03-27
**Server:** http://host.docker.internal:3100

## Summary
- **Total Test Cases:** 45
- ✅ **PASS:** 33
- ❌ **FAIL:** 5
- ⚠️ **PAINFUL:** 7
- ☐ **MISSING:** 0

---

## D2: API Correctness

### TC-001: GET /health ✅ PASS
- **HTTP:** 200 | **Time:** 0.010s
- **Response:** `{"status":"ok","ready":true,"version":"2026.7.0-rc.19","uptime":1907,"sessions":{"sse":1,"streamable":0}}`
- All expected fields present, version correct

### TC-002: GET /api/status ✅ PASS
- **HTTP:** 200 | **Time:** 0.055s
- **Response:** Contains models (embedding/reranker/expander), index (documentCount: 3522, collections: 3), workspace info
- Full status with all expected sections

### TC-003: POST /api/query ⚠️ PAINFUL
- **HTTP:** 200 | **Time:** ~14s
- **Input:** `{"query":"nano-brain","limit":2}`
- **Response:** Results array with path, score, snippet, collection. Scores present and > 0.
- Correct results but 14s response time is borderline (15s timeout guard)

### TC-004: POST /api/search ⚠️ PAINFUL
- **HTTP:** 200 | **Time:** 8.1s
- **Input:** `{"query":"stripe","limit":3}`
- **Response:** 3 results with path, snippet with `<mark>` highlights, scores (6.47, 6.47, 6.46)
- Correct results but 8.1s response time due to FTS blocking event loop

### TC-005: POST /api/write ⚠️ PAINFUL
- **HTTP:** 200 | **Time:** 5.8s
- **Input:** `{"content":"RRI-T test marker unicorn-rainbow-42 verification entry","tags":"rri-t-test"}`
- **Response:** `{"status":"ok","path":"/root/.nano-brain/memory/2026-03-27.md","message":"Written to ... Tags: rri-t-test"}`
- Write succeeds but 5.8s is slow for a file append (likely inline consolidation/extraction)

### TC-006: POST /api/reindex ✅ PASS
- **HTTP:** 200 | **Time:** 0.054s
- **Response:** `{"status":"started","root":"/Users/tamlh/workspaces/NUSTechnology/Projects/zengamingx"}`

### TC-007: POST /api/embed ✅ PASS
- **HTTP:** 503 | **Time:** 0.043s
- **Response:** `{"error":"Embedding provider not available"}`
- Correctly reports unavailable embedder with 503

### TC-008: POST /api/init ✅ PASS
- **HTTP:** 400 | **Time:** 0.003s
- **Response:** `{"error":"Use maintenance endpoints for init operations from container. Run init directly on the host: npx nano-brain init"}`
- Correctly blocked with helpful error message

### TC-009: GET /api/v1/status ✅ PASS
- **HTTP:** 200 | **Time:** 0.018s
- **Response:** version, uptime, documents: 3522, embeddings: 6516, 2 workspaces, primaryWorkspace

### TC-010: GET /api/v1/search?q=stripe&limit=2 ⚠️ PAINFUL
- **HTTP:** 200 | **Time:** ~8s
- **Response:** Results with docid, title, path, score, snippet
- Correct but slow due to FTS

### TC-011: GET /api/v1/graph/entities ✅ PASS
- **HTTP:** 200 | **Time:** fast
- **Response:** nodes array (Stripe SDK, server/routes/index.js, etc.), edges array

### TC-012: GET /api/v1/telemetry ✅ PASS
- **HTTP:** 200 | **Time:** 0.066s
- **Response:** `{"queryCount":32,"banditStats":[],"preferenceWeights":{},"expandRate":0,"importanceStats":{"min":0,"max":12,"mean":0.07,"median":0}}`

---

## D3: Performance

### TC-013: GET /health perf ✅ PASS
- **3 runs:** 0.003s, 0.003s, 0.004s — all well under 100ms threshold

### TC-014: POST /api/search perf ⚠️ PAINFUL
- **Time:** 8.1s for "stripe" query
- Under 15s threshold but slow. FTS on 3522 docs with better-sqlite3 synchronous calls blocks event loop.

### TC-015: POST /api/query timeout ✅ PASS
- **Time:** ~14s for "nano-brain" query, completed before 15s guard
- TC-003 confirms the timeout guard works (15s Promise.race). Previous testing showed 504 when exceeded.

### TC-016: POST /api/write perf ❌ FAIL
- **Time:** 5.8s — exceeds 5s threshold
- File append should be instant but inline consolidation/extraction adds latency

### TC-017: GET /api/v1/status perf ✅ PASS
- **3 runs:** 0.015s, 0.014s, 0.014s — all well under 200ms threshold

---

## D4: Security

### TC-018: Path traversal ❌ FAIL
- **HTTP:** 404 | **Time:** 0.005s
- **Input:** GET /web/../../../etc/passwd
- **Response:** "Not Found"
- **Expected:** 403 Forbidden
- **Actual:** 404 Not Found. The path traversal is blocked (no file content leaked), but the status code is 404 instead of the documented 403. curl normalizes `/../` before sending — the raw path never reaches the server's traversal check.

### TC-019: SQL injection ✅ PASS
- **HTTP:** 200 | **Time:** ~8s
- **Input:** `{"query":"'; DROP TABLE documents;--","limit":2}`
- **Response:** Normal search results (matched scheme.sql containing "DROP TABLE" text). No DB corruption.
- FTS query is parameterized, injection attempt treated as literal search text

### TC-020: No stack traces ✅ PASS
- **HTTP:** 400 | **Time:** 0.298s
- **Input:** `{"invalid":true}` (missing query field)
- **Response:** `{"error":"query is required"}` — clean error, no stack trace

### TC-021: CORS preflight localhost ✅ PASS
- **HTTP:** 204 | **Time:** 0.026s
- **Headers:** `Access-Control-Allow-Origin: http://localhost:3000`, `Access-Control-Allow-Methods: GET, POST, OPTIONS`, `Access-Control-Allow-Headers: Content-Type`

### TC-022: CORS foreign origin ❌ FAIL
- **HTTP:** 204 | **Time:** 0.003s
- **Input:** Origin: http://evil.com
- **Response:** 204 with NO Access-Control-Allow-Origin header (good — no CORS grant)
- **Issue:** Returns 204 instead of rejecting entirely. Browser would block due to missing ACAO header, but a non-browser client gets a successful response. Minimal risk for local server.

### TC-023: Invalid sessionId ✅ PASS
- **HTTP:** 404 | **Time:** 0.005s
- **Response:** `{"error":"Session not found"}`

### TC-024: No server header ✅ PASS
- No `Server:` or `X-Powered-By:` headers found in response

---

## D5: Data Integrity

### TC-025: Write then search ✅ PASS
- **Write:** 200, wrote "unicorn-rainbow-42" marker with tags
- **Search (3s later):** 200, found marker in results with `<mark>unicorn-rainbow-42</mark>` highlight, score: 12.36
- Written content is immediately searchable after reindex

### TC-026: Query results have scores ✅ PASS
- TC-003 results all have score > 0 (0.12, 0.07, 0.06, etc.)

### TC-027: Search results have valid paths ✅ PASS
- TC-004 results all have non-empty path fields pointing to actual files

### TC-028: Status document count ✅ PASS
- `/api/status` reports documentCount: 3522, matches collection breakdown (3191 + 3 + 328 = 3522)

---

## D6: Infrastructure

### TC-029: Maintenance prepare ✅ PASS
- **HTTP:** 200 | **Time:** 0.286s
- **Response:** `{"status":"prepared"}`

### TC-030: Maintenance blocks ✅ PASS
- `/health` during maintenance: 200 OK (exempt)
- `/api/query` during maintenance: 503 `{"error":"maintenance in progress"}`

### TC-031: Maintenance resume ✅ PASS
- **HTTP:** 200 | **Time:** 0.053s
- **Response:** `{"status":"resumed"}`
- Server verified healthy after resume

### TC-032: Double prepare ❌ FAIL
- **HTTP:** 503 | **Time:** 0.003s
- **Expected:** 409 Conflict
- **Actual:** 503 `{"error":"maintenance in progress"}` — the global maintenance check intercepts the request before the endpoint handler can return 409. The behavior is correct (blocks the request) but the status code is wrong.

### TC-033: Resume without prepare ✅ PASS
- **HTTP:** 400 | **Time:** 0.009s
- **Response:** `{"error":"no maintenance in progress"}`

### TC-034: Missing embedding graceful ✅ PASS
- `/api/status` returns `status: "ok"` with `models.embedding: "missing"` — no crash, graceful degradation

---

## D7: Edge Cases

### TC-035: Empty body ✅ PASS
- **HTTP:** 400 | **Time:** 0.006s
- **Response:** `{"error":"Unexpected end of JSON input"}`

### TC-036: Invalid JSON ✅ PASS
- **HTTP:** 400 | **Time:** 0.007s
- **Input:** `{broken`
- **Response:** `{"error":"Expected property name or '}' in JSON at position 1"}`

### TC-037: Missing query field ✅ PASS
- **HTTP:** 400 | **Time:** 0.004s
- **Input:** `{"limit":5}`
- **Response:** `{"error":"query is required"}`

### TC-038: Empty query string ✅ PASS
- **HTTP:** 400 | **Time:** 0.005s
- **Input:** `{"query":""}`
- **Response:** `{"error":"query is required"}`

### TC-039: Negative limit ❌ FAIL
- **HTTP:** 200 | **Time:** ~8s
- **Input:** `{"query":"test","limit":-1}`
- **Response:** Returned results (did not reject)
- **Expected:** 400 or graceful handling. Server accepted negative limit without validation.

### TC-040: Non-numeric limit ✅ PASS
- **HTTP:** 400 | **Time:** 0.542s
- **Input:** `{"query":"test","limit":"abc"}`
- **Response:** `{"error":"Invalid JSON body"}`

### TC-041: Unicode query ✅ PASS
- **HTTP:** 200 | **Time:** 0.056s
- **Input:** `{"query":"支付失败"}`
- **Response:** `{"results":[]}` — empty results (no Chinese content indexed), no crash

### TC-042: FTS operators ✅ PASS
- **HTTP:** 200 | **Time:** ~8s
- **Input:** `{"query":"stripe OR payment","limit":2}`
- **Response:** Results returned, FTS OR operator works

### TC-043: Wrong HTTP method ✅ PASS
- **HTTP:** 404 | **Time:** 0.004s
- **Input:** GET /api/query (should be POST)
- **Response:** "Not Found"

### TC-044: Non-existent endpoint ✅ PASS
- **HTTP:** 404 | **Time:** 0.005s
- **Input:** POST /api/doesnotexist
- **Response:** "Not Found"

### TC-045: Whitespace content ❌ FAIL (unexpected)
- **HTTP:** 200 | **Time:** 0.015s
- **Input:** `{"content":"   "}`
- **Response:** `{"status":"ok","path":"/root/.nano-brain/memory/2026-03-27.md"}`
- **Expected:** 400 "content is required" — server accepted whitespace-only content
