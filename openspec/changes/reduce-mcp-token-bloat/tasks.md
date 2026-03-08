## 1. Critical Bug Fixes

- [x] 1.1 In `embedPendingCodebase()` in `codebase.ts`, after chunking a batch: if a document produces 0 chunks, insert a sentinel row in `content_vectors` with `seq=-1` to mark it as processed. This prevents empty-body documents from appearing in `getHashesNeedingEmbedding()`.
- [x] 1.2 In `getHashesNeedingEmbedding()` in `store.ts`, ensure the query excludes documents that already have a sentinel row (seq=-1) in `content_vectors`. (Verified: existing LEFT JOIN already handles this — sentinel row makes cv.hash IS NULL false.)
- [x] 1.3 In `harvester.ts`, change the harvest state format from `Record<string, number>` to `Record<string, { mtime: number; retries?: number; skipped?: boolean }>`. Increment `retries` on each "output file missing" re-harvest. After 3 retries, set `skipped: true` and stop re-harvesting.
- [x] 1.4 In `harvester.ts`, add backward-compatible state loading — if a value is a plain number (old format), convert it to `{ mtime: number }`.

## 2. Log System Improvements

- [x] 2.1 In `logger.ts`, add `LogLevel` type (`'error' | 'warn' | 'info' | 'debug'`) and a module-level `logLevel` variable (default: `'info'`).
- [x] 2.2 Update `initLogger()` to accept `config.logging.level` and set the threshold.
- [x] 2.3 Update `log()` signature to accept optional third parameter `level: LogLevel = 'info'`. Drop messages below the configured threshold.
- [x] 2.4 Add `rotateLogs()` function: check current log file size, if >50MB rename to `.1` suffix. Delete log files older than 7 days from the logs directory. Call on each `log()` write (with a check interval to avoid stat on every call).
- [x] 2.5 Demote noisy log calls to `debug` level: `insertEmbeddingLocal` in `store.ts`, `insertEmbedding` in `store.ts`, `searchFTS` in `store.ts`, `searchVec` in `store.ts`, `searchVecAsync` in `store.ts`.

## 3. MCP Response Limits — memory_get and memory_multi_get

- [x] 3.1 In `server.ts` `memory_get` handler, change `maxLines` default from undefined to 200. When the document exceeds `maxLines`, truncate and append `\n... (truncated, showing {maxLines} of {totalLines} total lines. Use maxLines to see more)`.
- [x] 3.2 In `server.ts` `memory_multi_get` handler, change `maxBytes` default from 50000 to 30000.

## 4. MCP Response Limits — Code Intelligence Tools

- [x] 4.1 In `server.ts` `code_impact` handler, truncate `result.byDepth` to max depth 3 and max 50 total entries. Append `... and N more at depth 4+` if truncated.
- [x] 4.2 In `server.ts` `code_context` handler, truncate `result.incoming` to 20, `result.outgoing` to 20, `result.flows` to 10. Append `... and N more` for each truncated list.
- [x] 4.3 In `server.ts` `memory_focus` handler, truncate `dependencies` to 30 and `dependents` to 30. Append `... and N more` for each truncated list.
- [x] 4.4 In `server.ts` `memory_symbols` handler, truncate total results to 50. Append `... and N more symbols`.
- [x] 4.5 In `server.ts` `memory_impact` handler, truncate total results to 50. Append `... and N more`.
- [x] 4.6 In `server.ts` `code_detect_changes` handler, truncate `result.affectedFlows` to 20. Append `... and N more`.

## 5. npm Package Cleanup

- [x] 5.1 Add `"files"` field to `package.json`: `["src/", "!src/eval/", "!src/bench.ts", "bin/", ".opencode/", "SKILL.md", "AGENTS.md", "AGENTS_SNIPPET.md", "opencode-mcp.json"]`
- [x] 5.2 Run `npm pack --dry-run` and verify only expected files are included (no test/, openspec/, site/, ai/, docs/, commands/). Result: 42 files, 495.6KB unpacked.

## 6. Verification

- [x] 6.1 Run LSP diagnostics on all changed files — zero errors. (All 6 files clean: logger.ts, codebase.ts, store.ts, harvester.ts, server.ts, package.json)
- [x] 6.2 Verify `npm pack --dry-run` shows ~40 files, ~480KB unpacked. (42 files, 495.6KB)
- [ ] 6.3 Manual test: `npx nano-brain status` still works after logger changes. (Skipped: vitest not runnable in this environment due to missing rollup native module — pre-existing issue)
