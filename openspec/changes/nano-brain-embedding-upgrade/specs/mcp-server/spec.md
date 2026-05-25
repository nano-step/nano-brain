# MCP Server Specification

## Purpose

MCP server providing persistent memory tools (search, status, update, get) for AI coding agents via the Model Context Protocol.

## Requirements

### Requirement: ESM module compliance

All source files in `src/` SHALL use ESM `import` syntax exclusively. No `require()` calls SHALL exist in any TypeScript source file.

#### Scenario: Server starts under Node.js ESM runtime
- **WHEN** the MCP server is started via `node bin/cli.js mcp`
- **THEN** the server starts without `require is not defined` errors
- **THEN** all tool handlers execute without CJS/ESM compatibility errors

#### Scenario: No require() in source files
- **WHEN** running `grep -r "require(" src/` on the source directory
- **THEN** zero matches are returned (excluding comments and string literals)

### Requirement: Dynamic collection config reload

The `memory_update` tool handler SHALL reload the collection configuration file on every invocation, not use the cached startup value.

#### Scenario: Collection added after server start
- **WHEN** a user adds a collection via CLI (`collection add`) while the MCP server is running
- **THEN** calling `memory_update` through MCP indexes documents from the newly added collection
- **THEN** no server restart is required

#### Scenario: Collection removed after server start
- **WHEN** a user removes a collection via CLI while the MCP server is running
- **THEN** calling `memory_update` through MCP no longer indexes documents from the removed collection

### Requirement: All MCP tool handlers return valid responses

Every registered MCP tool SHALL return a valid JSON-RPC response for valid inputs, never an unhandled exception.

#### Scenario: memory_search with valid query
- **WHEN** `memory_search` is called with `{"query": "test"}` via JSON-RPC
- **THEN** a valid response with `content` array is returned

#### Scenario: memory_update with configured collections
- **WHEN** `memory_update` is called via JSON-RPC with collections configured
- **THEN** a valid response with reindex summary is returned, not a runtime error

#### Scenario: memory_status returns health info
- **WHEN** `memory_status` is called via JSON-RPC
- **THEN** a valid response with document count, chunk count, and collection info is returned

### Requirement: Search tools support workspace filtering

The `memory_search`, `memory_vsearch`, and `memory_query` MCP tools SHALL accept an optional `workspace` parameter. When omitted, results are scoped to the current workspace and global documents. When set to `"all"`, results include all workspaces.

#### Scenario: memory_search with default workspace scoping
- **WHEN** `memory_search` is called with `{"query": "test"}` and no `workspace` parameter
- **THEN** results are filtered to `currentProjectHash` and `'global'` documents only

#### Scenario: memory_vsearch with workspace="all"
- **WHEN** `memory_vsearch` is called with `{"query": "test", "workspace": "all"}`
- **THEN** results include documents from all workspaces

#### Scenario: memory_query with specific workspace
- **WHEN** `memory_query` is called with `{"query": "test", "workspace": "abc123def456"}`
- **THEN** results are filtered to `project_hash = 'abc123def456'` and `project_hash = 'global'`

### Requirement: memory_status reports storage usage

The `memory_status` tool SHALL report per-workspace document counts and total storage size, in addition to existing health information.

#### Scenario: memory_status with workspace data
- **WHEN** `memory_status` is called after documents from multiple workspaces are indexed
- **THEN** the response includes a breakdown of document counts per workspace (projectHash)
- **THEN** the response includes total storage size (DB + sessions directory)
- **THEN** the response includes storage limit configuration (maxSize, retention, minFreeDisk)

### Requirement: Search tool parameter schema includes workspace

The MCP tool registration for `memory_search`, `memory_vsearch`, and `memory_query` SHALL include `workspace` in their input schema as an optional string parameter with description explaining the scoping behavior.

#### Scenario: Tool schema advertises workspace parameter
- **WHEN** an MCP client lists available tools
- **THEN** `memory_search`, `memory_vsearch`, and `memory_query` each show a `workspace` parameter in their input schema
- **THEN** the parameter description explains: omit for current workspace, `"all"` for cross-workspace search

## MODIFIED Requirements

### Requirement: Default embedding model is mxbai-embed-large

The MCP server SHALL use `mxbai-embed-large` as the default embedding model with 1024 dimensions, replacing the previous default `nomic-embed-text` (768 dimensions).

#### Scenario: Server starts with new default model
- **WHEN** the MCP server starts without explicit model configuration
- **THEN** the embedding model is set to `mxbai-embed-large`
- **THEN** the embedding dimensions are set to 1024

#### Scenario: Existing vectors rebuilt on dimension mismatch
- **WHEN** the server detects existing `vectors_vec` table with 768 dimensions
- **THEN** `ensureVecTable()` rebuilds the table with 1024 dimensions
- **THEN** all rows in `content_vectors` are cleared to trigger re-embedding

#### Scenario: Model configuration overrides default
- **WHEN** the user configures a custom embedding model in config
- **THEN** the custom model is used instead of `mxbai-embed-large`
- **THEN** the custom model's dimensions are used

### Requirement: Increased truncation limit for embeddings

The `OLLAMA_MAX_CHARS` constant SHALL be increased from 1800 to 6000 characters to support longer context embeddings.

#### Scenario: Long text truncated at 6000 chars
- **WHEN** a document chunk with 8000 characters is embedded
- **THEN** the text is truncated to 6000 characters before sending to Ollama
- **THEN** the truncated text is embedded

#### Scenario: Text under 6000 chars not truncated
- **WHEN** a document chunk with 4500 characters is embedded
- **THEN** the full 4500 character text is sent to Ollama without truncation

### Requirement: Increased batch size for embedding operations

The default batch size for `embedPendingCodebase()` SHALL be increased from 10 to 50 chunks per Ollama request.

#### Scenario: Batch embedding with 50 chunks
- **WHEN** `embedPendingCodebase()` is called with default batch size
- **THEN** up to 50 chunks are sent to Ollama in a single `/api/embed` request
- **THEN** the request includes `input` as an array of up to 50 chunk texts

#### Scenario: Multiple batches for large codebase
- **WHEN** 200 chunks are pending embedding
- **THEN** 4 batch requests are made to Ollama (50, 50, 50, 50)
- **THEN** each batch returns embeddings matched to chunks by array index

#### Scenario: Batch size configurable
- **WHEN** `embedPendingCodebase()` is called with custom `batchSize=25`
- **THEN** batches are limited to 25 chunks per request
- **THEN** the custom batch size overrides the default of 50
