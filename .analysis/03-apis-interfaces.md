# nano-brain: External Interfaces & APIs

**Document**: Comprehensive catalog of all external interfaces in nano-brain
**Version**: 1.0
**Date**: 2026-04-08

## Overview

nano-brain exposes three primary interface layers:
1. **CLI Commands** - Command-line interface for direct usage
2. **MCP Tools** - Model Context Protocol tools for LLM integration
3. **REST/HTTP APIs** - HTTP endpoints for server-mode operations

---

## 1. CLI Commands

All commands follow the pattern: `npx nano-brain [global-options] <command> [command-options]`

### Global Options
- `--db=<path>` - SQLite database path (default: `~/.nano-brain/data/default.sqlite`)
- `--config=<path>` - Config YAML path (default: `~/.nano-brain/config.yml`)
- `--help, -h` - Show help
- `--version, -v` - Show version

### Core Commands

#### init
Initialize nano-brain for current workspace
```
nano-brain init [options]
  --root=<path>   Workspace root (default: current directory)
  --force         Clear current workspace memory and re-initialize
  --all           With --force, clear ALL workspaces
```

#### mcp
Start MCP server (default command if no args)
```
nano-brain mcp [options]
  --http          Use HTTP transport instead of stdio
  --port=<n>      HTTP port (default: 8282)
  --host=<addr>   Bind address (default: 127.0.0.1)
  --daemon        Run as background daemon
  stop            Stop running daemon
```

#### status
Show index health, embedding server status, and stats
```
nano-brain status [options]
  --all           Show status for all workspaces
```

#### collection
Manage collections
```
nano-brain collection add <name> <path> [--pattern=<glob>]
nano-brain collection remove <name>
nano-brain collection list
nano-brain collection rename <old> <new>
```

#### embed
Generate embeddings for unembedded chunks
```
nano-brain embed [options]
  --force         Re-embed all chunks
```

#### search, vsearch, query
Full-text, vector, and hybrid search
```
nano-brain search <query> [options]
nano-brain vsearch <query> [options]
nano-brain query <query> [options]
  -n <limit>      Max results (default: 10)
  -c <collection> Filter by collection
  --json          Output as JSON
  --files         Show file paths only
  --compact       Output compact single-line results
  --min-score=<n> Minimum score threshold (query only)
  --scope=all     Search across all workspaces
  --tags=<tags>   Filter by comma-separated tags (AND logic)
  --since=<date>  Filter by modified date (ISO format)
  --until=<date>  Filter by modified date (ISO format)
```

#### tags
List all tags with document counts
```
nano-brain tags
```

#### focus
Show dependency graph context for a file
```
nano-brain focus <filepath>
```

#### graph-stats
Show graph statistics (nodes, edges, clusters, cycles)
```
nano-brain graph-stats
```

#### symbols
Query cross-repo symbols (Redis keys, PubSub, MySQL, APIs, etc.)
```
nano-brain symbols [options]
  --type=<type>      Filter: redis_key, pubsub_channel, mysql_table, api_endpoint, http_call, bull_queue
  --pattern=<pat>    Glob pattern (e.g., "sinv:*")
  --repo=<name>      Filter by repository name
  --operation=<op>   Filter: read, write, publish, subscribe, define, call, produce, consume
  --json             Output as JSON
```

#### impact
Analyze cross-repo impact of a symbol
```
nano-brain impact [options]
  --type=<type>      Symbol type (required)
  --pattern=<pat>    Pattern to analyze (required)
  --json             Output as JSON
```

#### context
360° view of a code symbol (callers, callees, flows)
```
nano-brain context <name> [options]
  --file=<path>      Disambiguate when multiple symbols share the name
  --json             Output as JSON
```

#### code-impact
Analyze impact of changing a symbol
```
nano-brain code-impact <name> [options]
  --direction=<d>    upstream (callers) or downstream (callees) (default: upstream)
  --max-depth=<n>    Max traversal depth (default: 5)
  --min-confidence=<n> Min edge confidence 0-1 (default: 0)
  --file=<path>      Disambiguate symbol
  --json             Output as JSON
```

