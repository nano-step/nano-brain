## 1. FTS5 Query Sanitization

- [x] 1.1 Create `sanitizeFTS5Query(query: string): string` helper in `src/store.ts` that wraps user input in double quotes and escapes internal double quotes
- [x] 1.2 Handle edge cases: empty/whitespace-only queries return empty string, hyphenated words preserved as phrases
- [x] 1.3 Apply `sanitizeFTS5Query` in `searchFTS()` before passing to prepared statements
- [x] 1.4 Add unit tests for `sanitizeFTS5Query`: normal words, hyphens, FTS5 operators, double quotes, empty input, column name words
- [x] 1.5 Verify `memory_search` works via MCP with query `nano-brain architecture` (manual test)

## 2. ESM Compliance

- [x] 2.1 Audit all `src/*.ts` files for `require()` calls — confirm the `require('crypto')` fix in server.ts is the only instance
- [x] 2.2 Add a lint check script in package.json: `"lint:esm": "grep -r 'require(' src/ --include='*.ts' && exit 1 || exit 0"`
- [x] 2.3 Run lint:esm and verify it passes

## 3. Dynamic Config Reload

- [x] 3.1 Verify `memory_update` handler in server.ts reloads config from disk (already patched — confirm code is correct)
- [x] 3.2 Add test: start server with empty config, add collection to config file, call memory_update, verify it indexes the new collection's documents

## 4. Integration Tests

- [x] 4.1 Create `tests/integration.test.ts` with test helper that creates a real temp SQLite DB with sqlite-vec loaded
- [x] 4.2 Add test fixture: index 2-3 markdown documents into the real DB with FTS5 triggers firing
- [x] 4.3 Test `memory_search` handler end-to-end: valid query returns results with title, path, snippet
- [x] 4.4 Test `memory_search` with hyphenated query (`nano-brain`) — no SQL error
- [x] 4.5 Test `memory_search` with FTS5 operator words (`AND OR NOT`) — no SQL error
- [x] 4.6 Test `memory_search` with collection filter — only matching collection returned
- [x] 4.7 Test `memory_search` with empty query — returns empty results, no error
- [x] 4.8 Test `memory_update` handler: add file to collection dir, call update, verify document count increases
- [x] 4.9 Test `memory_status` handler: returns correct document count and collection info
- [x] 4.10 Teardown: verify temp DB is cleaned up after tests

## 5. Verification

- [x] 5.1 Run full test suite (`npm test`) — all existing 246 tests + new integration tests pass
- [x] 5.2 Start MCP server via `node bin/cli.js mcp`, send JSON-RPC initialize + tools/list, verify 8 tools listed
- [x] 5.3 Manual end-to-end: write memory, update index, search — full cycle works through MCP tools
