## Why

When testing nano-brain with real-world queries from the zengamingx project, we discovered search quality issues. A query like "workflow rút tiền của user là gì sau khi sell item thành công?" returned documentation files but missed actual code implementations. The search found flow documentation (`.agent/_flows/`) but could not locate the actual withdrawal handlers (`monnectPaymentService.js`, `monnectTransactionRepo.js`, route definitions). Additionally, duplicate results appeared from similar file paths, and snippets were truncated at 700 chars without code structure awareness.

**Note:** JS/TS/Python symbol extraction already exists in `internal/symbol/` and `internal/chunker/symbol.go` using tree-sitter. The issue is NOT missing extraction — it's search result quality.

## What Changes

- **Implement result deduplication** — Detect and merge duplicate results from files with similar content or paths (e.g., same file appearing twice from different search legs, or near-identical paths like `.agent/_flows/` vs `.agents/_flows/`)

- **Improve snippet generation** — Replace fixed 700-char truncation with context-aware extraction that understands code structure (function boundaries, comment blocks, enclosing scope)

- **Improve search result ranking** — Prioritize code files (`.js`, `.ts`, `.go`) over documentation (`.md`) when query terms match symbol names; boost results where query tokens appear in file path or symbol name

## Capabilities

### New Capabilities
- `result-deduplication`: Detect and merge duplicate search results from files with similar content or paths
- `context-aware-snippets`: Generate snippets based on code structure (function boundaries) rather than fixed character limits
- `code-aware-ranking`: Boost code files over documentation when query matches symbol names or file paths

### Modified Capabilities
- `search-quality`: Enhance search pipeline to prioritize actual code files alongside documentation

## Impact

- **Code affected**: 
  - `internal/search/service.go` — add deduplication after RRF merge
  - `internal/search/dedup.go` — new file for dedup logic
  - `internal/server/handlers/search.go` — use snippet field, apply dedup
  - `internal/chunker/fixed.go` — context-aware snippet extraction

- **Dependencies**: None (uses existing tree-sitter for AST-aware snippets)

- **Performance**: Deduplication adds O(n) post-processing; snippet extraction adds minimal overhead (AST already parsed during indexing)

- **API changes**: Search response may include `deduplicated_from` field; snippets become more meaningful