#### detect-changes
Map git changes to affected symbols and flows
```
nano-brain detect-changes [options]
  --scope=<s>        unstaged, staged, or all (default: all)
  --json             Output as JSON
```

#### reindex
Re-index codebase files and symbol graph
```
nano-brain reindex [options]
  --root=<path>      Workspace root (default: current directory)
```

#### reset
Delete nano-brain data (selective or all)
```
nano-brain reset [options]
  --databases        Delete SQLite workspace databases
  --sessions         Delete harvested session markdown
  --memory           Delete memory notes
  --logs             Delete log files
  --vectors          Delete Qdrant collection vectors
  --confirm          Required to actually delete (safety flag)
  --dry-run          Preview what would be deleted without deleting
```

#### rm
Remove a workspace and all its data
```
nano-brain rm <workspace> [options]
  --list             List all known workspaces
  --dry-run          Preview what would be deleted without deleting
```

#### harvest
Manually trigger session harvesting
```
nano-brain harvest
```

#### cache
Manage LLM cache
```
nano-brain cache clear [options]
  --all              Clear all cache entries across all workspaces
  --type=<type>      Filter by type (embed, expand, rerank)
nano-brain cache stats
```

#### write
Write content to daily log
```
nano-brain write <content> [options]
  --supersedes=<path-or-docid>  Mark a document as superseded
  --tags=<tags>                 Comma-separated tags to associate
```

#### logs
View diagnostic logs
```
nano-brain logs [file] [options]
  path               Print log directory path
  -f, --follow       Follow log output in real-time (tail -f)
  -n <lines>         Show last N lines (default: 50)
  --date=<date>      Show log for specific date (YYYY-MM-DD, default: today)
  --clear            Delete all log files
```

#### bench
Run performance benchmarks
```
nano-brain bench [options]
  --suite=<name>     Run specific suite (search, embed, cache, store)
  --iterations=<n>   Override iteration count
  --json             Output as JSON
  --save             Save results as baseline
  --compare          Compare with last saved baseline
```

#### docker
Manage nano-brain Docker services
```
nano-brain docker start
nano-brain docker stop
nano-brain docker restart [svc]
nano-brain docker status
```

#### qdrant
Manage Qdrant vector store
```
nano-brain qdrant up
nano-brain qdrant down
nano-brain qdrant status
nano-brain qdrant migrate [options]
  --workspace=<path>   Migrate specific workspace only
  --batch-size=<n>     Vectors per batch (default: 500)
  --dry-run            Show counts without migrating
  --activate           Switch to Qdrant provider after migration
nano-brain qdrant verify
nano-brain qdrant activate
nano-brain qdrant cleanup
nano-brain qdrant recreate
```

#### learning
Manage self-learning system
```
nano-brain learning rollback [id]
```

#### categorize-backfill
Backfill LLM categorization on existing documents
```
nano-brain categorize-backfill [options]
  --batch-size=<n>      Documents per batch (default: 50)
  --rate-limit=<n>      Requests per second (default: 10)
  --dry-run             Preview without making changes
  --workspace=<path>    Filter to specific workspace
```

---

## 2. MCP Tools (29 total)

All MCP tools are registered with the ModelContextProtocol server and support async execution.

### Search & Query Tools

#### 2.1 memory_search
**Type**: MCP Tool  
**Description**: BM25 full-text keyword search across indexed documents

**Parameters**:
- `query` (string, required) - Search query
- `limit` (number, optional, default: 10) - Max results
- `collection` (string, optional) - Filter by collection name
- `workspace` (string, optional) - Workspace path, hash, or "all". Required in daemon mode.
- `tags` (string, optional) - Comma-separated tags to filter by (AND logic)
- `since` (string, optional) - Filter documents modified on or after this date (ISO format)
- `until` (string, optional) - Filter documents modified on or before this date (ISO format)
- `compact` (boolean, optional, default: true) - Return compact single-line results with caching

