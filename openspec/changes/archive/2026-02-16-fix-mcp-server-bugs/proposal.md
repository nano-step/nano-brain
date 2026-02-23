## Why

The MCP server has multiple runtime bugs that only surface in real usage (not caught by unit tests). The server crashes or returns SQL errors when users call the tools through OpenCode. These bugs exist because: (1) tests mock internals rather than exercising real code paths, (2) ESM/CJS incompatibilities weren't caught, and (3) FTS5 query syntax isn't sanitized.

## What Changes

- Fix FTS5 search query sanitization — raw user queries containing words that match FTS5 syntax (column names, operators) cause `no such column` errors. Queries must be properly quoted/escaped before passing to `MATCH`.
- Remove all `require()` calls from ESM source files — `require('crypto')` in `server.ts` crashes at runtime under Node.js ESM mode. All CJS-style requires must use ESM `import`.
- Add integration tests that exercise real MCP tool handlers end-to-end — current tests mock store/server internals, missing runtime errors like the above.
- Fix `memory_update` to reload collection config dynamically — already patched but needs proper test coverage.
- Audit all prepared statements for parameter binding correctness — ensure collection filter parameters are bound in the right order.

## Capabilities

### New Capabilities
- `mcp-integration-testing`: End-to-end integration tests that start the real MCP server, send JSON-RPC requests, and verify responses against a real SQLite database with indexed documents.

### Modified Capabilities
- `search-pipeline`: Fix FTS5 query sanitization so user queries with hyphens, special characters, and words matching FTS5 column names don't cause SQL errors.
- `mcp-server`: Fix ESM compatibility (no `require()`), fix dynamic config reload in `memory_update`, ensure all tool handlers work under Node.js/tsx runtime.

## Impact

- **src/store.ts**: `searchFTS()` — must sanitize/quote FTS5 MATCH queries
- **src/server.ts**: Remove `require('crypto')` (already done), verify all tool handlers work end-to-end
- **tests/**: New integration test file exercising real MCP tool calls against real DB
- **No breaking changes** — all fixes are internal, MCP tool API unchanged
