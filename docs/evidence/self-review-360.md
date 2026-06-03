# Self-Review: Issue #360 Task Group §4

**Change Type:** infrastructure (cursor + hash mechanism update)  
**Story:** #360 (Add time-range filters to search pipeline)  
**Task Group:** §4 (Define TimeRangeFilter, thread through pipeline, extend QueryHash)  
**Date:** 2026-06-03  
**Reviewer (Self):** oracle  

---

## Summary

Implemented task group §4: defined `TimeRangeFilter` struct, threaded it through the search pipeline entry points, and extended `QueryHash` to hash ALL filter inputs (query + tags + scope + collections + time-range raw strings), fixing a pre-existing pagination bug where tag/scope/collections changes between cursor calls were silently ignored.

---

## Scope & Changes

### 1. New Structures

**`TimeRangeFilter` (cursor.go:19-61)**
- Holds both parsed times (`*time.Time`) and raw input strings (for cursor stability)
- Methods: `ToSqlNullTimes()`, `CursorString()`
- Design per D5: raw strings prevent drift when relative durations ("30d") are re-evaluated

**`QueryHashInput` (cursor.go:97-105)**
- Encapsulates all filter components: query, tags, scope, collections, timeRange
- Cleaner parameter passing than expanding QueryHash signature

### 2. Extended QueryHash

**Before:** `QueryHash(query string) string`  
**After:** `QueryHash(input QueryHashInput) string`

Hash now includes (in order, with `\x1f` delimiter):
1. Query text
2. Sorted tags
3. Scope
4. Sorted collections  
5. Time-range raw input strings

**Bug Fix:** Pre-existing pagination cursor bug where scope/tag/collections changes were ignored—now every filter change invalidates the cursor.

### 3. Updated Callers

**VerifyCursor signature:**
- Before: `VerifyCursor(token string, currentQuery string)`
- After: `VerifyCursor(token string, input QueryHashInput)`

**MCP Tool Entry Points (tools.go):**
- `registerMemoryQuery` (line 234-240)
- `registerMemorySearch` (line 334-340)
- `registerMemoryVSearch` (line 509-515)

All three now construct QueryHashInput and pass through verification.

### 4. Test Coverage

**cursor_test.go rewritten with 10 test scenarios:**