**Returns**: Text with formatted search results or compact cached format

---

#### 2.2 memory_vsearch
**Type**: MCP Tool  
**Description**: Semantic vector search using embeddings

**Parameters**:
- `query` (string, required) - Search query
- `limit` (number, optional, default: 10) - Max results
- `collection` (string, optional) - Filter by collection name
- `workspace` (string, optional) - Workspace path, hash, or "all"
- `tags` (string, optional) - Comma-separated tags to filter by (AND logic)
- `since` (string, optional) - Filter documents modified on or after this date (ISO format)
- `until` (string, optional) - Filter documents modified on or before this date (ISO format)
- `compact` (boolean, optional, default: true) - Return compact single-line results with caching

**Returns**: Text with semantic search results (falls back to FTS if embedder unavailable)

---

#### 2.3 memory_query
**Type**: MCP Tool  
**Description**: Full hybrid search combining BM25 and semantic vectors

**Parameters**:
- `query` (string, required) - Search query
- `limit` (number, optional, default: 10) - Max results
- `collection` (string, optional) - Filter by collection name
- `workspace` (string, optional) - Workspace path, hash, or "all"
- `tags` (string, optional) - Comma-separated tags
- `since` (string, optional) - Filter documents modified on or after this date (ISO format)
- `until` (string, optional) - Filter documents modified on or before this date (ISO format)
- `min_score` (number, optional, default: 0) - Minimum score threshold
- `compact` (boolean, optional, default: true) - Return compact results

**Returns**: Ranked search results from hybrid search

---

#### 2.4 memory_expand
**Type**: MCP Tool  
**Description**: Expand compact search results from cache by index

**Parameters**:
- `cache_key` (string, required) - Cache key from compact results
- `index` (number, required) - 1-based index of result to expand
- `full` (boolean, optional) - Return full document content

**Returns**: Full details of requested result from cache

---

### Document Management Tools

#### 2.5 memory_get
**Type**: MCP Tool  
**Description**: Retrieve a specific document by ID or path

**Parameters**:
- `document_id` (string, required) - Document ID (numeric) or path
- `workspace` (string, optional) - Workspace path, hash, or "all"

**Returns**: Full document with metadata

---

#### 2.6 memory_multi_get
**Type**: MCP Tool  
**Description**: Retrieve multiple documents by ID or path

**Parameters**:
- `ids` (string, required) - Comma-separated document IDs or paths
- `workspace` (string, optional) - Workspace path, hash, or "all"

**Returns**: Array of documents

---

#### 2.7 memory_write
**Type**: MCP Tool  
**Description**: Write content to memory with optional tagging and superseding

**Parameters**:
- `content` (string, required) - Document content (Markdown recommended)
- `tags` (string, optional) - Comma-separated tags to associate
- `supersedes` (string, optional) - Document ID or path this document supersedes
- `collection` (string, optional) - Collection to write to (default: 'memory')

**Returns**: Document metadata with new ID

---

### Metadata & Tagging

#### 2.8 memory_tags
**Type**: MCP Tool  
**Description**: List all tags with document counts

**Parameters**:
- `workspace` (string, optional) - Workspace path, hash, or "all"

**Returns**: List of tags with frequency

---

#### 2.9 memory_status
**Type**: MCP Tool  
**Description**: Show index health, embedding server status, and stats

**Parameters**:
- `workspace` (string, optional) - Workspace path, hash, or "all"

**Returns**: Formatted status report

---

### Indexing & Updates

#### 2.10 memory_index_codebase
**Type**: MCP Tool  
**Description**: Index or re-index codebase files

**Parameters**:
- `workspace` (string, optional) - Workspace root path
- `force` (boolean, optional) - Force re-indexing even if unchanged

**Returns**: Summary of indexed/skipped files

---

#### 2.11 memory_update
**Type**: MCP Tool  
**Description**: Manually trigger index update for collections

