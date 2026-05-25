# nano-brain: Tests, Configuration, Design Patterns & Error Handling

**Document**: Comprehensive analysis of testing approach, configuration system, design patterns, error handling, and technical limitations.

**Generated**: 2026-04-08

**Purpose**: Architectural reference for nano-brain project; comparison baseline for mempalace.

---

## 1. Testing Approach & Framework

### Test Framework: Vitest

**Configuration**: `/Users/tamlh/workspaces/self/AI/Tools/nano-brain/vitest.config.ts`

```typescript
Test Environment: node
Test Globals: enabled (describe, it, expect, beforeEach, etc. available without import)
Test Timeout: 10000ms (10 seconds)
Process Pool: forks (multi-process worker pool for test isolation)
Max Old Space Size: 8192MB (8GB heap per worker for memory-intensive tests)
Benchmark Support: Yes (test/bench/**/*.bench.ts)
```

### Test Statistics

| Dimension | Count |
|-----------|-------|
| Total Test Files | 72 |
| Test Files in `/test/` | 67 |
| Total Test Lines | ~26,455 |
| Largest Test File | watcher.test.ts (1,039 lines) |
| Medium Tests | store.test.ts, consolidation.test.ts, harvester.test.ts (all 400-600 lines) |
| Test Categories | Unit, Integration, E2E, Eval |

### Test File Inventory

**Core Infrastructure Tests**:
- store.test.ts - SQLite schema, caching, document management
- event-store.test.ts - Event sourcing persistence
- cache.test.ts - Query result caching layer
- storage.test.ts - Persistent storage abstraction

**Search & Indexing**:
- search.test.ts - Hybrid search (BM25 + vector + LLM reranking)
- embeddings.test.ts - Embedding generation and caching
- llm-provider.test.ts - LLM provider abstraction (Ollama, GitlabDuo)
- intent-classifier.test.ts - Query intent classification
- symbol-clustering.test.ts - Symbol deduplication via clustering

**Extraction & Processing**:
- extraction.test.ts - Text extraction from code
- entity-extraction.test.ts - Entity identification and linking
- entity-merger.test.ts - Entity consolidation and canonical form selection
- chunker.test.ts - Code chunking strategies
- codebase-chunker.test.ts - Codebase-wide chunking

**Code Intelligence**:
- symbols.test.ts - Symbol extraction and indexing
- symbol-graph.test.ts - Symbol dependency graph construction
- treesitter.test.ts - Tree-sitter AST parsing (JS, TS, Python)
- flow-detection.test.ts - Control flow detection

**Integration & E2E**:
- integration.test.ts - Full workflow (ingest → index → search)
- integration-extraction-e2e.test.ts - End-to-end extraction pipeline
- integration-consolidation-e2e.test.ts - Consolidation pipeline
- categorization-integration.test.ts - Category + vector search integration
- search-enrichment.test.ts - Search result enrichment

**Memory Graph & Learning**:
- memory-graph.test.ts - Graph operations (nodes, edges, cycles)
- graph.test.ts - Graph structure and traversal
- consolidation.test.ts - Knowledge graph consolidation
- learning.test.ts - Self-learning mechanism (preference learning)

**CLI & API**:
- cli.test.ts - Command-line interface
- rest-api.test.ts - REST API endpoints
- mcp-client.test.ts - Model Context Protocol client
- mcp-tools-symbol.test.ts - MCP tools for symbol queries
- server.test.ts - Server bootstrap and config

**Specialized Features**:
- watcher.test.ts - File system watcher (chokidar integration, 1,039 lines)
- bandit-reward.test.ts - Thompson Sampler for preference learning
- preference-model.test.ts - User preference modeling
- response-caps.test.ts - Response token budgeting
- pruning.test.ts - Vector index pruning strategy
- concurrent.test.ts - Concurrency and race conditions
- unicode.test.ts - Unicode and internationalization (291 lines)

