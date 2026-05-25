# nano-brain: Data Models & Storage Analysis

> Generated: 2026-04-08 | Source: src/store.ts, src/types.ts, src/embeddings.ts, src/event-store.ts, src/cache.ts, src/memory-graph.ts, src/fts-client.ts, src/metrics.ts, src/providers/sqlite-vec.ts, src/providers/qdrant.ts, src/vector-store.ts

---

## Table of Contents

## 1. Architecture Overview

nano-brain uses a **per-workspace SQLite database** pattern. Each workspace (project) gets its own `.sqlite` file, named `{dirName}-{hash12}.sqlite` where `hash12` is a 12-character SHA-256 prefix of the workspace path.

Key storage components:

| Component | Technology | File |
|-----------|-----------|------|
| Relational storage | better-sqlite3 (SQLite) | `src/store.ts` |
| Full-text search | SQLite FTS5 (porter + unicode61) | `src/store.ts` |
| Vector search (local) | sqlite-vec extension | `src/providers/sqlite-vec.ts` |
| Vector search (remote) | Qdrant REST API | `src/providers/qdrant.ts` |
| MCP event replay | SQLite table with TTL | `src/event-store.ts` |
| In-memory result cache | TypeScript Map with TTL | `src/cache.ts` |
| LLM response cache | SQLite `llm_cache` table | `src/store.ts` |
| Query embedding cache | SQLite `llm_cache` table (type='qembed') | `src/store.ts` |
| Metrics | In-memory counters + event ring buffer | `src/metrics.ts` |
| Memory graph | SQLite tables (`memory_entities`, `memory_edges`) | `src/memory-graph.ts` |

**Database connection management:**
- Singleton cache per DB path via `storeCache: Map<string, Store>`
- Guard against concurrent initialization via `storeCreating: Set<string>`
- WAL mode for concurrent read/write
- Corruption detection & recovery on open (`checkAndRecoverDB`)
- Cached stores use no-op `close()` — real close via `closeAllCachedStores()`

**Schema version:** Target version is **9**, managed via `PRAGMA user_version`.

## 2. SQLite Schema — Core Tables

### 2.1 `content`

Stores raw document bodies, deduplicated by SHA-256 hash.

```sql
CREATE TABLE IF NOT EXISTS content (
  hash TEXT PRIMARY KEY,
  body TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
```

- **hash**: SHA-256 of the `body` content — serves as content-addressable key
- **body**: Full text content of the document
- **created_at**: ISO 8601 timestamp
- Insert uses `INSERT OR IGNORE` — idempotent, no overwrites

### 2.2 `documents`

Metadata registry for all indexed documents. One row per unique (collection, path).

```sql
CREATE TABLE IF NOT EXISTS documents (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  collection TEXT NOT NULL,
  path TEXT NOT NULL,
  title TEXT NOT NULL,
  hash TEXT NOT NULL,
  agent TEXT,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  modified_at TEXT NOT NULL DEFAULT (datetime('now')),
  active INTEGER NOT NULL DEFAULT 1,
  project_hash TEXT DEFAULT 'global',
  centrality REAL DEFAULT 0.0,
  cluster_id INTEGER DEFAULT NULL,
  superseded_by INTEGER DEFAULT NULL,
  access_count INTEGER DEFAULT 0,
  last_accessed_at TEXT,
  FOREIGN KEY (hash) REFERENCES content(hash),
  UNIQUE(collection, path)
);
```

**Columns:**

| Column | Type | Description |
|--------|------|-------------|
| `id` | INTEGER PK | Auto-incrementing primary key |
| `collection` | TEXT | Collection name (e.g., 'memory', 'codebase', 'sessions') |
| `path` | TEXT | Relative file path within workspace |
| `title` | TEXT | Document title / heading |
| `hash` | TEXT | FK to `content.hash` — content-addressable reference |
| `agent` | TEXT | Optional agent identifier that created this doc |
| `created_at` | TEXT | ISO 8601 creation timestamp |
| `modified_at` | TEXT | ISO 8601 last modification timestamp |
| `active` | INTEGER | 1=active, 0=deactivated (soft delete) |
| `project_hash` | TEXT | 12-char hex workspace identifier, or 'global' |
| `centrality` | REAL | PageRank-style centrality score from file dependency graph |
| `cluster_id` | INTEGER | Cluster assignment from graph analysis |
| `superseded_by` | INTEGER | ID of document that supersedes this one |
| `access_count` | INTEGER | Number of times accessed in search results |
| `last_accessed_at` | TEXT | ISO 8601 timestamp of last access |