**Parameters**:
- `workspace` (string, optional) - Workspace path, hash, or "all"

**Returns**: Update statistics

---

### Symbol & Code Intelligence

#### 2.12 memory_focus
**Type**: MCP Tool  
**Description**: Show dependency graph context for a file or symbol

**Parameters**:
- `file_path` (string, required) - File path to analyze
- `workspace` (string, optional) - Workspace path, hash, or "all"
- `depth` (number, optional, default: 2) - Traversal depth

**Returns**: Dependency graph with incoming/outgoing edges

---

#### 2.13 memory_graph_stats
**Type**: MCP Tool  
**Description**: Show code graph statistics (nodes, edges, clusters, cycles)

**Parameters**:
- `workspace` (string, optional) - Workspace path, hash, or "all"

**Returns**: Graph statistics JSON

---

#### 2.14 memory_symbols
**Type**: MCP Tool  
**Description**: Query cross-repo symbols (Redis keys, APIs, database tables, etc.)

**Parameters**:
- `type` (string, optional) - Symbol type: `redis_key`, `pubsub_channel`, `mysql_table`, `api_endpoint`, `http_call`, `bull_queue`
- `pattern` (string, optional) - Glob pattern (e.g., "sinv:*")
- `repo` (string, optional) - Filter by repository name
- `operation` (string, optional) - Operation type: `read`, `write`, `publish`, `subscribe`, `define`, `call`, `produce`, `consume`
- `workspace` (string, optional) - Workspace path, hash, or "all"

**Returns**: List of symbols matching criteria

---

#### 2.15 memory_impact
**Type**: MCP Tool  
**Description**: Analyze cross-repo impact of a symbol

**Parameters**:
- `type` (string, required) - Symbol type
- `pattern` (string, required) - Pattern to analyze
- `workspace` (string, optional) - Workspace path, hash, or "all"

**Returns**: Impact analysis with affected files and operations

---

#### 2.16 code_context
**Type**: MCP Tool  
**Description**: 360° view of a code symbol (callers, callees, flows)

**Parameters**:
- `symbol_name` (string, required) - Function, class, method, or interface name
- `file_path` (string, optional) - Disambiguate when multiple symbols share the name
- `workspace` (string, optional) - Workspace path, hash, or "all"

**Returns**: Context with call graph, definitions, usage

---

#### 2.17 code_impact
**Type**: MCP Tool  
**Description**: Analyze impact of changing a symbol

**Parameters**:
- `symbol_name` (string, required) - Name to analyze
- `direction` (string, optional, default: "upstream") - `upstream` (callers) or `downstream` (callees)
- `max_depth` (number, optional, default: 5) - Max traversal depth
- `min_confidence` (number, optional, default: 0) - Min edge confidence 0-1
- `file_path` (string, optional) - Disambiguate symbol
- `workspace` (string, optional) - Workspace path, hash, or "all"

**Returns**: Impact analysis showing affected code paths

---

#### 2.18 code_detect_changes
**Type**: MCP Tool  
**Description**: Map git changes to affected symbols and flows

**Parameters**:
- `scope` (string, optional, default: "all") - `unstaged`, `staged`, or `all`
- `workspace` (string, optional) - Workspace path, hash, or "all"

**Returns**: List of changed symbols and impacted flows

---

### Consolidation & Learning

#### 2.19 memory_consolidate
**Type**: MCP Tool  
**Description**: Trigger consolidation to merge and synthesize memories

**Parameters**:
- `workspace` (string, optional) - Workspace path, hash, or "all"
- `batch_size` (number, optional, default: 50) - Documents to consolidate per run

**Returns**: Consolidation results and merged entities

---

#### 2.20 memory_consolidation_status
**Type**: MCP Tool  
**Description**: Check status of memory consolidation

**Parameters**:
- `workspace` (string, optional) - Workspace path, hash, or "all"

**Returns**: Consolidation statistics and pending counts

---

#### 2.21 memory_importance
**Type**: MCP Tool  
**Description**: Calculate and get importance/access metrics for documents

