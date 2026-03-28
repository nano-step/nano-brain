# RRI-T Test Cases — nano-brain HTTP API

**Feature:** nano-brain-http-api
**Generated from:** Persona Interview (2026-03-27)
**Total Test Cases:** 45

## Priority Distribution
| Priority | Count | Description |
|----------|-------|-------------|
| P0 | 12 | Critical — blocks release |
| P1 | 18 | Major — fix before release |
| P2 | 15 | Minor — next sprint |
| P3 | 0 | Trivial — backlog |

## Dimension Distribution
| Dimension | Count | Target Coverage |
|-----------|-------|----------------|
| D1: UI/UX | 0 | N/A (HTTP API) |
| D2: API | 12 | >= 85% |
| D3: Performance | 5 | >= 70% |
| D4: Security | 7 | >= 85% |
| D5: Data Integrity | 4 | >= 85% |
| D6: Infrastructure | 6 | >= 70% |
| D7: Edge Cases | 11 | >= 85% |

---

## D2: API Correctness (12 test cases)

| TC | P | Endpoint | Question | Expected | Persona |
|----|---|----------|----------|----------|---------|
| 001 | P0 | GET /health | Does health check return server state? | 200 with version, ready, uptime, sessions | DevOps |
| 002 | P0 | GET /api/status | Does status show full index health? | 200 with models, index, workspace, collections | DevOps |
| 003 | P0 | POST /api/query | Does hybrid search return ranked results? | 200 with results array, each having path, score, snippet | End User |
| 004 | P0 | POST /api/search | Does BM25 search return keyword matches? | 200 with results array, highlighted snippets | End User |
| 005 | P0 | POST /api/write | Does write persist content to daily log? | 200 with status, path, message confirming write | End User |
| 006 | P1 | POST /api/reindex | Does reindex trigger background scan? | 200 with status "started" and root path | DevOps |
| 007 | P1 | POST /api/embed | Does embed trigger or report unavailable? | 200 with "started" OR 503 if no embedder | DevOps |
| 008 | P1 | POST /api/init | Is init correctly blocked in container? | 400 with error explaining to use host | Security |
| 009 | P1 | GET /api/v1/status | Does REST v1 status return workspace list? | 200 with version, documents, workspaces array | BA |
| 010 | P1 | GET /api/v1/search | Does REST search with query params work? | 200 with results and executionMs | End User |
| 011 | P2 | GET /api/v1/graph/entities | Does entity graph return nodes/edges? | 200 with nodes, edges, stats | BA |
| 012 | P2 | GET /api/v1/telemetry | Does telemetry return learning stats? | 200 with queryCount, banditStats, preferenceWeights | BA |

---

## D3: Performance (5 test cases)

| TC | P | Test | Threshold | Persona |
|----|---|------|-----------|---------|
| 013 | P0 | GET /health response time | < 100ms (3 runs averaged) | DevOps |
| 014 | P1 | POST /api/search response time | < 15s for common query | End User |
| 015 | P0 | POST /api/query timeout guard | 504 if search exceeds 15s, not infinite hang | End User |
| 016 | P1 | POST /api/write response time | < 5s for typical write | End User |
| 017 | P2 | GET /api/v1/status response time | < 200ms | DevOps |

---

## D4: Security (7 test cases)

| TC | P | Attack Vector | Expected Defense | Persona |
|----|---|---------------|------------------|---------|
| 018 | P0 | GET /web/../../../etc/passwd | 403 Forbidden (path traversal blocked) | Security |
| 019 | P0 | POST /api/search with `'; DROP TABLE--` | Safe response, no DB damage | QA Destroyer |
| 020 | P1 | Trigger error, check for stack traces | Error response contains only message, no trace | Security |
| 021 | P1 | OPTIONS /api/v1/search from localhost | 204 with correct CORS headers | Security |
| 022 | P1 | OPTIONS /api/v1/search from evil.com | No Access-Control-Allow-Origin header | Security |
| 023 | P2 | POST /messages with fake sessionId | 404 "Session not found" | Security |
| 024 | P2 | Check response headers for Server/X-Powered-By | No server software disclosure | Security |

---

## D5: Data Integrity (4 test cases)

| TC | P | Test | Expected | Persona |
|----|---|------|----------|---------|
| 025 | P0 | Write unique content, then search for it | Search returns the written content | End User |
| 026 | P1 | POST /api/query results have scores | All results have score > 0 | BA |
| 027 | P1 | POST /api/search results have valid paths | All path fields are non-empty strings | BA |
| 028 | P2 | GET /api/status documentCount vs reality | Count matches actual indexed documents | DevOps |

---

## D6: Infrastructure (6 test cases)

| TC | P | Test | Expected | Persona |
|----|---|------|----------|---------|
| 029 | P0 | POST /api/maintenance/prepare | 200 "prepared", subsequent requests blocked | DevOps |
| 030 | P0 | During maintenance: /health OK, /api/query 503 | Health exempt, others blocked with 503 | DevOps |
| 031 | P0 | POST /api/maintenance/resume | 200 "resumed", normal operation restored | DevOps |
| 032 | P1 | Double maintenance prepare | 409 Conflict (already in maintenance) | QA Destroyer |
| 033 | P1 | Resume without prior prepare | 400 "no maintenance in progress" | QA Destroyer |
| 034 | P1 | Status with missing embedding provider | status "ok", embedding "missing", no crash | DevOps |

---

## D7: Edge Cases (11 test cases)

| TC | P | Input | Expected | Persona |
|----|---|-------|----------|---------|
| 035 | P1 | POST /api/query with empty body | 400 error | QA Destroyer |
| 036 | P1 | POST /api/query with `{broken` | 400 JSON parse error | QA Destroyer |
| 037 | P1 | POST /api/query with `{"limit":5}` (no query) | 400 "query is required" | QA Destroyer |
| 038 | P1 | POST /api/query with `{"query":""}` | 400 "query is required" | QA Destroyer |
| 039 | P2 | POST /api/search with `limit: -1` | Graceful handling (no crash) | QA Destroyer |
| 040 | P2 | POST /api/search with `limit: "abc"` | 400 or default to 10 | QA Destroyer |
| 041 | P2 | POST /api/search with Unicode `支付失败` | 200 (empty results OK if no match) | QA Destroyer |
| 042 | P2 | POST /api/search with FTS operators `stripe OR payment` | 200 with results | QA Destroyer |
| 043 | P2 | GET /api/query (wrong method for POST endpoint) | 404 or 405 | QA Destroyer |
| 044 | P2 | POST /api/doesnotexist | 404 | QA Destroyer |
| 045 | P2 | POST /api/write with whitespace-only content | 400 "content is required" | QA Destroyer |
