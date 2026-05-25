# Phase 2: DISCOVER u2014 Persona Interviews

## Persona 1: End User (AI Coding Agent)

**Context:** I'm an AI agent starting a new session. I need context briefing to know what workspace I'm in and what's important.

### Scenarios
1. I call `wake-up` and get a readable briefing with workspace name, collections, and key memories
2. I call `wake-up --json` and get structured JSON I can programmatically parse
3. I call `wake-up --workspace=/some/path` to get briefing for a different workspace
4. The briefing fits in my system prompt (~500 tokens / 2000 chars max)
5. Empty workspace gives me a helpful "no memories yet" message, not an error
6. Key memories are ordered by access count so most-used info appears first
7. Recent decisions show dates so I know how fresh they are

---

## Persona 2: Business Analyst

**Context:** I need the wake-up feature to deliver accurate, complete context to AI agents to reduce hallucination and improve session quality.

### Scenarios
1. The briefing includes ALL configured collections (not just some)
2. Topics from workspace profile are surfaced in L0 section
3. Top-accessed documents are genuinely the most-accessed (not random)
4. Decision-tagged documents are filtered correctly by the "decision" tag
5. The 2000-char cap is enforced so it doesn't blow up system prompts
6. Superseded (stale) documents are never shown
7. The MCP tool, CLI, and HTTP API all return consistent results for the same workspace

---

## Persona 3: QA Destroyer

**Context:** I'm going to break this feature. I'll test every edge case, boundary condition, and failure mode.

### Scenarios
1. What happens with 0 documents? (empty store)
2. What happens with 1000+ documents? (performance / truncation)
3. What if all documents are superseded? (should show "no memories")
4. What if `loadCollectionConfig()` returns null? (no config file)
5. What if `WorkspaceProfile.loadProfile()` throws? (error handling in L0)
6. What if document title is empty string? (truncateLine edge case)
7. What if document title is null/undefined?
8. What if projectHash is empty string?
9. What if limit is 0? negative? very large (10000)?
10. What if tags array is empty `[]` for getRecentDocumentsByTags?
11. What if the formatted output is exactly 2000 chars? 2001 chars?
12. What if document has no modified_at date? ("unknown" fallback)
13. What if configPath points to non-existent file?
14. What if maxChars option overrides the 2000 default?
15. truncateLine with text exactly at max length (no truncation needed)
16. truncateLine with text 1 char over max (needs "..." suffix)

---

## Persona 4: DevOps Tester

**Context:** I care about the three surfaces (CLI, MCP, HTTP) working correctly, performance, and operational reliability.

### Scenarios
1. CLI `wake-up` command prints briefing to stdout
2. CLI `wake-up --json` outputs valid parseable JSON
3. CLI `wake-up --workspace=<path>` computes correct projectHash
4. MCP tool `memory_wake_up` returns content in correct MCP format
5. MCP tool handles daemon mode workspace requirement
6. HTTP GET `/api/wake-up` returns JSON response
7. HTTP GET `/api/wake-up?json=true` returns full BriefingResult
8. HTTP GET `/api/wake-up?workspace=<path>` overrides workspace
9. HTTP GET `/api/wake-up?limit=5` respects limit parameter
10. HTTP POST `/api/wake-up` with body works identically to GET
11. HTTP error handling returns 500 with error message on failure
12. Performance: generateBriefing completes in <50ms for typical workspace
13. TypeScript compilation passes with no errors related to wake-up
14. Help text includes wake-up command documentation

---

## Persona 5: Security Auditor

**Context:** I'm reviewing for information disclosure, injection, and data integrity issues.

### Scenarios
1. No SQL injection via projectHash parameter (prepared statements)
2. No SQL injection via tags array parameter
3. Path traversal: workspace parameter doesn't expose files outside workspace
4. No sensitive data leakage in error messages
5. Superseded documents are truly excluded (no information leak of stale data)
6. access_count is not user-manipulable via the wake-up surface
7. HTTP endpoint doesn't expose internal file paths in error responses
8. JSON output doesn't include database IDs or internal hashes unnecessarily
9. Character cap prevents denial-of-service via extremely large briefings
10. Prepared statements used for both new Store methods (not string interpolation)
