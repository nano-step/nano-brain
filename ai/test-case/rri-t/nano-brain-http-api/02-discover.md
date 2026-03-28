# RRI-T Phase 2: DISCOVER — nano-brain HTTP API

**Feature:** nano-brain HTTP API
**Date:** 2026-03-27
**Personas Interviewed:** 5

---

## Persona 1: End User (AI Coding Agent)

*The primary consumer is an AI agent (Claude, GPT) accessing nano-brain via HTTP API for memory operations during coding sessions.*

### Interview

**Q1:** What do I expect when I query my memory for a topic I've worked on before?
**A1:** POST `/api/query` with `{"query":"stripe"}` should return relevant past memories ranked by relevance. Results should include path, title, snippet, and score. Response within 5 seconds.

**Q2:** What happens when I write a new memory entry?
**A2:** POST `/api/write` with content and tags should persist immediately. The response should confirm the file path written. Subsequent queries should find this new content.

**Q3:** What if the server is slow or unresponsive?
**A3:** I expect a timeout response (not infinite hang). Health endpoint should always be fast (<1s). If a query times out, I should get a clear error so I can retry.

**Q4:** Can I search for exact terms vs semantic concepts?
**A4:** `/api/search` for exact keyword matching (BM25). `/api/query` for hybrid search. Both should work independently.

**Q5:** How do I check if the server is ready before sending queries?
**A5:** GET `/health` should return quickly with `ready: true/false`. GET `/api/status` for detailed model availability.

### Scenarios Generated
- S1.1: Happy path query with known topic → returns results
- S1.2: Query with unknown topic → returns empty results (not error)
- S1.3: Write then immediately query → new content findable
- S1.4: Health check responsiveness under load
- S1.5: Status endpoint shows accurate model availability

---

## Persona 2: Business Analyst

*Concerned with API contract correctness, response schemas, and documentation accuracy.*

### Interview

**Q1:** Are all response shapes consistent and documented?
**A1:** Every endpoint should return valid JSON. Error responses should always include `{ "error": "message" }`. Success responses should have predictable structures.

**Q2:** What HTTP status codes are used and when?
**A2:** 200 for success, 400 for bad input, 404 for not found, 409 for conflict, 503 for maintenance, 504 for timeout. Each should be correct for its situation.

**Q3:** Are required vs optional parameters clearly enforced?
**A3:** Missing required params (e.g., `query` for `/api/query`) should return 400 with specific error message, not 500.

**Q4:** Do the REST v1 endpoints follow REST conventions?
**A4:** GET endpoints should accept query params. Responses should include counts and metadata. Pagination should be supported where applicable.

**Q5:** Is there API versioning?
**A5:** `/api/v1/*` suggests versioning. Unversioned `/api/*` endpoints should also remain stable.

### Scenarios Generated
- S2.1: All endpoints return valid JSON Content-Type
- S2.2: Error responses have consistent `{ "error": "..." }` format
- S2.3: Required param validation returns 400 (not 500)
- S2.4: Optional params use sensible defaults
- S2.5: Response schemas match documented structure

---

## Persona 3: QA Destroyer

*Tries to break every endpoint with malformed input, edge cases, and abuse patterns.*

### Interview

**Q1:** What happens with completely invalid JSON in POST body?
**A1:** Should return 400 with "Invalid JSON body", never 500 or crash.

**Q2:** What about an empty POST body?
**A2:** Should return 400 with appropriate error. Should NOT hang or crash.

**Q3:** What if I send a 10MB POST body?
**A3:** Server should handle gracefully — either reject with 413 or process with degraded performance. Should NOT OOM.

**Q4:** What about Unicode, emoji, null bytes in query strings?
**A4:** Should handle correctly. Unicode queries should search properly. Null bytes should not cause SQLite injection.

**Q5:** What if I send 100 concurrent requests?
**A5:** Server should handle without crashing. Some requests may be slow but none should get no response.

