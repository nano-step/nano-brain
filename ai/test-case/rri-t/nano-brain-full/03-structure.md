# RRI-T Phase 3: STRUCTURE — nano-brain-full

## Test Cases (Q-A-R-P-T Format)

### D2: API — MCP Tool Contract Validation

---

#### TC-001: memory_search basic FTS query
- **Q:** Does memory_search return relevant documents for a keyword query?
- **A:** Returns ranked list of documents containing the keyword, sorted by BM25 score
- **R:** Search pipeline must return FTS results with proper scoring
- **P:** P0
- **Persona:** End User
- **Preconditions:** Store has indexed documents containing known keywords
- **Steps:** 1) Index 10 docs with known content 2) Call memory_search with a keyword present in 3 docs
- **Expected:** Returns ≥3 results, all contain the search term

#### TC-002: memory_vsearch semantic search
- **Q:** Does memory_vsearch return semantically similar documents?
- **A:** Returns documents ranked by vector similarity score
- **R:** Vector search must use embeddings for semantic matching
- **P:** P1
- **Persona:** End User
- **Preconditions:** Documents have embeddings generated
- **Steps:** 1) Index docs with embeddings 2) Search for semantically related query
- **Expected:** Returns docs sorted by cosine similarity

#### TC-003: memory_query hybrid search pipeline
- **Q:** Does memory_query combine BM25 + vector + reranking correctly?
- **A:** Returns hybrid-ranked results using RRF fusion
- **R:** Hybrid search must fuse multiple signals
- **P:** P0
- **Persona:** End User
- **Preconditions:** Store has docs with both FTS index and embeddings
- **Steps:** 1) Index 20 docs 2) Call memory_query with a query that has both keyword and semantic matches
- **Expected:** Results combine BM25 and vector scores via RRF

#### TC-004: memory_expand compact result expansion
- **Q:** Does memory_expand return full content for a compact search result?
- **A:** Returns the complete document body for the given docid
- **R:** Expand must retrieve cached or stored full content
- **P:** P1
- **Persona:** End User
- **Steps:** 1) Search in compact mode 2) Expand a result by docid
- **Expected:** Full content including code blocks and markdown

#### TC-005: memory_get by path
- **Q:** Can I retrieve a document by its file path?
- **A:** Returns the exact document at the given path
- **R:** Get must resolve path to document
- **P:** P0
- **Persona:** End User
- **Steps:** 1) Write a doc at known path 2) memory_get with that path
- **Expected:** Returns exact content that was written

#### TC-006: memory_get by docid (#hash)
- **Q:** Can I retrieve a document using its #docid?
- **A:** Returns the document matching the hash ID
- **R:** Get must resolve docid format
- **P:** P0
- **Persona:** End User
- **Steps:** 1) Index a doc 2) Get its docid from search 3) memory_get #docid
- **Expected:** Returns same document

#### TC-007: memory_multi_get glob pattern
- **Q:** Does memory_multi_get return all matching documents for a glob?
- **A:** Returns all docs matching the glob pattern
- **R:** Multi-get must support glob patterns
- **P:** P1
- **Persona:** End User
- **Steps:** 1) Index docs at various paths 2) memory_multi_get with `*.md`
- **Expected:** Returns all markdown documents

#### TC-008: memory_write creates daily log entry
- **Q:** Does memory_write save content to the correct daily log?
- **A:** Content is appended to today's log file with timestamp
- **R:** Write must create/append to daily log
- **P:** P0
- **Persona:** End User
- **Steps:** 1) Call memory_write with text 2) Verify file exists 3) Read content
- **Expected:** File exists at expected path, content matches

#### TC-009: memory_tags returns accurate counts
- **Q:** Does memory_tags show correct document counts per tag?
- **A:** Returns tag list with accurate document counts
- **R:** Tags must be maintained during indexing
- **P:** P1
- **Persona:** Business Analyst
- **Steps:** 1) Index docs with known tags 2) Call memory_tags
- **Expected:** Counts match expected values

#### TC-010: memory_status shows index health
- **Q:** Does memory_status report correct index statistics?
- **A:** Returns doc count, embedding count, FTS status, collection info
- **R:** Status must query all subsystems
- **P:** P1
- **Persona:** DevOps Tester
- **Steps:** 1) Index some docs 2) Call memory_status
- **Expected:** Numbers match actual index state

#### TC-011: memory_update triggers reindex
- **Q:** Does memory_update reindex all collections?
- **A:** All collections are rescanned and reindexed
- **R:** Update must trigger full reindex cycle
- **P:** P1
- **Persona:** DevOps Tester
- **Steps:** 1) Modify a file outside nano-brain 2) Call memory_update
- **Expected:** Changed content is now searchable