**Indexes:**

```sql
CREATE INDEX idx_documents_collection ON documents(collection, active);
CREATE INDEX idx_documents_hash ON documents(hash);
CREATE INDEX idx_documents_path ON documents(path, active);
CREATE INDEX idx_documents_project_hash ON documents(project_hash, active);
CREATE INDEX idx_documents_modified ON documents(modified_at) WHERE active = 1;
CREATE INDEX idx_documents_access ON documents(access_count, last_accessed_at);
```

**UPSERT behavior:** Uses `ON CONFLICT(collection, path) DO UPDATE SET` to update title, hash, agent, modified_at, active, and project_hash on conflict.

**Triggers:** See Section 3 (FTS5) — three triggers keep `documents_fts` in sync with `documents`.

### 2.3 `content_vectors`

Tracks which content hashes have been embedded (embedding metadata, not the vectors themselves).

```sql
CREATE TABLE IF NOT EXISTS content_vectors (
  hash TEXT NOT NULL,
  seq INTEGER NOT NULL DEFAULT 0,
  pos INTEGER NOT NULL DEFAULT 0,
  model TEXT NOT NULL,
  embedded_at TEXT NOT NULL DEFAULT (datetime('now')),
  PRIMARY KEY (hash, seq)
);
```

| Column | Type | Description |
|--------|------|-------------|
| `hash` | TEXT | Content hash from `content` table |
| `seq` | INTEGER | Chunk sequence number (0-based) |
| `pos` | INTEGER | Character position offset in original content |
| `model` | TEXT | Embedding model name used |
| `embedded_at` | TEXT | When the embedding was created |

Pending embeddings are detected via: `LEFT JOIN content_vectors cv ON cv.hash = c.hash WHERE cv.hash IS NULL`

### 2.4 `llm_cache`

General-purpose key-value cache for LLM responses, query embeddings, and edge hashes.

```sql
CREATE TABLE IF NOT EXISTS llm_cache (
  hash TEXT NOT NULL,
  project_hash TEXT NOT NULL DEFAULT 'global',
  type TEXT NOT NULL DEFAULT 'general',
  result TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  PRIMARY KEY (hash, project_hash)
);
```

| Column | Type | Description |
|--------|------|-------------|
| `hash` | TEXT | Cache key (SHA-256 hash or special key like 'edge_hash') |
| `project_hash` | TEXT | Workspace scope, or 'global' |
| `type` | TEXT | Cache type: 'general', 'qembed', 'edge_hash' |
| `result` | TEXT | Cached result (JSON string for embeddings, plain text for others) |
| `created_at` | TEXT | When the cache entry was created |

**Cache types:**
- `general` — LLM response caching
- `qembed` — Query embedding vectors (serialized as JSON number arrays)
- `edge_hash` — File dependency graph hash for change detection

### 2.5 `file_edges`

File-level dependency graph (import relationships).

```sql
CREATE TABLE IF NOT EXISTS file_edges (
  source_path TEXT NOT NULL,
  target_path TEXT NOT NULL,
  edge_type TEXT NOT NULL DEFAULT 'import',
  project_hash TEXT NOT NULL DEFAULT 'global',
  PRIMARY KEY(source_path, target_path, project_hash)
);
CREATE INDEX idx_file_edges_source ON file_edges(source_path);
CREATE INDEX idx_file_edges_target ON file_edges(target_path);
```

Used for PageRank centrality computation and cluster analysis. Paths are stored as relative paths.

### 2.6 `document_tags`

Many-to-many tag association for documents.

```sql
CREATE TABLE IF NOT EXISTS document_tags (
  document_id INTEGER NOT NULL,
  tag TEXT NOT NULL,
  PRIMARY KEY(document_id, tag),
  FOREIGN KEY (document_id) REFERENCES documents(id) ON DELETE CASCADE
);
CREATE INDEX idx_document_tags_tag ON document_tags(tag);
```

