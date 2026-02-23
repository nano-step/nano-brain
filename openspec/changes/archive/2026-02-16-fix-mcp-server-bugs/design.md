## Context

opencode-memory is an MCP server that provides hybrid search (BM25 + vector) over indexed markdown documents. It runs as a subprocess spawned by OpenCode via stdio transport. The server uses better-sqlite3 with FTS5 for full-text search and sqlite-vec for vector search.

Three categories of bugs surfaced during real-world testing that unit tests failed to catch:

1. **FTS5 query injection** — User queries passed directly to FTS5 `MATCH` without sanitization. FTS5 interprets bare words matching column names (`filepath`, `title`, `body`) as column references, and hyphens as `NOT` operators.
2. **ESM/CJS mismatch** — `require('crypto')` in an ESM module crashes under Node.js. Tests use vitest which transpiles CJS-style requires, masking the error.
3. **Static config loading** — Collections loaded once at server startup. Adding a collection requires server restart.

## Goals / Non-Goals

**Goals:**
- All MCP tool handlers work correctly when called through real JSON-RPC over stdio
- User queries with hyphens, special characters, and FTS5 reserved words search correctly
- Zero `require()` calls in any `.ts` source file
- Integration tests that catch runtime errors before deployment

**Non-Goals:**
- Changing the MCP tool API or adding new tools
- Improving search relevance or ranking algorithms
- Adding new features beyond bug fixes

## Decisions

### D1: FTS5 Query Sanitization Strategy

**Decision:** Quote each search term individually and join with implicit AND.

**Rationale:** FTS5 has complex query syntax where bare words can be interpreted as column names, `AND`/`OR`/`NOT`/`NEAR` as operators, and `-` as NOT prefix. Rather than trying to escape individual characters, we split the user query into tokens, wrap each in double quotes (`"term"`), and join them. This ensures every token is treated as a literal search term.

**Example:** `opencode-memory architecture` → `"opencode" "memory" "architecture"` (hyphen splits into separate quoted terms since FTS5 treats `-` as NOT)

Actually, better approach: wrap the entire query in double quotes to preserve phrases, and escape any internal double quotes.

**Final decision:** `"opencode-memory architecture"` → `"opencode-memory architecture"` (single quoted phrase). Escape internal `"` as `""`.

**Alternatives considered:**
- Column-prefixed queries (`body:"query"`) — too restrictive, we want to search all columns
- FTS5 `simple` tokenizer — would lose porter stemming benefits

### D2: Integration Test Approach

**Decision:** Create a test helper that starts the real MCP server in-process, sends JSON-RPC messages, and asserts on responses. Use a temporary SQLite database with pre-indexed test documents.

**Rationale:** The current tests mock `Store` and `SearchProviders`, which means the actual SQL queries, FTS5 interactions, and parameter binding are never tested. An integration test that uses a real database catches the exact class of bugs we hit.

**Alternatives considered:**
- Subprocess-based testing (spawn `node bin/cli.js mcp` and pipe JSON-RPC) — too slow, harder to debug
- Just adding more unit tests — wouldn't catch ESM/CJS issues or SQL parameter binding bugs

### D3: ESM Compliance Audit

**Decision:** `grep -r "require(" src/` as a CI-enforceable check. All imports must use ESM `import` syntax.

**Rationale:** vitest transpiles `require()` calls, so they work in tests but fail at runtime under Node.js ESM. A simple grep catches this at lint time.

### D4: Dynamic Config Reload in memory_update

**Decision:** `memory_update` tool handler reloads `config.yml` on every invocation instead of using the cached startup value.

**Rationale:** Users add collections via CLI (`collection add`) which writes to config.yml. The MCP server is a long-running process — it shouldn't require restart to see new collections. The config file is tiny (< 1KB), so re-reading it on each update call has negligible cost.

## Risks / Trade-offs

- **FTS5 quoting may reduce search flexibility** — Users can't use FTS5 advanced syntax (OR, NEAR, column filters). This is acceptable because MCP tool users are AI agents, not power users writing FTS5 queries. → Mitigation: If needed later, add a `raw` parameter to bypass sanitization.
- **Integration tests add test runtime** — Real SQLite operations are slower than mocks. → Mitigation: Keep integration test count small (5-10 critical paths), run unit tests separately.
- **Dynamic config reload on every update** — Tiny performance cost. → Mitigation: Config file is < 1KB, `fs.readFileSync` + YAML parse is < 1ms.