#### TC-012: memory_index_codebase indexes source files
- **Q:** Does memory_index_codebase scan and index the workspace?
- **A:** Source files are indexed with AST symbols extracted
- **R:** Codebase indexer must parse supported languages
- **P:** P0
- **Persona:** End User
- **Steps:** 1) Set up workspace with .ts/.js files 2) Call memory_index_codebase
- **Expected:** Files are indexed, symbols extractable

#### TC-013: memory_focus returns dependency context
- **Q:** Does memory_focus show imports/exports for a file?
- **A:** Returns dependency graph centered on the target file
- **R:** Focus must use file dependency graph
- **P:** P1
- **Persona:** End User
- **Steps:** 1) Index codebase 2) Call memory_focus on a file with imports
- **Expected:** Shows importers and importees

#### TC-014: memory_symbols cross-repo query
- **Q:** Can memory_symbols find Redis keys, API endpoints, etc.?
- **A:** Returns cross-repo infrastructure symbols
- **R:** Symbol extraction must detect infra patterns
- **P:** P1
- **Persona:** Business Analyst
- **Steps:** 1) Index code with Redis/API patterns 2) Query memory_symbols
- **Expected:** Returns matching infrastructure symbols

#### TC-015: code_context 360° view
- **Q:** Does code_context show callers, callees, cluster, and flows?
- **A:** Returns complete context for a code symbol
- **R:** Code context must combine multiple analysis sources
- **P:** P1
- **Persona:** End User
- **Steps:** 1) Index codebase 2) Call code_context for a known function
- **Expected:** Shows upstream/downstream/cluster info

#### TC-016: code_impact change analysis
- **Q:** Does code_impact correctly identify what breaks when I change a function?
- **A:** Returns risk level and affected downstream code
- **R:** Impact must trace dependency chains
- **P:** P1
- **Persona:** Business Analyst
- **Steps:** 1) Index codebase 2) Call code_impact for a widely-used function
- **Expected:** Lists all dependent functions and risk level

#### TC-017: code_detect_changes from git diff
- **Q:** Does code_detect_changes parse git diff and find affected flows?
- **A:** Returns changed symbols and impacted data flows
- **R:** Must parse unified diff format
- **P:** P1
- **Persona:** End User
- **Steps:** 1) Make a code change 2) Generate diff 3) Call code_detect_changes
- **Expected:** Lists changed functions and affected flows

#### TC-018: memory_consolidate manual trigger
- **Q:** Does memory_consolidate compress redundant memories?
- **A:** Triggers LLM-based consolidation cycle
- **R:** Consolidation must detect and merge redundant content
- **P:** P1
- **Persona:** Business Analyst
- **Steps:** 1) Write similar memories multiple times 2) Trigger consolidation
- **Expected:** Redundant memories are merged

#### TC-019: memory_importance scores
- **Q:** Does memory_importance return meaningful scores?
- **A:** Returns per-document importance with factor breakdown
- **R:** Importance must combine access frequency, recency, connections
- **P:** P2
- **Persona:** Business Analyst
- **Steps:** 1) Index docs 2) Access some frequently 3) Check importance
- **Expected:** Frequently accessed docs score higher

#### TC-020: memory_graph_query traversal
- **Q:** Does memory_graph_query traverse entity relationships?
- **A:** Returns connected entities from starting point
- **R:** Graph query must follow relationship edges
- **P:** P1
- **Persona:** End User
- **Steps:** 1) Create entities with relationships 2) Query from start entity
- **Expected:** Returns connected entities and relationship types

#### TC-021: memory_related topic search
- **Q:** Does memory_related find topically connected memories?
- **A:** Returns memories related by entity co-occurrence
- **R:** Related must enrich results with entity context
- **P:** P1
- **Persona:** End User
- **Steps:** 1) Write memories about related topics 2) Query memory_related
- **Expected:** Returns topically connected memories

#### TC-022: memory_timeline chronological order
- **Q:** Does memory_timeline show events in correct time order?
- **A:** Returns memories sorted by creation timestamp
- **R:** Timeline must sort by time, not relevance
- **P:** P1
- **Persona:** End User
- **Steps:** 1) Write memories at different times 2) Query memory_timeline
- **Expected:** Results in chronological order

#### TC-023: memory_connect creates bidirectional link
- **Q:** Does memory_connect create a typed relationship between docs?
- **A:** Creates a connection visible from both documents
- **R:** Connect must store bidirectional edge
- **P:** P1
- **Persona:** End User
- **Steps:** 1) Create two docs 2) Connect them 3) Check connections from both sides
- **Expected:** Both docs show the connection