**Parameters**:
- `workspace` (string, optional) - Workspace path, hash, or "all"
- `limit` (number, optional, default: 20) - Top N documents to return

**Returns**: Documents ranked by importance and access patterns

---

#### 2.22 memory_learning_status
**Type**: MCP Tool  
**Description**: Check self-learning system status and config

**Parameters**:
- `workspace` (string, optional) - Workspace path, hash, or "all"

**Returns**: Learning configuration, preferences, and statistics

---

### Suggestions & Related Content

#### 2.23 memory_suggestions
**Type**: MCP Tool  
**Description**: Get AI-powered suggestions for related memories and actions

**Parameters**:
- `query` (string, required) - Input query or topic
- `workspace` (string, optional) - Workspace path, hash, or "all"
- `limit` (number, optional, default: 5) - Number of suggestions

**Returns**: Suggested related memories and next actions

---

### Graph Query & Traversal

#### 2.24 memory_graph_query
**Type**: MCP Tool  
**Description**: Query the knowledge graph using relationships

**Parameters**:
- `query` (string, required) - Graph query (free-text or structured)
- `workspace` (string, optional) - Workspace path, hash, or "all"

**Returns**: Query results with relationship edges

---

#### 2.25 memory_related
**Type**: MCP Tool  
**Description**: Find documents related to a given document or concept

**Parameters**:
- `document_id` (string, optional) - Document ID to find relations for
- `concept` (string, optional) - Concept to find related documents
- `limit` (number, optional, default: 10) - Max results
- `workspace` (string, optional) - Workspace path, hash, or "all"

**Returns**: Related documents ranked by relevance

---

#### 2.26 memory_timeline
**Type**: MCP Tool  
**Description**: Get temporal view of memories (chronological or evolution)

**Parameters**:
- `document_id` (string, optional) - Document ID to trace history
- `start_date` (string, optional) - Start date (ISO format)
- `end_date` (string, optional) - End date (ISO format)
- `workspace` (string, optional) - Workspace path, hash, or "all"

**Returns**: Timeline of documents or evolution of a document

---

#### 2.27 memory_connections
**Type**: MCP Tool  
**Description**: Get explicit connections/relationships between documents

**Parameters**:
- `document_id` (string, required) - Source document ID
- `direction` (string, optional, default: "both") - `incoming`, `outgoing`, or `both`
- `type` (string, optional) - Filter by relationship type
- `workspace` (string, optional) - Workspace path, hash, or "all"

**Returns**: List of connected documents with relationship metadata

---

#### 2.28 memory_traverse
**Type**: MCP Tool  
**Description**: Traverse knowledge graph starting from a document

**Parameters**:
- `start_id` (string, required) - Starting document ID
- `relationship_type` (string, optional) - Filter by relationship type
- `max_depth` (number, optional, default: 3) - Max traversal depth
- `direction` (string, optional, default: "downstream") - `upstream`, `downstream`, or `both`
- `workspace` (string, optional) - Workspace path, hash, or "all"

**Returns**: Traversal graph showing all reachable documents

---

#### 2.29 memory_connect
**Type**: MCP Tool  
**Description**: Create or update explicit connection between documents

**Parameters**:
- `from_document_id` (string, required) - Source document ID
- `to_document_id` (string, required) - Target document ID
- `relationship_type` (string, required) - Type of relationship (see VALID_RELATIONSHIP_TYPES)
- `strength` (number, optional, default: 0.5) - Connection strength (0-1)
- `description` (string, optional) - Human-readable description of connection
- `workspace` (string, optional) - Workspace path, hash, or "all"

**Returns**: Created/updated connection metadata

---

## 3. REST/HTTP APIs

When nano-brain runs with `--http` flag, a full REST API is available.

### Server Configuration
- **Default Port**: 8282
- **Default Host**: 127.0.0.1
- **Base URL**: `http://localhost:8282`

### Core Endpoints

#### GET /health
Health check endpoint

