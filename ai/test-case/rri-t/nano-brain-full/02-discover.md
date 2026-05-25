# RRI-T Phase 2: DISCOVER — nano-brain-full

## Persona 1: End User (AI Developer using nano-brain as MCP)

| # | Question | Dimension |
|---|----------|-----------|
| 1 | Can I search for a memory I wrote yesterday using Vietnamese text with diacritics? | D1/D7 |
| 2 | If I write a memory and immediately search for it, does it appear? | D5 |
| 3 | What happens when I search for something and nano-brain has no data? | D7 |
| 4 | Can I retrieve a document using its #docid from a different workspace? | D2 |
| 5 | If I write a very long document (100KB+), does it get indexed correctly? | D7 |
| 6 | Does memory_query return results in a useful ranked order? | D2 |
| 7 | Can I use memory_tags to find all documents with a specific tag? | D2 |
| 8 | Does memory_expand show the full content including code blocks? | D2 |
| 9 | If I write memories from two different AI sessions simultaneously, do both get saved? | D5/D6 |
| 10 | Does memory_status show accurate counts after bulk operations? | D2 |
| 11 | Can I use memory_multi_get with glob patterns like `*.md`? | D2 |
| 12 | Does the knowledge graph show connections between related memories? | D2 |
| 13 | If I index my codebase, can I find function definitions? | D2 |
| 14 | Does memory_timeline show events in correct chronological order? | D5 |
| 15 | Can I connect two memories and then traverse the connection? | D2 |

## Persona 2: Business Analyst (Team Lead evaluating nano-brain)

| # | Question | Dimension |
|---|----------|-----------|
| 1 | Does nano-brain maintain workspace isolation — team A can't see team B's data? | D4 |
| 2 | Can multiple developers use the same nano-brain instance without data corruption? | D5 |
| 3 | Does the consolidation system reduce redundant memories without losing information? | D5 |
| 4 | Can we measure how much the learning system improves search quality over time? | D3 |
| 5 | Does the importance scoring reflect actual document relevance? | D2 |
| 6 | Can we audit what documents were consolidated or pruned? | D4 |
| 7 | Does the system handle codebase indexing for a large monorepo (10K+ files)? | D3 |
| 8 | Can we track which AI sessions have been harvested? | D2 |
| 9 | Does the proactive suggestions feature actually save developer time? | D2 |
| 10 | Can we configure retention policies per workspace? | D2 |
| 11 | Does symbol impact analysis correctly identify cross-repo dependencies? | D2 |
| 12 | Can we trust the release gate metrics from the testing pipeline? | D5 |
| 13 | Is the system's memory usage acceptable for a Docker container (< 512MB)? | D3 |
| 14 | Can we monitor index health without stopping the service? | D6 |
| 15 | Does code_detect_changes accurately flag affected flows? | D2 |

## Persona 3: QA Destroyer (Breaking nano-brain)

| # | Question | Dimension |
|---|----------|-----------|
| 1 | What happens if I send SQL injection in a search query: `'; DROP TABLE docs; --`? | D4 |
| 2 | What if I send a search query with 10,000 characters? | D7 |
| 3 | What if memory_write receives binary/null bytes in content? | D7 |
| 4 | What happens if Qdrant is down but I try memory_vsearch? | D6 |
| 5 | What if I try to memory_get a document that was just deleted? | D7 |
| 6 | What happens if two processes call memory_write simultaneously to the same file? | D5 |
| 7 | What if the SQLite database file is corrupted mid-operation? | D6 |
| 8 | What happens if I call memory_update during an active memory_write? | D5 |
| 9 | What if I create a circular connection in the knowledge graph? | D7 |
| 10 | What if embedding generation fails for half the documents in a batch? | D6 |
| 11 | What happens if I call memory_consolidate with no documents to consolidate? | D7 |
| 12 | What if the harvest state file is deleted while harvesting? | D6 |
| 13 | What happens if I search with an empty string? | D7 |
| 14 | What if memory_traverse is called with depth=100 on a large graph? | D3/D7 |
| 15 | What happens if the reranker API returns invalid indices? | D6 |
| 16 | What if I index a codebase with symlink loops? | D7 |
| 17 | What if memory_connect is called with non-existent document IDs? | D7 |
| 18 | What if I write UTF-16 or mixed-encoding content? | D7 |
| 19 | What happens if the FTS index gets out of sync with the main table? | D5 |
| 20 | What if parseSearchConfig receives completely invalid JSON? | D7 |

## Persona 4: DevOps Tester (Infrastructure & scaling)

