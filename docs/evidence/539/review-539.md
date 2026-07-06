## Review Gate — Issue #539 (agent-ergonomics)

Date: 2026-07-06
Branch: feat/539-agent-ergonomics
Reviewer: independent `oh-my-claudecode:code-reviewer` sub-agent (NOT the implementing agents; R88 satisfied)

### Review Verdict: PASS (after fixes)

The independent reviewer returned PASS with no CRITICAL/HIGH blockers, 2 MEDIUM and
3 LOW findings. All actionable findings were fixed by the integrating agent (author
of the fixes ≠ implementing executors ≠ reviewer):

| Finding | Sev | Resolution | Evidence |
|---|---|---|---|
| F3 symbol-path returned the WHOLE FILE when line metadata missing (un-reindexed workspaces) | MEDIUM | `memory_get` now swaps to the parent file only when a line span actually resolves; otherwise keeps the signature (tools.go) | `TestMemoryGet_SymbolPathWithParentButNoLineMetadata_ReturnsSignature` PASS |
| F1 negative-slice panic on a symbol `source_path` with no `?` | MEDIUM | Guard `strings.Index(...) < 0` → skip the malformed row instead of panicking | build+vet clean; defensive |
| F2 masked non-`ErrNoRows` DB errors behind chunk fallback | LOW | `resolveDocumentByAnyID` returns early on a real DB error | code inspection |
| F4 could cache a degenerate empty embedding | LOW | `embedQueryCached` skips caching when `len(vec)==0` | code inspection |
| AC-1 trace node not directly re-feedable into `memory_get` | LOW→closed | `memory_get` now accepts a `file::Symbol` graph node (reuses `ResolveSymbolByName` + symbol-body path) | `TestMemoryGet_GraphNodePathReturnsBody` PASS |

### Acceptance criteria → evidence

| AC | Status | Evidence |
|---|---|---|
| AC-1 trace nodes `file::symbol`, re-feedable into get/graph/impact/trace | PASS | `TestMemoryTrace_QualifiesBareTargetsAndDropsExternal`, `TestMemoryGet_GraphNodePathReturnsBody` |
| AC-2 builtins absent by default, present with include_external | PASS | `TestMemoryTrace_QualifiesBareTargetsAndDropsExternal` |
| AC-3 same-named methods distinct; no phantom re-entry | PASS | `TestMemoryTrace_AmbiguousSameNameSymbolsYieldDistinctNodes`, `TestMemoryTrace_EntrySymbolDoesNotReappearViaNameOnlyEdge` |
| AC-4/5/6 chunk-id→parent doc; doc-id works; clean error, no sql leak | PASS | `TestMemoryGet_ChunkIDResolvesToParentDocument`, `TestMemoryGet_UnknownIDReturnsCleanError` |
| AC-7 memory_symbols start_line/end_line | PASS | `TestMemorySymbols_ExposesLineSpan` |
| AC-8 memory_get symbol body > signature (fresh index) | PASS | `TestMemoryGet_SymbolPathReturnsFullBody` |
| AC-9 query-embed cache embeds once on repeat | PASS | `TestHybridSearch_RepeatedQuery_EmbedsOnce` (internal/search) |
| AC-10 no search regression | PASS | full `-race -short ./...` exit 0 |

### Test evidence (run against nanobrain_test, :5432/nanobrain_test — never dev DB)

```
go test -race -short ./...                → exit 0 (all packages pass)
go test -race -tags=integration -run 'TestMemoryGet|TestMemoryTrace_*|TestMemorySymbols_ExposesLineSpan' ./internal/mcp/...
  → 16 PASS incl. all 10 #539 tests; ok  internal/mcp  2.692s
CGO_ENABLED=0 go build ./...              → clean
go vet ./internal/mcp/... ./internal/search/... → clean
```

### smoke:e2e note (high-risk lane)

Live-server smoke (`serve --unsafe-no-auth`) was intentionally NOT run: the auto-mode
safety classifier blocked the `--unsafe-no-auth` flag and I did not work around it.
User-flow coverage is provided instead by `agent_ergonomics_539_integration_test.go`,
which constructs the **real MCP adapter against real Postgres** and drives every
changed handler through the agent's actual entry point (the MCP tool call surface) —
stronger than a curl smoke against an empty DB.

### Pre-existing failures (NOT caused by this work — confirmed on clean origin/master)

`TestMemoryGraph_RelativeNodeInputResolvesToAbsolute`, `TestMemoryGraph_RelativeOutputStripsPrefix`,
`TestMemoryTrace_RelativeInputAndOutput` (panic), `TestMemoryWakeUp_OnlyReturnsMemoryAndSessionSummaryDocs`
fail identically on `origin/master` (verified via stash + checkout). Root cause is a
stale fixture in `graph_paths_integration_test.go` (absolute nodes vs relative storage);
`TestMemoryTrace_RelativeInputAndOutput` is already logged as pre-existing in STATE.md
(Phase 09-03). Out of scope for #539.

### Deferred (follow-ups, per DESIGN non-goals)
- Extraction-time scope-aware call resolution → #501.
- pgvector/Ollama pool + batch tuning → #372/#373 perf track.
- Existing workspaces must re-index (watcher re-extract / `memory_update`) for the new
  symbol line metadata (F3) to appear.
</content>
</invoke>