**Response** (200 OK):
```json
{
  "status": "ok",
  "uptime": 3600
}
```

---

#### GET /api/status
Get detailed server and index status

**Query Parameters**:
- `workspace` (optional) - Workspace hash or path

**Response** (200 OK):
```json
{
  "uptime": 3600,
  "ready": true,
  "indexHealth": {
    "documentCount": 1500,
    "embeddedCount": 1400,
    "pendingEmbeddings": 100,
    "databaseSize": 52428800
  }
}
```

---

#### POST /api/search
Full-text search endpoint

**Request Body**:
```json
{
  "query": "authentication",
  "limit": 10,
  "collection": "memory",
  "tags": "security,auth",
  "scope": "workspace"
}
```

**Response** (200 OK):
```json
{
  "results": [
    {
      "id": "123",
      "docid": 123,
      "title": "OAuth Integration",
      "path": "memory/oauth.md",
      "score": 0.95,
      "snippet": "...",
      "collection": "memory"
    }
  ],
  "query": "authentication",
  "executionMs": 45
}
```

---

#### POST /api/query
Hybrid search endpoint (BM25 + vectors)

**Request Body**:
```json
{
  "query": "how to implement caching",
  "limit": 10,
  "min_score": 0.3
}
```

**Response** (200 OK):
```json
{
  "results": [...],
  "query": "how to implement caching",
  "executionMs": 120
}
```

---

#### POST /api/embed
Trigger embedding of pending documents

**Request Body**: (empty)

**Response** (200 OK):
```json
{
  "embedded": 50,
  "remaining": 25,
  "message": "Embedding started in background"
}
```

---

#### GET /api/v1/graph/entities
Get knowledge graph entities

**Query Parameters**:
- `workspace` (optional)

**Response** (200 OK):
```json
{
  "nodes": [
    {
      "id": "entity-1",
      "name": "AuthService",
      "type": "class",
      "description": "...",
      "firstLearnedAt": "2026-04-08T10:00:00Z",
      "lastConfirmedAt": "2026-04-08T14:00:00Z"
    }
  ],
  "edges": [
    {
      "source": "entity-1",
      "target": "entity-2",
      "type": "calls",
      "createdAt": "2026-04-08T10:00:00Z"
    }
  ],
  "stats": {
    "nodeCount": 150,
    "edgeCount": 320,
    "typeDistribution": { "class": 45, "function": 60, ... }
  }
}
```

---

#### GET /api/v1/graph/stats
Get code graph statistics

**Response** (200 OK):
```json
{
  "nodeCount": 500,
  "edgeCount": 1200,
  "clusterCount": 15,
  "cycleCount": 3,
  "avgDegree": 2.4
}
```

---

#### GET /api/v1/code/dependencies
Get file dependency graph

**Response** (200 OK):
```json
{
  "files": [
    {
      "path": "src/auth.ts",
      "centrality": 0.85,
      "clusterId": 1
    }
  ],
  "edges": [
    {
      "source": "src/auth.ts",
      "target": "src/session.ts"
    }
  ]
}
```

---

#### GET /api/v1/search
Fast HTTP search endpoint

**Query Parameters**:
- `q` (required) - Query string
- `limit` (optional, default: 10) - Result limit
- `workspace` (optional) - Workspace hash/path

**Response** (200 OK):
```json
{
  "results": [...],
  "query": "...",
  "executionMs": 35,
  "warning": "search timed out, try again when indexing completes" (optional)
}
```

---

#### GET /api/v1/graph/symbols
Get code symbol graph

**Query Parameters**:
- `workspace` (optional)
- `limit` (optional, default: 2000) - 0 means unlimited
- `kinds` (optional) - Symbol kinds to include (default: excludes 'property')

**Response** (200 OK):
```json
{
  "symbols": [
    {
      "id": 1,
      "name": "processRequest",
      "kind": "function",
      "file": "src/handler.ts",
      "line": 42,
      "exported": true
    }
  ],
  "edges": [
    {
      "sourceId": 1,
      "targetId": 2,
      "type": "calls"
    }
  ],
  "clusters": [...],
  "total": 500,
  "truncated": false
}
```