Tags are stored lowercase and trimmed. Special prefix `llm:` used for LLM-assigned categorization tags.

### 2.7 `symbols`

Infrastructure symbol tracking (Redis keys, API endpoints, env vars, etc.).

```sql
CREATE TABLE IF NOT EXISTS symbols (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  type TEXT NOT NULL,
  pattern TEXT NOT NULL,
  operation TEXT NOT NULL,
  repo TEXT NOT NULL,
  file_path TEXT NOT NULL,
  line_number INTEGER,
  raw_expression TEXT,
  project_hash TEXT NOT NULL DEFAULT 'global',
  UNIQUE(type, pattern, operation, repo, file_path, line_number)
);
CREATE INDEX idx_symbols_type_pattern ON symbols(type, pattern);
CREATE INDEX idx_symbols_repo ON symbols(repo);
CREATE INDEX idx_symbols_file_project ON symbols(file_path, project_hash);
```

### 2.8 `token_usage`

Tracks embedding API token consumption per model.

```sql
CREATE TABLE IF NOT EXISTS token_usage (
  model TEXT PRIMARY KEY,
  total_tokens INTEGER NOT NULL DEFAULT 0,
  request_count INTEGER NOT NULL DEFAULT 0,
  last_updated TEXT NOT NULL DEFAULT (datetime('now'))
);
```

Uses `ON CONFLICT(model) DO UPDATE` to atomically increment counters.

### 2.9 `path_prefixes` (Schema v9)

Maps project hashes to workspace root paths for path resolution.

```sql
CREATE TABLE IF NOT EXISTS path_prefixes (
  project_hash TEXT PRIMARY KEY,
  prefix TEXT NOT NULL
);
```

Used by `resolvePath()` to convert relative paths back to absolute paths.

## 3. SQLite Schema — FTS5 Virtual Tables

### 3.1 `documents_fts`

Full-text search virtual table using FTS5 with porter stemming and unicode61 tokenizer.

```sql
CREATE VIRTUAL TABLE IF NOT EXISTS documents_fts USING fts5(
  filepath,
  title,
  body,
  tokenize='porter unicode61'
);
```

**FTS5 columns:**

| Column | Content Source |
|--------|---------------|
| `filepath` | `documents.collection || '/' || documents.path` |
| `title` | `documents.title` |
| `body` | `content.body` (joined via `documents.hash`) |

**Tokenizer:** `porter unicode61` — Porter stemming algorithm with Unicode-aware tokenization. Handles multiple languages and strips diacritics.

**Synchronization triggers:**

```sql
-- After INSERT on documents: insert FTS row
CREATE TRIGGER documents_ai AFTER INSERT ON documents BEGIN
  INSERT INTO documents_fts(filepath, title, body)
  SELECT NEW.collection || '/' || NEW.path, NEW.title, c.body
  FROM content c WHERE c.hash = NEW.hash;
END;

-- After DELETE on documents: remove FTS row
CREATE TRIGGER documents_ad AFTER DELETE ON documents BEGIN
  DELETE FROM documents_fts WHERE filepath = OLD.collection || '/' || OLD.path;
END;

-- After UPDATE of hash on documents: replace FTS row
CREATE TRIGGER documents_au AFTER UPDATE OF hash ON documents BEGIN
  DELETE FROM documents_fts WHERE filepath = OLD.collection || '/' || OLD.path;
  INSERT INTO documents_fts(filepath, title, body)
  SELECT NEW.collection || '/' || NEW.path, NEW.title, c.body
  FROM content c WHERE c.hash = NEW.hash;
END;
```

**FTS5 query sanitization** (`sanitizeFTS5Query` in `store.ts`):
1. Split query into whitespace-separated tokens
2. Double-quote each token (escaping internal double-quotes via doubling)
3. Join with `OR` for multi-token queries
4. Result: `"token1" OR "token2"` format

**Search ranking:** Uses `bm25(documents_fts)` for relevance scoring with `snippet()` extraction (64-token window). Results are filtered by `documents.active = 1` and optionally by collection, project_hash, date range, and tags.