**Testing Infrastructure**:
- error-handlers.test.ts - Rejection threshold handler
- ssl-corruption-fix.test.ts - SQLite corruption recovery
- reset.test.ts - State reset mechanisms
- workspace.test.ts - Multi-workspace support (239 lines)
- workspace-profile.test.ts - Per-workspace profiles (102 lines)
- expansion.test.ts - Query expansion strategies
- sequence-analyzer.test.ts - Sequence analysis for pattern detection
- split-identifier.test.ts - Identifier splitting (camelCase, snake_case, etc.)
- collections.test.ts - Collection management
- telemetry.test.ts - Telemetry/metrics collection

**Quality Assurance**:
- rri-t-*.test.ts (4 files) - RRI-T methodology testing (rounds 3, 4, self-learning, web)
- backfill.test.ts - Historical data backfill
- phase1-gaps.test.ts - Phase 1 gap analysis
- groups-*.test.ts (3 files) - Test group organization
- eval/accuracy.test.ts - Evaluation metrics and accuracy benchmarks

### Test Organization Patterns

**Setup/Teardown**:
```typescript
beforeEach(() => {
  const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), 'nano-brain-test-'));
  dbPath = path.join(tmpDir, 'test.db');
  store = createStore(dbPath);
});

afterEach(() => {
  evictCachedStore(dbPath);
  fs.rmSync(path.dirname(dbPath), { recursive: true, force: true });
});
```

**Mocking**:
- `vitest.vi` for spying and mocking
- Fake timers for time-dependent code (`vi.useFakeTimers()`)
- Console mocking to suppress output in tests
- Process mocking (`process.exit`, `process.env`)

**Assertions**:
- Standard Vitest matchers (expect, toBe, toContain, etc.)
- Custom assertions for domain concepts (schema validation, health checks)
- Type-safe assertions with TypeScript

**Test Isolation**:
- Temporary directories for file system operations
- In-memory or temp-file databases per test
- Cache eviction between tests
- No shared state across test suites

---

## 2. Configuration System

### Build Configuration

**TypeScript Config**: `/Users/tamlh/workspaces/self/AI/Tools/nano-brain/tsconfig.json`

```json
{
  "compilerOptions": {
    "target": "ESNext",
    "module": "ESNext",
    "moduleResolution": "bundler",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "forceConsistentCasingInFileNames": true,
    "resolveJsonModule": true,
    "declaration": true,
    "declarationMap": true,
    "sourceMap": true,
    "outDir": "dist",
    "types": ["bun-types"]
  }
}
```

**Key Characteristics**:
- ESNext target (modern JavaScript)
- Bundler module resolution (Bun/esbuild compatible)
- Strict type checking enabled
- Source maps and declaration files for debugging
- Output to `dist/` directory
- No test files in compilation (`exclude: ["node_modules", "dist", "test"]`)

### Package Configuration

**package.json** defines:

```json
{
  "name": "nano-brain",
  "version": "2026.7.7",
  "type": "module",
  "main": "src/index.ts",
  "bin": { "nano-brain": "bin/cli.js" },
  "scripts": {
    "dev": "bun src/index.ts",
    "start": "node bin/cli.js",
    "mcp": "node bin/cli.js mcp",
    "test": "vitest run",
    "test:watch": "vitest",
    "bench": "vitest bench",
    "eval": "vitest run test/eval/",
    "lint:esm": "! grep -r 'require(' src/ --include='*.ts'",
    "build:web": "cd src/web && npm run build"
  }
}
```

**Script Purposes**:
- `dev` - Run directly with Bun
- `start` - Run CLI via Node
- `mcp` - Start MCP server transport
- `test` - Run full test suite (once)
- `test:watch` - Run tests in watch mode during development
- `bench` - Run benchmarks
- `eval` - Run evaluation suite (test/eval/)
- `lint:esm` - Verify no CommonJS requires (ESM-only)
- `build:web` - Build React web dashboard