---

#### GET /api/v1/graph/flows
Get execution flows

**Response** (200 OK):
```json
{
  "flows": [
    {
      "id": 1,
      "label": "Authentication Flow",
      "flowType": "execution",
      "stepCount": 5,
      "entryName": "login",
      "terminalName": "redirectHome",
      "steps": [...]
    }
  ]
}
```

---

#### GET /api/v1/connections
Get document connections

**Query Parameters**:
- `docId` (required) - Document ID
- `direction` (optional, default: "both") - `incoming`, `outgoing`, or `both`

**Response** (200 OK):
```json
{
  "connections": [
    {
      "fromDocId": "doc-1",
      "toDocId": "doc-2",
      "relationshipType": "references",
      "strength": 0.8,
      "description": "Error handling pattern",
      "createdAt": "2026-04-08T10:00:00Z"
    }
  ]
}
```

---

#### GET /api/v1/telemetry
Get telemetry and usage statistics

**Response** (200 OK):
```json
{
  "queryCount": 450,
  "banditStats": {...},
  "preferenceWeights": {...},
  "expandRate": 0.35,
  "importanceStats": {
    "min": 1,
    "max": 450,
    "mean": 45.2,
    "median": 23
  }
}
```

---

#### POST /api/maintenance/prepare
Pause server for maintenance

**Response** (200 OK):
```json
{
  "status": "paused",
  "message": "Server paused for maintenance"
}
```

---

#### POST /api/maintenance/resume
Resume server from maintenance

**Response** (200 OK):
```json
{
  "status": "running",
  "message": "Server resumed"
}
```

---

## 4. Library Exports (TypeScript/JavaScript API)

For programmatic use, nano-brain exports a comprehensive library API.

### Core Exports from `src/index.ts`

#### Store Management
- `createStore(dbPath: string): Promise<Store>` - Create/open database store
- `openDatabase(dbPath: string, options?: OpenOptions): Database` - Low-level database access
- `computeHash(content: string): string` - Compute content hash
- `indexDocument(store, collection, path, content, title, projectHash)` - Index a document

#### Collections
- `loadCollectionConfig(configPath: string): CollectionConfig | null`
- `addCollection(configPath, name, path, pattern)` - Add collection
- `removeCollection(configPath, name)` - Remove collection
- `listCollections(config)` - List all collections
- `getCollections(config): Collection[]`
- `scanCollectionFiles(collection): Promise<string[]>`
- `saveCollectionConfig(configPath, config)`
- `getWorkspaceConfig(config, workspaceRoot)`

#### Search
- `hybridSearch(query, options): Promise<SearchResult[]>` - Full hybrid search
- `parseSearchConfig(raw?): SearchConfig` - Parse search configuration

#### Embeddings
- `createEmbeddingProvider(config): Promise<EmbeddingProvider | null>`
- `detectOllamaUrl(): string`
- `checkOllamaHealth(url): Promise<{ reachable: boolean; models?: string[]; error?: string }>`
- `checkOpenAIHealth(url, apiKey, model): Promise<...>`

#### Codebase Indexing
- `indexCodebase(store, root, config, projectHash, ...): Promise<CodebaseStats>`
- `embedPendingCodebase(store, provider, batchSize): Promise<number>`
- `getCodebaseStats(store, config, root): CodebaseStats | null`

#### Graph Analysis
- `findCycles(store, projectHash): Cycle[]` - Find cycles in dependency graph

#### Vector Store
- `createVectorStore(config): VectorStore` - Create/initialize vector store
- `type VectorStoreHealth` - Health check result type

#### Other
- `harvestSessions(options): Promise<Session[]>` - Harvest OpenCode sessions
- `handleBench(args): Promise<void>` - Run benchmarks
- `resolveHostUrl(): string` - Resolve host URL
- `SymbolGraph` - Symbol graph class