## 4. SQLite Schema — Vector Storage

### 4.1 `vectors_vec` (sqlite-vec virtual table)

Vector embeddings stored via the `sqlite-vec` extension using the `vec0` module.

```sql
CREATE VIRTUAL TABLE IF NOT EXISTS vectors_vec USING vec0(
  hash_seq TEXT PRIMARY KEY,
  embedding float[768] distance_metric=cosine
);
```

| Column | Type | Description |
|--------|------|-------------|
| `hash_seq` | TEXT PK | Composite key: `"{content_hash}:{seq}"` |
| `embedding` | float[N] | Float32 vector, default 768 dimensions |

**Key characteristics:**
- **Distance metric:** Cosine similarity (`distance_metric=cosine`)
- **Default dimensions:** 768 (nomic-embed-text). Dynamically adjustable via `ensureVecTable(dimensions)`
- **Dynamic rebuild:** If model changes dimensions, table is dropped and recreated; `content_vectors` + `llm_cache` are cleared to force re-embedding
- **Data format:** Embeddings stored as `Float32Array` — sqlite-vec expects raw float32 buffers
- **Primary key format:** `"{sha256_hash}:{chunk_seq}"` e.g., `"a1b2c3...f4:0"`

**Vector search query pattern:**
```sql
SELECT v.hash_seq, v.distance
FROM vectors_vec v
WHERE v.embedding MATCH ?  -- Float32Array of query embedding
  AND k = ?                -- top-k parameter
ORDER BY v.distance;
```

**Score conversion:** `score = 1 - distance` (cosine distance to cosine similarity)

### 4.2 Consistency Checks

`ensureVecTable(dimensions)` performs:
1. Probe existing table with a zero vector of target dimensions
2. If dimensions mismatch (throws error), drop and recreate with new dimensions
3. If `vectors_vec` is empty but `content_vectors` has rows (and no external vector store), clear `content_vectors` for re-embedding
4. When external vector store (Qdrant) is active, `vectors_vec` being empty is expected

## 5. SQLite Schema — Self-Learning & Telemetry Tables

### 5.1 `search_telemetry`

Records every search query for self-learning and A/B testing.

```sql
CREATE TABLE IF NOT EXISTS search_telemetry (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  query_id TEXT NOT NULL,
  timestamp TEXT NOT NULL DEFAULT (datetime('now')),
  query_text TEXT NOT NULL,
  tier TEXT NOT NULL,
  config_variant TEXT,
  result_docids TEXT NOT NULL DEFAULT '[]',
  expanded_indices TEXT NOT NULL DEFAULT '[]',
  execution_ms INTEGER NOT NULL DEFAULT 0,
  session_id TEXT,
  feedback_signal TEXT NOT NULL DEFAULT 'neutral',
  cache_key TEXT,
  workspace_hash TEXT NOT NULL DEFAULT 'global'
);
CREATE INDEX idx_telemetry_timestamp ON search_telemetry(timestamp);
CREATE INDEX idx_telemetry_session ON search_telemetry(session_id);
CREATE INDEX idx_telemetry_config ON search_telemetry(config_variant);
CREATE INDEX idx_telemetry_cache_key ON search_telemetry(cache_key);
CREATE INDEX idx_telemetry_ws_session_ts ON search_telemetry(workspace_hash, session_id, timestamp);
```

| Column | Type | Description |
|--------|------|-------------|
| `query_id` | TEXT | UUID for this query |
| `query_text` | TEXT | The raw search query |
| `tier` | TEXT | Search tier used |
| `config_variant` | TEXT | A/B test variant identifier |
| `result_docids` | TEXT | JSON array of result document IDs |
| `expanded_indices` | TEXT | JSON array of indices user expanded |
| `execution_ms` | INTEGER | Query execution time in milliseconds |
| `session_id` | TEXT | Session identifier for chain detection |
| `feedback_signal` | TEXT | 'neutral', 'positive' (expanded), or 'negative' (reformulated) |
| `cache_key` | TEXT | Cache key for linking expand events |