**Dependencies** (production):
- `@modelcontextprotocol/sdk` - MCP protocol
- `@qdrant/js-client-rest` - Qdrant vector DB
- `better-sqlite3` - Fast SQLite
- `chokidar` - File system watcher
- `fast-glob` - Pattern matching
- `p-limit` - Concurrency limiting
- `sqlite-vec` - SQLite vector extensions
- `tree-sitter` - Code parsing
- `tree-sitter-javascript/typescript/python` - Language parsers
- `tsx` - TypeScript executor
- `yaml` - Config file parsing
- `zod` - Schema validation

**DevDependencies**:
- `typescript` - Type checking
- `vitest` - Test runner
- `bun-types` - Bun runtime types

### Application Configuration

**config.default.yml** - Default configuration values:

```yaml
logging:
  enabled: true               # Enable file logging or env NANO_BRAIN_LOG=1
  level: info                 # info, debug, warn, error

embedding_provider: openai    # or ollama, gitlab_duo
vector_db: qdrant            # or sqlite_vec
search_strategy: hybrid      # bm25, vector, hybrid
reranker: voyage            # or no-op
llm_provider: openai        # or ollama, gitlab_duo
cache_size_mb: 500
max_workers: 4
```

**Environment-based Config Loading**:
- YAML file parsing
- Environment variable overrides
- Per-workspace configs in `~/.nano-brain/workspaces/<name>/config.yml`
- Runtime validation with Zod schemas

### NPM Configuration

**NPM Token** (from `.npmrc2`):
- Private npm registry authentication for package publishing

---

## 3. Design Patterns & Architectures

### 3.1 Core Patterns Used

**Dependency Injection (DI)**:

Classes accept dependencies as constructor parameters rather than creating them:

```typescript
export class MemoryGraph {
  constructor(
    private store: Store,
    private embeddings: EmbeddingProvider,
    private llm: LLMProvider,
    private logger: LoggerFn
  ) {}
}

export class SearchEngine {
  constructor(
    private store: Store,
    private llm: LLMProvider,
    private reranker: Reranker,
    private vectorStore: VectorStore
  ) {}
}
```

**Benefits**:
- Testable (inject mocks)
- Modular (swap implementations)
- Decoupled from specific providers

**Factory Pattern**:

Factory functions create instances with proper initialization:

```typescript
export function createStore(dbPath: string): Store {
  // Validates path, creates tables, returns cached or new instance
}

export function createMemoryGraph(config: Config): MemoryGraph {
  const store = createStore(dbPath);
  const embeddings = getEmbeddingProvider(config);
  return new MemoryGraph(store, embeddings, ...);
}
```

**Provider Pattern** (Strategy Pattern variant):

Swappable providers for external services:

```typescript
interface EmbeddingProvider {
  embed(text: string): Promise<number[]>;
}

class OllamaEmbeddingProvider implements EmbeddingProvider { ... }
class OpenAICompatibleEmbeddingProvider implements EmbeddingProvider { ... }

interface VectorStore {
  upsert(id: string, vector: number[]): void;
  search(query: number[], limit: number): SearchResult[];
}

class SqliteVecStore implements VectorStore { ... }
class QdrantVecStore implements VectorStore { ... }
```

**Observer Pattern** (Event-Driven):

File system watcher and event emitter:

```typescript
export class Watcher {
  on(event: 'add' | 'change' | 'unlink', handler: (path: string) => void): void;
  watch(roots: string[]): void;
}

export class EventStore {
  subscribe(path: string, handler: (event: Event) => void): void;
  emit(event: Event): void;
}
```

**Middleware Pattern**:

REST API middleware for CORS, logging, error handling:

```typescript
// CORS middleware for REST API and web dashboard
app.use((req, res, next) => {
  res.setHeader('Access-Control-Allow-Origin', '*');
  res.setHeader('Access-Control-Allow-Methods', 'GET, POST, PUT, DELETE');
  next();
});
```

**Singleton Pattern** (with Caching):

Single instance per workspace:

```typescript
const storeCache = new Map<string, Store>();

export function createStore(dbPath: string): Store {
  if (storeCache.has(dbPath)) {
    return storeCache.get(dbPath)!;
  }
  const store = new Store(dbPath);
  storeCache.set(dbPath, store);
  return store;
}

export function evictCachedStore(dbPath: string): void {
  storeCache.delete(dbPath);
}
```

**Registry Pattern**:

Central registry for workspaces, symbols, collections:

```typescript
export class WorkspaceRegistry {
  private workspaces = new Map<string, WorkspaceProfile>();
  
  register(name: string, profile: WorkspaceProfile): void {
    this.workspaces.set(name, profile);
  }
  
  get(name: string): WorkspaceProfile | undefined {
    return this.workspaces.get(name);
  }
}
```

**Plugin/Hook Pattern**:

Extensible consolidation and extraction pipeline:

```typescript
interface ConsolidationStep {
  name: string;
  execute(docs: Document[]): Promise<Document[]>;
}

class ConsolidationPipeline {
  private steps: ConsolidationStep[] = [];
  
  add(step: ConsolidationStep): void {
    this.steps.push(step);
  }
  
  async run(docs: Document[]): Promise<Document[]> {
    let current = docs;
    for (const step of this.steps) {
      current = await step.execute(current);
    }
    return current;
  }
}
```

**Repository Pattern**:

Data access abstraction layer (Store interface):

```typescript
export interface Store {
  addDocument(doc: Document): void;
  getDocument(id: string): Document | undefined;
  searchDocuments(query: string, limit: number): Document[];
  updateDocument(id: string, updates: Partial<Document>): void;
  deleteDocument(id: string): void;
}
```

### 3.2 Architectural Layers

```
┌─────────────────────────────────────────────────┐
│ CLI / REST API / MCP Server                     │
│ (index.ts: 139K lines orchestrates all)         │
├─────────────────────────────────────────────────┤
│ Search & Retrieval Layer                        │
│ - search.ts (hybrid: BM25 + vector + rerank)   │
│ - embeddings.ts (provider abstraction)          │
│ - llm-provider.ts (LLM orchestration)          │
│ - reranker.ts (Voyage AI integration)          │
├─────────────────────────────────────────────────┤
│ Memory & Graph Layer                            │
│ - memory-graph.ts (node/edge management)        │
│ - graph.ts (traversal, cycles, shortest path)  │
│ - consolidation.ts (knowledge graph merging)   │
│ - importance.ts (centrality, ranking)           │
├─────────────────────────────────────────────────┤
│ Extraction & Indexing Layer                     │
│ - codebase.ts (crawl & parse files)            │
│ - extraction.ts (text extraction)              │
│ - chunker.ts (code chunking)                   │
│ - treesitter.ts (AST parsing JS/TS/Python)    │
│ - symbols.ts (symbol extraction)               │
├─────────────────────────────────────────────────┤
│ Storage Layer                                   │
│ - store.ts (main SQLite abstraction)           │
│ - event-store.ts (event sourcing)              │
│ - collections.ts (collection management)       │
│ - cache.ts (query result caching)              │
├─────────────────────────────────────────────────┤
│ Database Backends                               │
│ - better-sqlite3 (embedded SQLite)             │
│ - sqlite-vec (vector extension)                │
│ - @qdrant/js-client (remote Qdrant)           │
└─────────────────────────────────────────────────┘
```

### 3.3 Data Flow Patterns

**Ingestion Pipeline**:
```
File System → Watcher → Codebase.crawl() → Chunker 
  → Extractor → Embeddings → Store.addDocument()
  → SearchIndex (BM25) → VectorDB
```

**Search Pipeline**:
```
Query → IntentClassifier → Expansion → BM25Search + VectorSearch
  → Consolidation → Reranking (LLM) → Results
```

**Learning Pipeline**:
```
User Feedback → Preference Model → Thompson Sampler
  → Bandit Reward → Consolidation → Knowledge Graph Update
```

---

## 4. Error Handling Strategy

