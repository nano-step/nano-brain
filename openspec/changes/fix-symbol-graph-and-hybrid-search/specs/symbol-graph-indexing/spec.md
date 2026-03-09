## ADDED Requirements

### Requirement: Pass db parameter to indexCodebase
The MCP server's `memory_index_codebase` handler SHALL pass the `deps.db` instance as the 6th argument to `indexCodebase()`. The watcher and CLI callers SHALL also pass `db` when available, so the `if (db && isTreeSitterAvailable())` guard in `codebase.ts` line 427 passes and tree-sitter symbol extraction runs.

#### Scenario: MCP handler passes db to indexCodebase
- **WHEN** `memory_index_codebase` is called via the MCP server and `deps.db` is defined
- **THEN** `indexCodebase(storeToUse, effectiveRoot, configToUse, effectiveProjectHash, providers.embedder, deps.db)` is called with `db` as the 6th argument
- **THEN** the `code_symbols` table is populated with extracted symbols after indexing completes

#### Scenario: MCP handler without db still works
- **WHEN** `memory_index_codebase` is called and `deps.db` is undefined
- **THEN** `indexCodebase()` is called with `undefined` as the 6th argument
- **THEN** symbol graph indexing is skipped (existing behavior preserved)
- **THEN** document indexing and embedding still complete successfully

#### Scenario: Watcher passes db to indexCodebase
- **WHEN** the file watcher triggers `indexCodebase()` in `watcher.ts` (lines 146, 164)
- **THEN** the `db` instance is passed as the 6th argument when available
- **THEN** symbol graph is updated incrementally on file changes

#### Scenario: CLI passes db to indexCodebase
- **WHEN** the CLI `init` command triggers `indexCodebase()` in `index.ts` (line 820)
- **THEN** the `db` instance is passed as the 6th argument when available

### Requirement: Symbol graph populated after indexing
After `indexCodebase()` runs with a valid `db` parameter and tree-sitter is available, the `code_symbols` table SHALL contain rows for extracted symbols.

#### Scenario: Symbols extracted from TypeScript files
- **WHEN** `indexCodebase()` runs on a workspace containing TypeScript files with exported functions and classes
- **THEN** `SELECT COUNT(*) FROM code_symbols WHERE project_hash = ?` returns a value greater than 0
- **THEN** each row has non-empty `name`, `kind`, `file_path`, valid `start_line` and `end_line`

#### Scenario: Re-indexing updates symbols
- **WHEN** `indexCodebase()` runs a second time after a file is modified
- **THEN** symbols for the modified file are updated (old entries replaced)
- **THEN** symbols for unmodified files are unchanged

### Requirement: Add searchByName to SymbolGraph
The `SymbolGraph` class SHALL expose a `searchByName(pattern: string, projectHash: string, limit?: number): SymbolRecord[]` method that finds symbols by name with camelCase/snake_case-aware matching.

#### Scenario: Exact name match
- **WHEN** `searchByName("getUserData", "proj123")` is called and a symbol named `getUserData` exists
- **THEN** the result includes the `getUserData` symbol with all fields populated

#### Scenario: CamelCase partial match
- **WHEN** `searchByName("userData", "proj123")` is called and a symbol named `getUserData` exists
- **THEN** the result includes `getUserData` because the sub-tokens `user` and `data` match

#### Scenario: Case-insensitive matching
- **WHEN** `searchByName("getuser", "proj123")` is called and a symbol named `getUserData` exists
- **THEN** the result includes `getUserData` because case-insensitive sub-token matching succeeds

#### Scenario: No matches
- **WHEN** `searchByName("nonExistentSymbol", "proj123")` is called
- **THEN** an empty array is returned

#### Scenario: Limit parameter
- **WHEN** `searchByName("get", "proj123", 5)` is called and 20 symbols contain "get"
- **THEN** at most 5 results are returned, ordered by relevance (exact match first, then prefix, then substring)

### Requirement: CamelCase/snake_case token splitting
A utility function `splitIdentifier(name: string): string[]` SHALL split identifiers into lowercase sub-tokens for matching.

#### Scenario: CamelCase splitting
- **WHEN** `splitIdentifier("getUserData")` is called
- **THEN** the result is `["get", "user", "data"]`

#### Scenario: snake_case splitting
- **WHEN** `splitIdentifier("get_user_data")` is called
- **THEN** the result is `["get", "user", "data"]`

#### Scenario: Mixed case splitting
- **WHEN** `splitIdentifier("parseJSON_response")` is called
- **THEN** the result is `["parse", "json", "response"]`

#### Scenario: Single word
- **WHEN** `splitIdentifier("store")` is called
- **THEN** the result is `["store"]`

#### Scenario: Acronym handling
- **WHEN** `splitIdentifier("parseHTTPResponse")` is called
- **THEN** the result is `["parse", "http", "response"]`