#### TC-024: memory_traverse N-hop
- **Q:** Does memory_traverse find docs N hops away in the graph?
- **A:** Returns docs reachable within N hops
- **R:** Traverse must do BFS/DFS with depth limit
- **P:** P2
- **Persona:** End User
- **Steps:** 1) Create chain A→B→C→D 2) Traverse from A with depth=2
- **Expected:** Returns B and C, not D

---

### D3: Performance

#### TC-025: FTS search latency on 500 docs
- **Q:** Can memory_search return results within 300ms for 500 indexed docs?
- **A:** Response time < 300ms for 50 sequential queries
- **R:** FTS5 performance target
- **P:** P1
- **Persona:** DevOps Tester
- **Steps:** 1) Index 500 docs 2) Run 50 sequential queries 3) Measure p95 latency
- **Expected:** p95 < 300ms

#### TC-026: Hybrid search latency
- **Q:** Can memory_query return results within 1s including reranking?
- **A:** End-to-end hybrid search < 1s (without external API calls)
- **R:** Search pipeline performance
- **P:** P1
- **Persona:** DevOps Tester
- **Steps:** 1) Index 200 docs with embeddings 2) Run hybrid queries
- **Expected:** p95 < 1000ms (mock reranker)

#### TC-027: Bulk indexing throughput
- **Q:** Can nano-brain index 1000 documents in under 30s?
- **A:** Indexing throughput ≥ 33 docs/sec
- **R:** Indexing must be efficient for initial load
- **P:** P2
- **Persona:** DevOps Tester
- **Steps:** 1) Prepare 1000 markdown docs 2) Trigger full index 3) Measure time
- **Expected:** Completes within 30s

#### TC-028: Memory usage under load
- **Q:** Does memory stay under 512MB after indexing 10K docs?
- **A:** RSS < 512MB
- **R:** Docker container memory limit
- **P:** P2
- **Persona:** DevOps Tester
- **Steps:** 1) Index 10K docs 2) Measure RSS 3) Run searches
- **Expected:** RSS stays under 512MB

---

### D4: Security

#### TC-029: SQL injection in memory_search
- **Q:** Is memory_search safe from SQL injection?
- **A:** Malicious SQL is treated as literal search text
- **R:** All queries must use parameterized statements
- **P:** P0
- **Persona:** Security Auditor
- **Steps:** 1) Search for `'; DROP TABLE docs; --` 2) Verify DB intact
- **Expected:** Returns empty results, no DB damage

#### TC-030: FTS injection via MATCH syntax
- **Q:** Can FTS5 MATCH syntax be injected via search queries?
- **A:** Special FTS operators are escaped
- **R:** FTS queries must sanitize MATCH expressions
- **P:** P0
- **Persona:** QA Destroyer
- **Steps:** 1) Search with `" OR 1=1 --` 2) Search with `NEAR(a b)` syntax
- **Expected:** Treated as literal text, no FTS operator injection

#### TC-031: Path traversal in memory_get
- **Q:** Can `../../etc/passwd` be used to read arbitrary files?
- **A:** Path is resolved within workspace boundary
- **R:** File access must be sandboxed
- **P:** P0
- **Persona:** Security Auditor
- **Steps:** 1) memory_get with `../../etc/passwd` 2) memory_get with absolute path outside workspace
- **Expected:** Returns error, no file access outside workspace

#### TC-032: SHA-256 hash integrity
- **Q:** Is content hashing done correctly for deduplication?
- **A:** SHA-256 produces correct, consistent hashes
- **R:** Hash function must be deterministic
- **P:** P0
- **Persona:** Security Auditor
- **Steps:** 1) Hash known content 2) Compare with expected SHA-256 3) Hash same content again
- **Expected:** Hashes match expected values and are deterministic

#### TC-033: Stdio mode suppresses sensitive output
- **Q:** Does stdio transport prevent leaking data to stdout?
- **A:** setStdioMode suppresses all non-protocol output
- **R:** Claude Desktop stdio integrity
- **P:** P0
- **Persona:** Security Auditor
- **Steps:** 1) Enable stdioMode 2) Trigger log output 3) Check stdout
- **Expected:** Only MCP protocol JSON on stdout

---

### D5: Data Integrity

#### TC-034: Concurrent file append atomicity
- **Q:** Do concurrent memory_write calls produce valid file content?
- **A:** All writes are serialized via sequentialFileAppend
- **R:** Write queue must serialize concurrent appends
- **P:** P0
- **Persona:** QA Destroyer
- **Steps:** 1) Fire 20 concurrent memory_write calls 2) Read the file
- **Expected:** All 20 entries present, no corruption