**Feedback signals:**
- `neutral` — No interaction after search
- `positive` — User expanded a result (implicit relevance signal)
- `negative` — User reformulated the query (implicit irrelevance signal)

### 5.2 `bandit_stats` (Schema v1)

Thompson sampling statistics for search parameter optimization.

```sql
CREATE TABLE IF NOT EXISTS bandit_stats (
  parameter_name TEXT NOT NULL,
  variant_value REAL NOT NULL,
  successes INTEGER NOT NULL DEFAULT 1,
  failures INTEGER NOT NULL DEFAULT 1,
  workspace_hash TEXT NOT NULL DEFAULT 'global',
  updated_at TEXT NOT NULL DEFAULT (datetime('now')),
  PRIMARY KEY (parameter_name, variant_value, workspace_hash)
);
```

### 5.3 `config_versions` (Schema v1)

Versioned search configuration snapshots.

```sql
CREATE TABLE IF NOT EXISTS config_versions (
  version_id INTEGER PRIMARY KEY AUTOINCREMENT,
  config_json TEXT NOT NULL,
  expand_rate REAL,
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
```

### 5.4 `importance_scores` (Schema v1)

Computed importance scores for documents.

```sql
CREATE TABLE IF NOT EXISTS importance_scores (
  doc_hash TEXT PRIMARY KEY,
  usage_count INTEGER NOT NULL DEFAULT 0,
  entity_density REAL NOT NULL DEFAULT 0.0,
  last_accessed TEXT,
  connection_count INTEGER NOT NULL DEFAULT 0,
  importance_score REAL NOT NULL DEFAULT 0.0,
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
```

### 5.5 `workspace_profiles` (Schema v1)

Per-workspace learning profiles (JSON data).

```sql
CREATE TABLE IF NOT EXISTS workspace_profiles (
  workspace_hash TEXT PRIMARY KEY,
  profile_data TEXT NOT NULL DEFAULT '{}',
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
```

### 5.6 `global_learning` (Schema v1)

Cross-workspace learned parameter values.

```sql
CREATE TABLE IF NOT EXISTS global_learning (
  parameter_name TEXT PRIMARY KEY,
  value REAL NOT NULL,
  confidence REAL NOT NULL DEFAULT 0.0,
  updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
```

## 6. SQLite Schema — Proactive Intelligence Tables

### 6.1 `query_chain_membership` (Schema v2)

Tracks sequences of related queries within a session for chain detection.

```sql
CREATE TABLE IF NOT EXISTS query_chain_membership (
  chain_id TEXT NOT NULL,
  query_id TEXT NOT NULL,
  position INTEGER NOT NULL,
  workspace_hash TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  PRIMARY KEY (chain_id, position)
);
CREATE INDEX idx_chain_workspace ON query_chain_membership(workspace_hash);
```

### 6.2 `query_clusters` (Schema v2)

K-means query clusters for proactive suggestion prediction.

```sql
CREATE TABLE IF NOT EXISTS query_clusters (
  cluster_id INTEGER NOT NULL,
  centroid_embedding TEXT NOT NULL,
  representative_query TEXT NOT NULL,
  query_count INTEGER NOT NULL DEFAULT 0,
  workspace_hash TEXT NOT NULL,
  updated_at TEXT NOT NULL DEFAULT (datetime('now')),
  PRIMARY KEY (cluster_id, workspace_hash)
);
```

| Column | Type | Description |
|--------|------|-------------|
| `cluster_id` | INTEGER | Cluster identifier |
| `centroid_embedding` | TEXT | JSON-serialized centroid vector |
| `representative_query` | TEXT | Most representative query text for this cluster |
| `query_count` | INTEGER | Number of queries assigned to this cluster |

### 6.3 `cluster_transitions` (Schema v2)

Per-workspace Markov chain transition probabilities between query clusters.

```sql
CREATE TABLE IF NOT EXISTS cluster_transitions (
  from_cluster_id INTEGER NOT NULL,
  to_cluster_id INTEGER NOT NULL,
  frequency INTEGER NOT NULL DEFAULT 0,
  probability REAL NOT NULL DEFAULT 0.0,
  workspace_hash TEXT NOT NULL,
  updated_at TEXT NOT NULL DEFAULT (datetime('now')),
  PRIMARY KEY (from_cluster_id, to_cluster_id, workspace_hash)
);
```