### Key Types

```typescript
interface Store {
  searchFTS(query: string, options: SearchOptions): SearchResult[]
  searchVecAsync(query: string, embedding: number[], options): Promise<SearchResult[]>
  getIndexHealth(): IndexHealth
  getDocumentTags(docId: number): string[]
  getSymbolsForProject(hash: string): Symbol[]
  getSymbolEdgesForProject(hash: string): SymbolEdge[]
  // ... 50+ methods
}

interface SearchResult {
  id: string | number
  docid: number
  title: string
  path: string
  score: number
  snippet: string
  collection?: string
  startLine?: number
  endLine?: number
  tags?: string[]
  symbols?: string[]
  clusterLabel?: string
  flowCount?: number
}

interface IndexHealth {
  documentCount: number
  embeddedCount: number
  pendingEmbeddings: number
  databaseSize: number
  modelStatus: { embedding: string; reranker: string; expander: string }
  collections: Collection[]
  workspaceStats?: Array<{ projectHash: string; count: number }>
  extractedFacts?: number
}
```

---

## 5. Configuration & Environment Variables

### Configuration File (~/.nano-brain/config.yml)

```yaml
embedding:
  provider: ollama | openai | local
  url: http://localhost:11434
  model: nomic-embed-text
  apiKey: (optional, for OpenAI)

vector:
  provider: sqlite-vec | qdrant
  url: http://localhost:6334 (for Qdrant)

logging:
  enabled: true

watcher:
  pollIntervalMs: 120000
  sessionPollMs: 120000
  embedIntervalMs: 60000

workspaces:
  /path/to/project:
    codebase:
      enabled: true
      extensions: [".ts", ".js"]
      maxSize: "1GB"
      excludePatterns: ["node_modules/**", "dist/**"]

collections:
  memory:
    path: ~/.nano-brain/memory
    pattern: "**/*.md"
    update: auto
  sessions:
    path: ~/.nano-brain/sessions
    pattern: "**/*.md"
    update: auto

storage:
  maxSize: "10GB"
  retention: "30d"
  minFreeDisk: "500MB"

learning:
  enabled: true
  model: gpt-4
  batchSize: 50

consolidation:
  enabled: true
  interval: "1h"
  model: ollama:mistral

categorization:
  enabled: true
  model: ollama:neural-chat

telemetry:
  enabled: true
  trackExpansions: true
  trackPreferences: true
```

### Environment Variables

- `NANO_BRAIN_HOME` - Override default home directory
- `NANO_BRAIN_DB_PATH` - Override database path
- `NANO_BRAIN_CONFIG` - Override config file path
- `NANO_BRAIN_PORT` - HTTP server port
- `NANO_BRAIN_HOST` - HTTP server bind address
- `NANO_BRAIN_LOG` - Enable logging (0/1)
- `OLLAMA_URL` - Override Ollama endpoint
- `OPENAI_API_KEY` - OpenAI API key
- `QDRANT_URL` - Qdrant vector store URL

---

## 6. Plugin & Extension System

### No built-in plugin system, but extensibility via:

1. **Collections** - Add custom collections via config
2. **Custom Commands** - Bash-based command wrappers at `~/.opencode/command/`
3. **LLM Providers** - Pluggable embedding/categorization models
4. **Vector Stores** - SQLite-vec or Qdrant (configurable)
5. **Search Providers** - Swap BM25/vector/hybrid implementations

---

## 7. Summary Table

| Layer | Type | Count | Transport |
|-------|------|-------|-----------|
| CLI | Commands | 25+ | stdout/files |
| MCP | Tools | 29 | Stdio or HTTP |
| REST | API Endpoints | 10+ | HTTP JSON |
| Library | Exported Functions | 50+ | TypeScript imports |

---

## 8. Comparison with Mempalace

(To be filled in after mempalace analysis)

- nano-brain: **MCP-first**, REST-secondary, strong CLI, persistent indexing
- mempalace: (pending analysis)

