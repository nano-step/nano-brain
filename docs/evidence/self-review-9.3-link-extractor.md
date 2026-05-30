# Self-Review Gate 2.4 — Story 9.3 Link Extractor

**Reviewer:** Oracle  
**Implementer:** Sisyphus-Junior  
**Date:** 2026-05-30  
**Commit range:** story-9.3-link-extractor branch

## Verdict: PASS

(with 2 medium findings that should be addressed before or shortly after merge)

## Per-Criterion Table

| AC | Description | Verdict | Evidence |
|----|-------------|---------|----------|
| AC1 | Migration 00011 correct | **PASS** | File exists. Up: drops CHECK, adds new CHECK with `('contains','imports','calls','references')`. Down: restores old CHECK with documented "fails if references rows exist" comment. Constraint name `graph_edges_edge_type_check` matches Postgres default. |
| AC2 | Zero internal deps in prod | **PASS** | `grep -E '"github.com/nano-brain/nano-brain/internal/' internal/links/*.go \| grep -v _test.go` → empty. Test files import only `internal/links`, `internal/storage/sqlc`, `internal/testutil` — all allowed. No `eventbus` import. |
| AC3 | Parser 8 cases | **PASS** | All 8 tested: simple_uuid, simple_title, mixed, escaped, malformed_unterminated, nested_brackets_inner_extracted, over_200_chars, unicode_title. `go test -race -v ./internal/links/... -run TestParse` → all PASS (0.00s). |
| AC4 | Resolver 6 cases | **PASS** | All 6 tested: resolve_id_hit, resolve_id_miss, resolve_title_case_insensitive, resolve_title_ambiguous, cache_hit_no_extra_query, flush_clears_cache. `go test -race -v ./internal/links/... -run TestResolver` → all PASS (0.00s). LRU cache is workspace-scoped (key = `workspace\x00lower(title)`). |
| AC5 | Extractor integration test | **PASS** | File has `//go:build integration` tag. Scenarios: write A → B refs A → 1 edge; re-extract idempotent → still 1; C refs A → 2 total; rename title + flush + new doc → resolves; scope-skip on code:go collection → 0 edges. TEST_DATABASE_URL required (skips gracefully per testutil). |
| AC6 | Cache-flush ordering | **PASS** | 3 wiring sites confirmed: `document.go:195`, `automemory.go:142`, `claudecode.go:256`. All follow `FlushWorkspace(ws)` THEN `Extract(ctx, doc)` pattern. Extract errors are logged-warn, not failed-write. |
| AC7 | No regression | **PASS** | `go test -race -short ./...` → 22 packages pass (including `internal/graph/`, `internal/server/handlers/`). Handler tests updated correctly — all `WriteDocument` calls pass `nil, nil` for new linkResolver/linkExtractor params. |
| AC8 | Build + vet clean | **PASS** | `go build ./...` exit 0, `go vet ./internal/links/...` exit 0. No output. |

## Critical Findings (Blocking)

None.

## Medium Findings (Should Fix)

### M1: No transaction wrapping delete+upsert in `extract.go`

**Location:** `internal/links/extract.go` lines 171–188  
**Spec says:** "diff-and-upsert 'references' edges in a single transaction" (story spec line 53)  
**Code does:** Separate `DeleteReferenceEdgesBySource` then loop of `UpsertReferenceEdge` — no `BeginTx`.  
**Impact:** If delete succeeds but an upsert fails mid-loop, edges are lost until next Extract call for the same doc. Since Extract is best-effort (logged-warn), the risk is bounded — recovery is automatic on next write. The `ExtractorQueries` interface lacks a `BeginTx` method, so fixing requires interface extension.  
**Recommendation:** Add a `WithTx(tx *sql.Tx) ExtractorQueries` method to the adapter and wrap delete+upsert in a transaction inside Extract. Medium effort.  
**Severity:** MEDIUM — data consistency gap under transient DB failures, but recovery is automatic.

### M2: Ambiguous title picks by sorted UUID, not `updated_at`

**Location:** `internal/links/extract.go` lines 152–156  
**Spec says:** "extractor inserts edge to most-recent (by `updated_at`)" (story spec line 85)  
**Code does:** `sort.Slice(ids, func(i, j int) bool { return ids[i].String() < ids[j].String() })` — sorts by UUID string, which is arbitrary.  
**Impact:** When multiple docs share the same title (rare), the linked target is deterministic but not the most-recent. Metadata correctly stamps `ambiguous: true` + candidate IDs.  
**Recommendation:** Change `ListDocIDsByTitle` query to `ORDER BY updated_at DESC` and pick `ids[0]` without re-sorting. Requires sqlc query change.  
**Severity:** MEDIUM — spec deviation, but low-impact since ambiguous titles are rare and metadata flags it.

## Minor Findings (Nits)

### N1: `claudecode.go` passes `Collection: "sessions"` to Extract

**Location:** `internal/harvest/claudecode.go` line 263  
**Issue:** Extract's scope check only allows `"memory"` and `"session-summary:*"`. `"sessions"` doesn't match either → Extract is always a no-op for this path. The wiring code runs FlushWorkspace + Extract for no reason.  
**Recommendation:** Either change collection to the correct value or remove the dead wiring. Verify intent with the implementer.