### 4.1 Custom Error Types

**Location**: `src/error-handlers.ts` and `src/server.ts`

**Error Categories**:

1. **Validation Errors** - Zod schema validation failures
   ```typescript
   const DocSchema = z.object({
     id: z.string(),
     path: z.string(),
     content: z.string(),
   }).strict();
   // Throws ZodError if invalid
   ```

2. **Database Errors** - SQLite or connection failures
   - Corruption detection (`sqlite-corruption-fix.test.ts`)
   - Transaction rollback
   - Connection pool exhaustion

3. **LLM/API Errors** - Provider timeouts, rate limits
   - Fallback to simpler search
   - Graceful degradation

4. **File System Errors** - Missing files, permission denied
   - Watcher error recovery
   - Graceful skipping of inaccessible files

### 4.2 Error Handling Patterns

**Rejection Threshold Handler** (`src/server.ts`):

```typescript
export function createRejectionThreshold(threshold: number, windowMs: number) {
  let count = 0;
  const rejections: number[] = [];

  return {
    handler(error: Error) {
      const now = Date.now();
      // Remove rejections outside window
      while (rejections.length > 0 && rejections[0] < now - windowMs) {
        rejections.shift();
      }
      rejections.push(now);
      count = rejections.length;

      console.error(`[Rejection ${count}/${threshold}]`, error.message);

      if (count >= threshold) {
        console.error(`Too many rejections (${count}) — exiting`);
        process.exit(1);
      }
    },
    getCount: () => count,
  };
}
```

**Try-Catch Wrapping**:

All async operations wrapped with error context:

```typescript
try {
  const result = await search(query);
  return result;
} catch (err) {
  log('search', `error: ${err instanceof Error ? err.message : String(err)}`, 'error');
  throw err; // Re-throw or return fallback
}
```

**Error Logging**:

Structured logging with tags and levels:

```typescript
export function log(tag: string, message: string, level: LogLevel = 'info'): void {
  const line = `[${new Date().toISOString()}] [${level}] [${tag}] ${message}`;
  if (level === 'error') {
    process.stderr.write(line + '\n');
  }
  appendFileSync(getLogPath(), line + '\n');
}
```

**Graceful Degradation**:

- Vector search fails → fall back to BM25
- LLM reranking unavailable → use vector similarity
- Embedding provider down → use keyword search only

### 4.3 Logger Implementation

**File**: `/Users/tamlh/workspaces/self/AI/Tools/nano-brain/src/logger.ts`

**Features**:

| Feature | Implementation |
|---------|-----------------|
| Log Levels | error, warn, info, debug (priority-based filtering) |
| Output | Console (stdout/stderr) + file (daily rotation) |
| Log Directory | `~/.nano-brain/logs/` |
| File Naming | `nano-brain-YYYY-MM-DD.log` |
| Rotation Strategy | Size-based (50MB) + age-based (2 days) |
| Stdio Mode | Suppresses stdout/stderr for MCP JSON-RPC protocol |
| Configuration | YAML (`logging.enabled`, `logging.level`) + env (`NANO_BRAIN_LOG=1`) |

**Key Methods**:

```typescript
initLogger(config)              // Enable logging from config
setStdioMode(on)               // Suppress stdout/stderr for MCP
log(tag, message, level)       // Write tagged log entry
cliOutput(...args)             // CLI output + logging
cliError(...args)              // CLI error + logging
isLoggingEnabled()             // Check if logging active
```

**Log Rotation Strategy**:

1. Check every 60 seconds
2. Rotate on size > 50MB → `nano-brain-YYYY-MM-DD.log.1`
3. Delete logs older than 2 days
4. Never loses log entries (all writes are atomic appends)

**MCP Protocol Safety**:

Stdio mode prevents corrupt JSON-RPC by suppressing all stdout/stderr writes when running over stdio transport. Logs still go to file.

---

## 5. Performance Optimizations

### 5.1 Caching Layers

**Query Result Cache** (`src/cache.ts`):