### 6.4 `global_transitions` (Schema v2)

Cross-workspace transition probabilities (aggregated from all workspaces).

```sql
CREATE TABLE IF NOT EXISTS global_transitions (
  from_cluster_id INTEGER NOT NULL,
  to_cluster_id INTEGER NOT NULL,
  frequency INTEGER NOT NULL DEFAULT 0,
  probability REAL NOT NULL DEFAULT 0.0,
  updated_at TEXT NOT NULL DEFAULT (datetime('now')),
  PRIMARY KEY (from_cluster_id, to_cluster_id)
);
```

### 6.5 `suggestion_feedback` (Schema v3)

Tracks accuracy of proactive suggestions vs actual next queries.

```sql
CREATE TABLE IF NOT EXISTS suggestion_feedback (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  suggested_query TEXT NOT NULL,
  actual_next_query TEXT NOT NULL,
  match_type TEXT NOT NULL DEFAULT 'none',
  workspace_hash TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
CREATE INDEX idx_suggestion_feedback_workspace ON suggestion_feedback(workspace_hash);
```

| `match_type` values | Description |
|---------------------|-------------|
| `exact` | Suggestion matched actual query exactly |
| `partial` | Partial match detected |
| `none` | No match — suggestion was wrong |

## 7. SQLite Schema — Consolidation Tables

### 7.1 `consolidation_queue` (Schema v4)

Job queue for LLM-powered memory consolidation processing.

```sql
CREATE TABLE IF NOT EXISTS consolidation_queue (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  document_id INTEGER NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending',
  created_at TEXT NOT NULL DEFAULT (datetime('now')),
  processed_at TEXT,
  result TEXT,
  error TEXT
);
CREATE INDEX idx_consolidation_queue_status ON consolidation_queue(status);
```

| Status | Description |
|--------|-------------|
| `pending` | Awaiting processing |
| `processing` | Currently being processed by LLM |
| `completed` | Successfully processed |
| `failed` | Processing failed (error stored in `error` column) |

### 7.2 `consolidation_log` (Schema v4)

Audit trail for all consolidation actions taken on documents.

```sql
CREATE TABLE IF NOT EXISTS consolidation_log (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  document_id INTEGER NOT NULL,
  action TEXT NOT NULL,
  reason TEXT,
  target_doc_id INTEGER,
  model TEXT,
  tokens_used INTEGER DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
```

### 7.3 `consolidations` (Schema v1)

Stores LLM-generated memory consolidation results.

```sql
CREATE TABLE IF NOT EXISTS consolidations (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  source_ids TEXT NOT NULL,
  summary TEXT NOT NULL,
  insight TEXT NOT NULL,
  connections TEXT NOT NULL DEFAULT '[]',
  confidence REAL NOT NULL DEFAULT 0.0,
  retry_count INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT (datetime('now'))
);
```

| Column | Type | Description |
|--------|------|-------------|
| `source_ids` | TEXT | JSON array of document IDs that were consolidated |
| `summary` | TEXT | LLM-generated summary of the consolidated memories |
| `insight` | TEXT | LLM-generated insight from the consolidation |
| `connections` | TEXT | JSON array of discovered connections between documents |
| `confidence` | REAL | Confidence score of the consolidation (0.0-1.0) |
| `retry_count` | INTEGER | Number of retry attempts |

## 8. SQLite Schema — Memory Graph Tables

## 9. SQLite Schema — Memory Connections

## 10. SQLite Schema — Code Intelligence Tables

## 11. SQLite Schema — MCP Event Store

## 12. Schema Versioning & Migrations

## 13. Vector Store Abstraction Layer

## 14. TypeScript Interfaces — Core Data Models

## 15. TypeScript Interfaces — Configuration Models

## 16. TypeScript Interfaces — Search & Telemetry

## 17. TypeScript Interfaces — Memory Graph

## 18. Caching Mechanisms

## 19. Data Serialization Patterns

## 20. Embedding Providers

## 21. FTS Worker Architecture

## 22. Metrics System

## 23. Database Pragmas & Connection Management
