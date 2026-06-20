## 1. Search Pipeline Enhancement

- [ ] 1.1 Add `mode` parameter to `memory_search` MCP tool definition in `internal/mcp/tools.go`
- [ ] 1.2 Add `mode` parameter to `memory_query` MCP tool definition in `internal/mcp/tools.go`
- [ ] 1.3 Implement `debugSearch()` function in `internal/search/service.go` that runs 3 parallel searches
- [ ] 1.4 Add `source` field to search result struct in `internal/search/` types
- [ ] 1.5 Wire `mode=debugging` through MCP handler to search service

## 2. Parallel Search Orchestration

- [ ] 2.1 Implement parallel search with errgroup in `internal/search/service.go`
- [ ] 2.2 Add 2s timeout per sub-search in debugging mode
- [ ] 2.3 Implement RRF merge of 3 result sets with source labels
- [ ] 2.4 Handle partial failures gracefully (return results from successful searches only)

## 3. Debugging Skill

- [ ] 3.1 Create `.opencode/skills/debugging/SKILL.md` with debugging workflow guidance
- [ ] 3.2 Include tool sequence: memory_search(mode=debugging) → memory_graph → memory_impact
- [ ] 3.3 Include source-aware result interpretation guidance

## 4. Testing & Benchmarking

- [ ] 4.1 Add unit tests for `debugSearch()` in `internal/search/service_test.go`
- [ ] 4.2 Run debugging benchmark before/after implementation
- [ ] 4.3 Verify no regression on existing feature understanding benchmark