#### TC-035: computeDecayScore NaN guard
- **Q:** Does computeDecayScore handle invalid dates without returning NaN?
- **A:** Returns 0 for unparseable dates instead of NaN
- **R:** Decay score must never propagate NaN
- **P:** P0
- **Persona:** QA Destroyer
- **Steps:** 1) Call with null date 2) Call with "invalid" 3) Call with future date
- **Expected:** Returns numeric value (0 or valid score), never NaN

#### TC-036: Store transaction atomicity
- **Q:** Are cleanOrphanedEmbeddings and bulkDeactivateExcept atomic?
- **A:** Operations wrapped in SQLite transactions
- **R:** Batch operations must be atomic
- **P:** P0
- **Persona:** QA Destroyer
- **Steps:** 1) Start cleanOrphanedEmbeddings 2) Verify transaction wrapping in code
- **Expected:** Operations are atomic (all-or-nothing)

#### TC-037: Harvest state TOCTOU protection
- **Q:** Is saveHarvestState safe from time-of-check-to-time-of-use races?
- **A:** Uses atomic rename (temp file + renameSync)
- **R:** State persistence must be crash-safe
- **P:** P0
- **Persona:** QA Destroyer
- **Steps:** 1) Call saveHarvestState 2) Verify temp-file-then-rename pattern
- **Expected:** State file is always complete or absent, never partial

#### TC-038: getIndexHealth consistent snapshot
- **Q:** Does getIndexHealth return internally consistent metrics?
- **A:** All counts come from a single transaction
- **R:** Health metrics must be a point-in-time snapshot
- **P:** P1
- **Persona:** DevOps Tester
- **Steps:** 1) Run getIndexHealth during active writes 2) Check total = active + inactive
- **Expected:** Metrics are consistent (no torn reads)

#### TC-039: Write-then-search consistency
- **Q:** After memory_write, does memory_search immediately find the content?
- **A:** FTS index is updated synchronously during write
- **R:** Read-after-write consistency
- **P:** P0
- **Persona:** End User
- **Steps:** 1) memory_write with unique text 2) Immediately memory_search for that text
- **Expected:** Search returns the just-written document

---

### D6: Infrastructure

#### TC-040: SSE connection cleanup on disconnect
- **Q:** Does the SSE transport clean up when a client disconnects?
- **A:** onclose registered before connect, wrapped in try/catch
- **R:** No session leak on disconnect
- **P:** P0
- **Persona:** DevOps Tester
- **Steps:** 1) Connect SSE client 2) Abruptly disconnect 3) Check server state
- **Expected:** Session cleaned up, no memory leak

#### TC-041: Qdrant init serialization
- **Q:** Do concurrent requests to Qdrant serialize initialization?
- **A:** Uses initPromise to prevent duplicate ensureCollection calls
- **R:** Vector store init must be idempotent
- **P:** P1
- **Persona:** DevOps Tester
- **Steps:** 1) Fire 10 concurrent requests that trigger ensureCollection
- **Expected:** Only one collection creation, all requests succeed

#### TC-042: Embedding batch partial failure
- **Q:** Does embedding generation handle partial batch failures?
- **A:** Per-sub-batch try/catch with zero-vector fallback
- **R:** Partial failure must not block entire batch
- **P:** P1
- **Persona:** QA Destroyer
- **Steps:** 1) Send batch where some texts cause embedding errors
- **Expected:** Successful items get real vectors, failed items get zero vectors

#### TC-043: Reranker bounds check
- **Q:** Does the reranker filter out invalid indices?
- **A:** Bounds check before accessing documents[r.index]
- **R:** Reranker must handle out-of-range indices
- **P:** P1
- **Persona:** QA Destroyer
- **Steps:** 1) Mock reranker returning index > array length
- **Expected:** Invalid indices filtered, no crash

#### TC-044: Watcher timer cleanup
- **Q:** Does the file watcher clean up timers on stop?
- **A:** initialEmbedTimer cancelled in stop()
- **R:** No orphaned timers after shutdown
- **P:** P1
- **Persona:** DevOps Tester
- **Steps:** 1) Start watcher 2) Stop watcher 3) Check for leaked timers
- **Expected:** All timers cancelled

#### TC-045: Store initialization dedup
- **Q:** Can createStore be called twice without creating duplicate databases?
- **A:** storeCreating Set guard prevents duplicate init
- **R:** Store must be singleton per database path
- **P:** P1
- **Persona:** DevOps Tester
- **Steps:** 1) Call createStore twice concurrently for same path
- **Expected:** Same store instance returned, no duplicate