| # | Question | Dimension |
|---|----------|-----------|
| 1 | Does nano-brain survive a Docker container restart without data loss? | D6 |
| 2 | Can multiple containers share the same SQLite volume safely? | D5/D6 |
| 3 | What's the memory footprint after indexing 50K documents? | D3 |
| 4 | Does WAL mode prevent read-write contention in concurrent access? | D3/D5 |
| 5 | Can we scale vector search by switching from sqlite-vec to Qdrant without re-indexing? | D6 |
| 6 | What happens if the disk fills up during a write operation? | D6 |
| 7 | Does the file watcher (chokidar) handle volume mounts correctly in Docker? | D6 |
| 8 | Can we monitor nano-brain health via the REST API? | D6 |
| 9 | Does the SSE transport properly clean up connections on client disconnect? | D6 |
| 10 | What's the startup time for a cold start vs warm start? | D3 |
| 11 | Does the stdio transport work correctly through Docker exec? | D6 |
| 12 | Can we backup and restore the SQLite database while the server is running? | D6 |
| 13 | Does the Thompson Sampling bandit state persist across restarts? | D5 |
| 14 | What happens if Ollama embedding service is unreachable at startup? | D6 |
| 15 | Does the harvest state survive container recreation? | D5/D6 |

## Persona 5: Security Auditor (Security & data safety)

| # | Question | Dimension |
|---|----------|-----------|
| 1 | Are all user inputs parameterized before hitting SQLite? | D4 |
| 2 | Can path traversal in memory_get access files outside the workspace? | D4 |
| 3 | Are API keys (VoyageAI, OpenAI) properly protected in config? | D4 |
| 4 | Does the stdio transport prevent stdout/stderr leakage of sensitive data? | D4 |
| 5 | Can malicious content in a harvested session execute code? | D4 |
| 6 | Are there any command injection vectors in file path handling? | D4 |
| 7 | Does workspace isolation prevent cross-workspace data leakage? | D4 |
| 8 | Are the REST API endpoints protected from SSRF? | D4 |
| 9 | Can the knowledge graph reveal information from other workspaces? | D4 |
| 10 | Are temporary files cleaned up securely after operations? | D4 |
| 11 | Does the reranker send raw document content to VoyageAI API? | D4 |
| 12 | Is the hash function (SHA-256) used correctly for content integrity? | D4 |
| 13 | Are there rate limiting protections on the API endpoints? | D4 |
| 14 | Can a malformed MCP request crash the server? | D4/D6 |
| 15 | Does the consolidation agent send full document content to the LLM? | D4 |

---

## Summary

| Persona | Questions | Dimensions Covered |
|---------|-----------|-------------------|
| End User | 15 | D1, D2, D3, D5, D6, D7 |
| Business Analyst | 15 | D2, D3, D4, D5, D6 |
| QA Destroyer | 20 | D3, D4, D5, D6, D7 |
| DevOps Tester | 15 | D3, D5, D6 |
| Security Auditor | 15 | D4, D6 |
| **Total** | **80** | **All 7 dimensions** |

## Raw Test Ideas Consolidation

| # | Idea | Source | Priority |
|---|------|--------|----------|
| 1 | Vietnamese diacritics in FTS search | End User Q1 | P1 |
| 2 | Write-then-search latency | End User Q2 | P0 |
| 3 | Concurrent multi-container writes | QA Destroyer Q6 / DevOps Q2 | P0 |
| 4 | SQL injection in all search tools | QA Destroyer Q1 / Security Q1 | P0 |
| 5 | Empty/null/binary input handling | QA Destroyer Q3,Q13 | P1 |
| 6 | Large document handling (100KB+) | End User Q5 | P1 |
| 7 | Qdrant failure fallback | QA Destroyer Q4 / DevOps Q14 | P0 |
| 8 | Knowledge graph cycles | QA Destroyer Q9 | P1 |
| 9 | Embedding batch partial failure | QA Destroyer Q10 | P0 |
| 10 | Reranker invalid response | QA Destroyer Q15 | P1 |
| 11 | Container restart data persistence | DevOps Q1 | P0 |
| 12 | WAL concurrent read-write | DevOps Q4 | P0 |
| 13 | Workspace isolation | Security Q7 | P0 |
| 14 | Path traversal in memory_get | Security Q2 | P0 |
| 15 | Stdio transport integrity | Security Q4 / DevOps Q11 | P0 |
| 16 | SSE connection cleanup | DevOps Q9 | P1 |
| 17 | FTS index consistency | QA Destroyer Q19 | P1 |
| 18 | Harvest state atomicity | QA Destroyer Q12 | P1 |
| 19 | Thompson Sampling persistence | DevOps Q13 | P2 |
| 20 | Memory consolidation empty set | QA Destroyer Q11 | P2 |