### N2: `golang-lru/v2` marked `// indirect` in `go.mod`

**Location:** `go.mod`  
**Issue:** Package is directly imported in `internal/links/resolve.go` but tagged `// indirect`.  
**Fix:** Run `go mod tidy` — it should correct the annotation.

### N3: Event payload `deleted_count` is hardcoded to 0

**Location:** `internal/links/extract.go` line 194  
**Issue:** The code deletes edges but doesn't track the count. Since `pub` is nil in production (eventbus from Story 9.1 not merged), this is cosmetic until then.

### N4: `resolve_title_case_insensitive` test doesn't actually prove case insensitivity

**Location:** `internal/links/resolve_test.go` lines 70–82  
**Issue:** The fake query map key uses the exact input casing. A call with different casing (e.g., "ARCHITECTURE OVERVIEW") would fail the fake lookup. Case insensitivity is correctly handled by the SQL `lower()` function, not tested by the unit test. This is a test gap, not a production bug.

## Skeptical Checks

| Check | Result |
|-------|--------|
| `linksadapter.go` justified? | **YES** — necessary adapter between sqlc generated types and links package interfaces (zero-internal-dep requirement forces mirrored param structs). Same pattern used in integration test. |
| `go.mod` additions | `hashicorp/golang-lru/v2 v2.0.7` added (marked indirect, should be direct — N2). No other new deps. |
| Reindex wiring skipped? | **CORRECT** — `TriggerReindex` resets chunks/embeddings, doesn't upsert documents. Write handler (which has wiring) is the path that creates documents. Reindex has nothing to extract from. |
| Cross-workspace defense | **VERIFIED** — `ListDocIDsByTitle` scopes by `workspace_hash`. `UpsertReferenceEdge` uses `doc.Workspace` as `WorkspaceHash`. Resolver only returns IDs from same workspace. |
| `Document.Title` ambiguous stamp | **VERIFIED** — `extract.go` lines 154, 161-162: sets `m["ambiguous"] = true` and `m["candidate_ids"]` when `len(ids) > 1`. |
| Handler test modifications | **FAITHFUL** — all 8 `WriteDocument` calls add `nil, nil` for new params. No error suppression, no unrelated changes. |
| `Publisher` interface decoupling | **VERIFIED** — `links` package defines its own `Publisher` interface and `Event` struct. No `eventbus` import. Comment in extract.go explains structural subtyping intent. |

VERIFIED

---

## Re-review after N1 Course-Correction

**Date:** 2026-05-30  
**Trigger:** N1 finding — `claudecode.go` passed `Collection: "sessions"` to Extract (always no-op). First fix attempted adding `"sessions"` to the scope filter — WRONG (raw transcripts ≠ summaries). Course-corrected to proper fix below.

### Changes Applied

1. Scope filter in `extract.go` reverted to `"memory" || "session-summary"` (exact match, no `"sessions"`).
2. claudecode.go wiring removed entirely (raw transcript path, not summary).
3. New wiring added to `internal/summarize/persist.go` (where summaries are written under `Collection: "session-summary"`).
4. opencode.go and opencode_sqlite.go confirmed never wired (correct).
5. document.go (memory API) and automemory.go (memory harvest) wiring preserved.

### Per-Criterion Table

| # | Check | Verdict | Evidence |
|---|-------|---------|----------|
| 1 | Scope filter exact | **PASS** | `extract.go:101`: `if doc.Collection != "memory" && doc.Collection != "session-summary" { return nil }` — no `HasPrefix`, no `"sessions"`. |
| 2 | persist.go wiring correct | **PASS** | Fields: `linkResolver *links.Resolver` (L29), `linkExtractor *links.Extractor` (L30). `SetLinkExtractor` injection (L44). Call: `FlushWorkspace` (L126) THEN `Extract` (L127) with `Collection: "session-summary"` (L133). Error: `logger.Warn()` (L135) — not failed-write. |
| 3 | claudecode.go wiring removed | **PASS** | `grep -n 'linkExtractor\|linkResolver\|links\.\|Extract\|FlushWorkspace' internal/harvest/claudecode.go` → empty. Same for `opencode.go`, `opencode_sqlite.go`. |
| 4 | Exactly 3 Extract call sites | **PASS** | `grep -rn 'extractor.Extract\|linkExtractor.Extract' internal/ --include='*.go' \| grep -v _test.go` → `summarize/persist.go:127`, `handlers/document.go:196`, `harvest/automemory.go:143`. No extras. |
| 5 | Build + tests pass | **PASS** | `go build ./...` exit 0. `go test -race -short ./...` → 22 packages pass (including `internal/summarize`). |
| 6 | AC6 cache-flush ordering | **PASS** | All 3 wiring sites call `FlushWorkspace` before `Extract`: document.go:195→196, automemory.go:142→143, persist.go:126→127. |
| 7 | Integration test collections | **PASS** | Uses `"memory"` for positive cases, `"code:go"` for scope-skip. No `"sessions"` usage. |

### Newly Surfaced Issues

None. The course-correction is clean.

### Verdict: PASS

VERIFIED