**Q6:** What about SQL injection in search queries?
**A6:** SQLite FTS queries should be parameterized. Special characters like `'`, `"`, `;`, `--` should be safe.

**Q7:** What if I hit endpoints that don't exist?
**A7:** Should return 404 or appropriate error, not crash or expose stack traces.

### Scenarios Generated
- S3.1: Invalid JSON body (missing braces, single quotes, trailing commas)
- S3.2: Empty POST body
- S3.3: Very large POST body (1MB+)
- S3.4: Unicode/emoji in query strings
- S3.5: Null bytes in query
- S3.6: SQL injection patterns in search query
- S3.7: 10 concurrent query requests
- S3.8: Non-existent endpoints return proper error
- S3.9: Wrong HTTP method (GET on POST endpoint, POST on GET)
- S3.10: Extremely long query string (10,000+ chars)
- S3.11: Query with only whitespace
- S3.12: Negative `limit` values
- S3.13: Non-numeric `limit` values
- S3.14: Special FTS characters (`*`, `OR`, `AND`, `NOT`, `NEAR`)

---

## Persona 4: DevOps Tester

*Concerned with deployment, monitoring, graceful degradation, and infrastructure resilience.*

### Interview

**Q1:** Does the health check accurately reflect server readiness?
**A1:** `/health` should return `ready: false` during startup/maintenance. Should always respond fast (<100ms).

**Q2:** How does maintenance mode work?
**A2:** After `/api/maintenance/prepare`, all endpoints except `/health` and `/api/maintenance/resume` should return 503. After `/api/maintenance/resume`, everything resumes.

**Q3:** What happens when the embedding provider is down?
**A3:** Server should degrade gracefully — FTS still works, vector search returns empty. Status should show `embedding: "missing"`.

**Q4:** Can I monitor the server via status endpoints?
**A4:** `/api/status` should show document count, collection sizes, model availability, and database size for monitoring dashboards.

**Q5:** What about Docker container restart?
**A5:** Server should start up correctly, health should return `ready: true` after initialization, existing data should persist.

**Q6:** Is there request logging?
**A6:** `docker logs` should show request activity. Errors should be logged with stack traces.

### Scenarios Generated
- S4.1: Health endpoint always fast (<100ms)
- S4.2: Maintenance mode blocks all non-exempt endpoints
- S4.3: Maintenance mode auto-resumes after timeout
- S4.4: Double maintenance prepare returns 409
- S4.5: Resume without prepare returns 400
- S4.6: Graceful degradation with missing embedding provider
- S4.7: Status shows accurate document/collection counts
- S4.8: CORS preflight returns 204 on allowed origins

---

## Persona 5: Security Auditor

*Looks for injection, information leakage, path traversal, and authentication gaps.*

### Interview

**Q1:** Is there authentication on any endpoint?
**A1:** Currently no auth on HTTP endpoints. MCP sessions use sessionId but no auth tokens. This is by design (local-only server).

**Q2:** Can path traversal access arbitrary files via `/web/*`?
**A2:** Should be blocked — `../` patterns should return 403.

**Q3:** Are error messages safe (no stack traces, no internal paths)?
**A3:** Error responses should be generic. No stack traces in HTTP responses. Internal file paths may leak in status/results (acceptable for local server).

**Q4:** Is the CORS policy restrictive enough?
**A4:** Only `localhost` and `127.0.0.1` origins allowed. No wildcard `*`.

**Q5:** Can I inject commands via query parameters?
**A5:** All search queries should be parameterized. No shell execution from user input.

**Q6:** What about SSE session hijacking?
**A6:** SSE sessions use random IDs. Guessing a sessionId should be computationally infeasible.

### Scenarios Generated
- S5.1: Path traversal on `/web/*` returns 403
- S5.2: No stack traces in error responses
- S5.3: CORS blocks non-localhost origins
- S5.4: SQL/FTS injection attempts are safe
- S5.5: Invalid SSE sessionId returns 404 (not error)
- S5.6: No command injection via search queries
- S5.7: Response headers don't leak server internals
