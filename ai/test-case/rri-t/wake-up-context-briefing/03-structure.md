# Phase 3: STRUCTURE u2014 Q-A-R-P-T Test Cases

Format: Question | Answer | Risk | Priority | Test

## Dimension 1: UI/UX (CLI Output)

| ID | Q | A | R | P | T |
|----|---|---|---|---|---|
| UX-1 | Does CLI `wake-up` print readable briefing? | Yes, markdown-formatted to stdout | Low | P1 | Verify CLI handler calls generateBriefing and prints formatted |
| UX-2 | Does `--json` output valid JSON? | Yes, JSON.stringify of BriefingResult | Low | P1 | Parse CLI JSON output, verify structure |
| UX-3 | Does help text include wake-up? | Yes, in showHelp() output | Low | P2 | Check help text contains 'wake-up' |
| UX-4 | Is empty workspace message helpful? | "No memories yet" message shown | Medium | P1 | Generate briefing on empty store |

## Dimension 2: API Correctness

| ID | Q | A | R | P | T |
|----|---|---|---|---|---|
| API-1 | Does generateBriefing return correct structure? | BriefingResult with l0, l1_memories, l1_decisions, formatted | High | P0 | Unit test return shape |
| API-2 | Does MCP tool return content array? | Yes, [{type:'text', text: ...}] | Medium | P1 | Verify MCP handler output |
| API-3 | Does HTTP GET /api/wake-up return JSON? | Yes, {formatted: string} | Medium | P1 | Test HTTP route handler |
| API-4 | Does HTTP POST /api/wake-up work? | Yes, accepts {workspace, json, limit} | Medium | P1 | Test POST route |
| API-5 | Are all 3 surfaces consistent? | Same generateBriefing() call | Low | P1 | Code review: single function |
| API-6 | Does limit parameter work? | Controls top docs count | Medium | P1 | Call with limit=3, verify count |
| API-7 | getTopAccessedDocuments returns correct order? | Ordered by access_count DESC | High | P0 | Insert docs with varying access_count, verify order |
| API-8 | getRecentDocumentsByTags filters correctly? | Only docs with matching tags | High | P0 | Insert tagged/untagged docs, verify filter |

## Dimension 3: Performance

| ID | Q | A | R | P | T |
|----|---|---|---|---|---|
| PERF-1 | Is briefing generation <50ms? | Yes, template-based, no LLM | Low | P2 | Time generateBriefing call |
| PERF-2 | Do SQL queries use indexes? | Yes, prepared statements on indexed cols | Low | P2 | Check EXPLAIN QUERY PLAN |
| PERF-3 | Does 2000-char cap prevent unbounded output? | Yes, hard truncation | Low | P1 | Generate briefing with many docs |

## Dimension 4: Security

| ID | Q | A | R | P | T |
|----|---|---|---|---|---|
| SEC-1 | Are store methods using prepared statements? | Yes, getTopAccessedDocumentsStmt | High | P0 | Code review: no string interpolation in SQL |
| SEC-2 | Is projectHash sanitized? | Used in prepared statement parameter | Medium | P1 | Code review: parameter binding |
| SEC-3 | Are error messages safe? | No internal paths leaked in HTTP 500 | Medium | P2 | Test error response format |
| SEC-4 | Does supersede exclusion prevent stale data? | WHERE superseded_by IS NULL | High | P0 | Insert superseded doc, verify excluded |

## Dimension 5: Data Integrity

| ID | Q | A | R | P | T |
|----|---|---|---|---|---|
| DATA-1 | Are inactive docs excluded? | WHERE active = 1 | High | P0 | Insert inactive doc, verify excluded |
| DATA-2 | Are superseded docs excluded? | WHERE superseded_by IS NULL | High | P0 | Insert superseded doc, verify excluded |
| DATA-3 | Is project_hash scoping correct? | IN (?, 'global') | High | P0 | Insert docs with different hashes, verify scope |
| DATA-4 | Does tag join work correctly? | JOIN document_tags ON tag IN (?) | High | P0 | Test with multiple tags |
| DATA-5 | Are documents deduplicated? | GROUP BY d.id in tag query | Medium | P1 | Doc with 2 matching tags appears once |

## Dimension 6: Infrastructure

| ID | Q | A | R | P | T |
|----|---|---|---|---|---|
| INFRA-1 | Does TypeScript compile? | No errors from tsc --noEmit | High | P0 | Run tsc check |
| INFRA-2 | Are types.ts interfaces correct? | Both methods declared in Store | Medium | P1 | Check interface has both methods |
| INFRA-3 | Does store.ts implement both methods? | Yes, with prepared statements | High | P0 | Call methods on real store |

## Dimension 7: Edge Cases

| ID | Q | A | R | P | T |
|----|---|---|---|---|---|
| EDGE-1 | Empty store (0 docs) | Returns "no memories yet" | Medium | P0 | Test empty store briefing |
| EDGE-2 | Empty title | truncateLine returns "(untitled)" | Low | P1 | Test truncateLine('', 80) |
| EDGE-3 | Null/falsy title | truncateLine returns "(untitled)" | Low | P1 | Test truncateLine(null/undefined) |
| EDGE-4 | Title exactly at max length | No truncation | Low | P2 | Test truncateLine('a'.repeat(80), 80) |
| EDGE-5 | Title 1 char over max | Truncated with "..." | Low | P2 | Test truncateLine('a'.repeat(81), 80) |
| EDGE-6 | Output exactly 2000 chars | No truncation | Low | P2 | Construct briefing at boundary |
| EDGE-7 | Output 2001+ chars | Truncated with "..." | Medium | P1 | Briefing with many items |
| EDGE-8 | No decision-tagged docs | L1 decisions section empty | Low | P1 | Generate with no decision tags |
| EDGE-9 | WorkspaceProfile throws | L0 still works, just no topics | Medium | P1 | Mock profile to throw |
| EDGE-10 | missing modified_at | Shows "unknown" date | Low | P2 | Doc with null modified_at |