---

### D7: Edge Cases

#### TC-046: Empty string search
- **Q:** What happens when searching with an empty string?
- **A:** Returns empty results or all docs, no crash
- **R:** Must handle empty input gracefully
- **P:** P1
- **Persona:** QA Destroyer
- **Steps:** 1) memory_search with "" 2) memory_query with ""
- **Expected:** No crash, returns appropriate response

#### TC-047: Unicode/Vietnamese diacritics in search
- **Q:** Can FTS handle Vietnamese text with diacritics?
- **A:** Vietnamese characters are indexed and searchable
- **R:** UTF-8 support in FTS5
- **P:** P1
- **Persona:** End User
- **Steps:** 1) Index doc with "Xin chào thế giới" 2) Search for "chào"
- **Expected:** Returns the Vietnamese document

#### TC-048: Very large document (100KB+)
- **Q:** Can nano-brain index and retrieve a 100KB+ document?
- **A:** Document is chunked, indexed, and retrievable
- **R:** Chunker must handle large content
- **P:** P1
- **Persona:** End User
- **Steps:** 1) Write 100KB markdown doc 2) Index it 3) Search for content within it
- **Expected:** Document indexed (possibly chunked), searchable

#### TC-049: Special characters in queries
- **Q:** Do special characters (quotes, brackets, backslash) in queries work?
- **A:** Characters are escaped or handled gracefully
- **R:** Query parser must sanitize special chars
- **P:** P1
- **Persona:** QA Destroyer
- **Steps:** 1) Search for `[test](url)` 2) Search for `"quoted"` 3) Search for `path\to\file`
- **Expected:** No crashes, reasonable results

#### TC-050: Duplicate document handling
- **Q:** What happens if the same document is indexed twice?
- **A:** Content hash dedup prevents duplicates
- **R:** Deduplication via SHA-256
- **P:** P1
- **Persona:** QA Destroyer
- **Steps:** 1) Index same content twice 2) Check doc count
- **Expected:** Only one copy in the index

#### TC-051: Non-existent docid in memory_get
- **Q:** What happens when memory_get receives a non-existent docid?
- **A:** Returns appropriate error message
- **R:** Must handle missing documents gracefully
- **P:** P1
- **Persona:** QA Destroyer
- **Steps:** 1) memory_get with #nonexistent123
- **Expected:** Returns "not found" message, no crash

#### TC-052: Knowledge graph circular connections
- **Q:** Does memory_traverse handle cycles in the graph?
- **A:** Visited-set prevents infinite loops
- **R:** Graph traversal must detect cycles
- **P:** P1
- **Persona:** QA Destroyer
- **Steps:** 1) Create A→B→C→A cycle 2) Traverse from A with depth=10
- **Expected:** Returns A,B,C without infinite loop

#### TC-053: parseSearchConfig invalid input
- **Q:** Does parseSearchConfig handle garbage input?
- **A:** Returns default config, no crash
- **R:** Config parser must have fallback
- **P:** P1
- **Persona:** QA Destroyer
- **Steps:** 1) Call with undefined 2) Call with random string 3) Call with partial config
- **Expected:** Returns valid SearchConfig in all cases

#### TC-054: Empty tag list
- **Q:** What does memory_tags return when no documents have tags?
- **A:** Returns empty array
- **R:** Must handle empty state
- **P:** P2
- **Persona:** QA Destroyer
- **Steps:** 1) Create store with no tagged docs 2) Call memory_tags
- **Expected:** Returns empty array, no error

#### TC-055: memory_connect with invalid relationship type
- **Q:** What happens if memory_connect receives an invalid relationship type?
- **A:** Rejects with validation error
- **R:** Relationship types must be validated
- **P:** P2
- **Persona:** QA Destroyer
- **Steps:** 1) memory_connect with type "INVALID_REL"
- **Expected:** Returns validation error

---

## Summary Table

| Dimension | TCs | Priority Breakdown |
|-----------|-----|-------------------|
| D2: API | TC-001 → TC-024 (24) | P0:5, P1:16, P2:3 |
| D3: Performance | TC-025 → TC-028 (4) | P1:2, P2:2 |
| D4: Security | TC-029 → TC-033 (5) | P0:5 |
| D5: Data Integrity | TC-034 → TC-039 (6) | P0:5, P1:1 |
| D6: Infrastructure | TC-040 → TC-045 (6) | P0:1, P1:5 |
| D7: Edge Cases | TC-046 → TC-055 (10) | P1:8, P2:2 |
| **Total** | **55** | **P0:16, P1:32, P2:7** |