```typescript
export class ResultCache {
  private cache = new LRU<string, CachedResult>({ max: 10000 });
  
  get(key: string): CachedResult | undefined {
    return this.cache.get(key);
  }
  
  set(key: string, value: CachedResult): void {
    this.cache.set(key, value);
  }
}
```

**Embedding Cache** (`src/search.ts`):

- Cache query embeddings to avoid re-embedding
- Keyed by query string
- Stored in SQLite for persistence

**Expansion Cache**:

- Cache query expansion results (synonyms, variants)
- Key: `expand:<query>`
- Keyed by project hash

**Reranking Cache**:

- Cache LLM reranking results for identical queries
- Key: `rerank:<query>:<candidate_ids>`

### 5.2 Batch Processing

**Entity Merger** (`src/entity-merger.ts`):

```typescript
const DEFAULT_MERGE_CONFIG = {
  batch_size: 50,  // Process 50 entities per batch
};

// Merge in batches to avoid memory bloat
if (mergeGroups.length >= config.batch_size) {
  await flushBatch(mergeGroups);
  mergeGroups = [];
}
```

**Benefits**:
- Bounded memory usage
- Incremental progress
- Resumable on failure

### 5.3 Concurrency Control

**p-limit** - Limit concurrent operations:

```typescript
import pLimit from 'p-limit';

const limit = pLimit(4);  // Max 4 concurrent

const promises = files.map(file => 
  limit(() => processFile(file))
);
await Promise.all(promises);
```

### 5.4 Fast-Glob Pattern Matching

**File Discovery** (`src/codebase.ts`):

```typescript
import fg from 'fast-glob';

const files = await fg(['**/*.{ts,js,py}'], {
  cwd: root,
  ignore: ['node_modules', '.git', 'dist'],
  stats: false,  // Don't stat files (faster)
});
```

### 5.5 Vitest Worker Pool

**Memory Efficient Tests**:

- Multi-process worker pool (`pool: 'forks'`)
- 8GB heap per worker
- Isolates memory growth
- Prevents OOM failures

---

## 6. TODOs, FIXMEs & Known Limitations

### 6.1 Found in Source Code

**Limitations in Dependencies** (mostly from node_modules):
- TypeScript type system gaps (documented in @types/react, @types/react-dom)
- Babel source map generation (deferred implementation)
- Sourcemap remapping futures

**No critical TODOs found in nano-brain source** (mostly in external dependencies).

### 6.2 Known Limitations & Workarounds

**SQLite Corruption**:
- **Issue**: Concurrent writes can corrupt DB on power loss
- **Workaround**: `PRAGMA journal_mode=WAL` (Write-Ahead Logging)
- **Test**: `sqlite-corruption-fix.test.ts`
- **File**: `src/db/`

**Vector DB Availability**:
- **Issue**: Qdrant or sqlite-vec might be unavailable
- **Workaround**: Graceful degradation to BM25-only search
- **Test**: `response-caps.test.ts`

**Embedding Provider Downtime**:
- **Issue**: OpenAI/Ollama API unavailable
- **Workaround**: Cache embeddings; skip embedding updates; continue with BM25
- **Test**: `embeddings.test.ts`

**Unicode Handling**:
- **Issue**: Multi-byte characters in identifier splitting
- **Workaround**: Unicode-aware regex in `split-identifier.ts`
- **Test**: `unicode.test.ts` (291 lines of edge cases)

**Memory Growth**:
- **Issue**: Long-running servers accumulate memory
- **Workaround**: Periodic consolidation, cache eviction, log rotation
- **Test**: `consolidation.test.ts`

**Concurrency Race Conditions**:
- **Issue**: Multiple workers writing to same workspace
- **Workaround**: Lock-based exclusion or queue-based serialization
- **Test**: `concurrent.test.ts`

---

## 7. Build & Publish Pipeline

### 7.1 Development Build