| Scenario | Test | Status | Purpose |
|----------|------|--------|---------|
| (a) Determinism | TestQueryHashInput | PASS | Same input → same hash always |
| (b) Query change | TestQueryHashDifferent | PASS | Different query → different hash |
| (c) Tag added | TestQueryHashDifferent | PASS | Regression test for pre-existing bug |
| (d) Tag order | TestQueryHashTagOrder | PASS | Sort stability (order doesn't matter) |
| (e) Collections change | TestQueryHashDifferent | PASS | Collections part of hash |
| (f) Collection order | TestQueryHashCollectionOrder | PASS | Sort stability |
| (g) Time-range stability | TestQueryHashTimeRangeRawStrings | PASS | Raw strings prevent drift |
| (h) Time-range nil vs empty | TestQueryHashNilTimeRange | PASS | Backward compat |
| (i) Cursor round-trip | TestEncodeDecodeRoundtrip | PASS | Encode/decode determinism |
| (j) Verify cursor | TestVerifyCursor | PASS | Cursor validation with all filters |

---

## Verification

### Build & Tests
```bash
✓ go build ./...                              (clean)
✓ go test -race -short ./internal/search/...  (all pass)
✓ go test -race -short ./...                  (all pass)
✓ go vet ./internal/search/...                (clean)
✓ golangci-lint on modified files             (0 issues)
```

### Code Quality Checks

**LSP/Type Safety:**
- No type errors in cursor.go or cursor_test.go
- All function signatures properly declared
- Imports clean (database/sql, sort, strings, time)

**Test Coverage:**
- 10 distinct cursor hash scenarios
- 4 QueryHashDifferent change scenarios  
- Tag/collection sort stability verified
- Nil vs empty TimeRangeFilter equivalence verified
- Round-trip encode/decode determinism verified

**Design Compliance:**
- ✓ Raw time-range strings used for cursor (not parsed times) — prevents drift per D5
- ✓ Cursor token external format unchanged (base64, version byte preserved) — only hash input changed
- ✓ No `time.Now()` calls in cursor.go — purity maintained
- ✓ No new HTTP handlers, MCP tool schemas, or CLI flags — those are Task 5
- ✓ TimeRange field threading ready for Task 5 (currently nil in all MCP calls)
- ✓ ASCII Unit Separator (`\x1f`) delimiter (cannot appear in user input)

---

## Scope Boundaries (MUST NOT DO Constraints)

| Constraint | Status | Rationale |
|-----------|--------|-----------|
| Do NOT change cursor token external format | ✓ PASS | Only hash input changed; encoding/decoding identical |
| Do NOT change VerifyCursor error message | ✓ PASS | ErrCursorQueryMismatch preserved; same UX |
| Do NOT call `time.Now()` in cursor.go | ✓ PASS | No time-dependent logic; purity maintained |
| Do NOT add HTTP handlers | ✓ PASS | Task 5 responsibility; MCP calls have nil TimeRange |
| Do NOT modify `internal/timefilter/` | ✓ PASS | Task 2 complete; untouched |
| Do NOT modify sqlc or migrations | ✓ PASS | Task 3 complete; only calling existing params |
| Do NOT introduce new dependencies | ✓ PASS | stdlib only (database/sql, sort, strings, time) |

---

## Bug Fix Evidence

**Pre-existing Bug:** Pagination cursor was computed from query text only, ignoring tags, scope, and collections. A user could:
1. Call `memory_search` with query="foo", tags=["a"]
2. Get cursor C1, offset O1
3. Call `memory_search` with query="foo", tags=["b"] (different tags)
4. Call memory_search with cursor=C1 — **BUG**: returns results for tags=["a"], not tags=["b"]

**Fix Verification (Test c):** `TestQueryHashDifferent/tag_change` now FAILS if tags are not included in hash, proving the regression test catches the bug.

---

## Files Modified

1. **internal/search/cursor.go** (241 → 261 lines)
   - TimeRangeFilter struct
   - QueryHashInput struct
   - QueryHash refactor
   - VerifyCursor signature update

2. **internal/search/cursor_test.go** (271 → 471 lines)
   - 10 test scenarios
   - QueryHashInput usage
   - Pre-existing bug regression tests

3. **internal/mcp/tools.go** (3 functions updated)
   - registerMemoryQuery
   - registerMemorySearch
   - registerMemoryVSearch

---

## Task 5 Readiness

The implementation is prepared for Task 5 (HTTP handler layer wiring):
- `TimeRange` field in QueryHashInput currently nil in all MCP calls
- Handler layer (Task 5) will populate from parsed request parameters
- No API changes needed; QueryHash/VerifyCursor signatures stable

---

## Compliance Checklist

- [x] All unit tests pass with `-race -short`
- [x] Cursor hash includes all filter inputs (query, tags, scope, collections, time-range)
- [x] Sort stability verified for tags and collections
- [x] Raw time-range strings used (not parsed times)
- [x] Determinism verified (multiple hash calls identical)
- [x] No new dependencies
- [x] No breaking public API changes (QueryHash/VerifyCursor signature change is expected per design)
- [x] Pre-existing pagination bug fixed (regression test added)
- [x] No `time.Now()` in cursor.go
- [x] Cursor token format preserved (base64, version byte)
- [x] Code quality: go vet clean, golangci-lint 0 issues on modified files
- [x] Self-review complete

---

## Sign-Off

**Status:** READY FOR MERGE  
**Confidence:** HIGH (all gates pass, regression tests verify bug fix)  
**Next Phase:** Task 5 (HTTP handler wiring + MCP request parameter parsing)