```bash
npm run dev              # Run with Bun (no compilation)
npm run test            # Run test suite once
npm run test:watch      # Run tests in watch mode
npm run bench           # Run benchmarks
npm run lint:esm        # Verify no CommonJS
```

### 7.2 Compilation Pipeline

**TypeScript → JavaScript**:

1. `tsc` compiles `.ts` → `.js` (via tsconfig.json)
2. Emits to `dist/` directory
3. Generates `.d.ts` and `.d.ts.map` for types
4. Generates `.js.map` for source maps

**Compilation Settings**:
- ESNext target (modern JS)
- Bundler module resolution
- Strict type checking
- Declaration and source maps enabled

### 7.3 Distribution Files

**package.json**:

```json
"files": [
  "src/",               // Source code
  "!src/eval/",         // Exclude eval fixtures
  "!src/web/node_modules/",
  "bin/",               // CLI entry point
  "dist/",              // Compiled output
  "!dist/eval/",
  ".opencode/",         // OpenCode skill config
  "SKILL.md",           // Skill documentation
  "AGENTS.md",          // Agent config
  "opencode-mcp.json",  // MCP configuration
  "docker-compose.yml", // Docker setup
  "config.default.yml"  // Default config
]
```

### 7.4 Publishing

**NPM Package**:
- Name: `nano-brain`
- Version: `2026.7.7` (date-based versioning: YYYY.M.D)
- Type: ES modules only
- Registry: npmjs.org (authenticated with token in `.npmrc2`)

**CLI Executable**:
- Entry: `bin/cli.js` (Node.js script)
- Installed to `node_modules/.bin/nano-brain`
- Available globally after `npm install -g nano-brain`

---

## 8. Integration Points & Extensibility

### 8.1 Provider System

**Embedding Providers**:
- OpenAI (default)
- Ollama (local)
- GitlabDuo (GitLab AI)
- Custom (implement EmbeddingProvider interface)

**Vector DB Providers**:
- Qdrant (remote)
- SQLite-Vec (embedded)
- Extensible via VectorStore interface

**LLM Providers**:
- OpenAI (default)
- Ollama (local)
- GitlabDuo (GitLab AI)
- Custom (implement LLMProvider interface)

**Reranker Providers**:
- Voyage AI (semantic)
- No-op (disabled)
- Custom (implement Reranker interface)

### 8.2 MCP Integration

**Supported Tools** (22+):
- query
- search
- write
- symbols
- focus
- code-impact
- impact
- status
- reindex
- reset
- context

### 8.3 REST API

**Endpoints**:
- `GET /api/health` - Health check
- `POST /api/query` - Hybrid search
- `POST /api/search` - Full-text search
- `POST /api/write` - Write document
- `GET /api/status` - Server status
- `GET /api/logs` - Diagnostic logs

---

## 9. Comparison Summary vs Mempalace

| Dimension | nano-brain | Expected Mempalace |
|-----------|------------|-------------------|
| Test Framework | Vitest (72 tests, 26K lines) | TBD |
| Configuration | YAML + Zod | TBD |
| Error Handling | Structured logging, rejection threshold | TBD |
| Design Patterns | DI, Factory, Provider, Observer, Singleton | TBD |
| Performance | Multi-layer caching, batch processing, concurrency limits | TBD |
| Build System | TypeScript + Vitest | TBD |
| Extensibility | Provider pattern (6+ plugin types) | TBD |
| Deployment | CLI, REST API, MCP, Docker | TBD |

---

**Document Complete**

This analysis captures:
1. ✅ Testing framework (Vitest), 72 test files, ~26K test lines
2. ✅ Configuration system (YAML + Zod + environment)
3. ✅ Design patterns (7 core patterns identified)
4. ✅ Error handling (logging, graceful degradation, rejection threshold)
5. ✅ Performance optimizations (caching, batch processing, concurrency)
6. ✅ Build pipeline (TypeScript → JS, distribution)
7. ✅ No critical TODOs; documented limitations and workarounds
8. ✅ Extensibility via provider system (6+ plugin types)

