# Design: Vector Store Abstraction

## Architecture

```
┌─────────────────────────────────────────────────────┐
│ nano-brain                                          │
├─────────────────────────────────────────────────────┤
│                                                     │
│  search.ts ──→ VectorStore.search(embedding, opts)  │
│  codebase.ts → VectorStore.upsert(id, embedding)    │
│  store.ts ───→ SQLite (metadata, FTS5, cache)       │
│                                                     │
│  ┌─────────────────────────────────────────────┐    │
│  │ VectorStore (interface)                     │    │
│  │  search(embedding, limit, filters) → Result │    │
│  │  upsert(id, embedding, metadata)            │    │
│  │  delete(id)                                 │    │
│  │  batchUpsert(points[])                      │    │
│  │  health() → { ok, count, provider }         │    │
│  │  close()                                    │    │
│  └──────────┬──────────────┬───────────────────┘    │
│             │              │                        │
│  ┌──────────┴───┐  ┌──────┴──────────┐             │
│  │ SqliteVecStore│  │ QdrantVecStore  │  (future)   │
│  │ (default)     │  │ (opt-in)        │  Pinecone   │
│  │ vectors_vec   │  │ REST API        │  Chroma     │
│  └──────────────┘  └─────────────────┘  Weaviate    │
│                                                     │
└─────────────────────────────────────────────────────┘
```

## VectorStore Interface

```typescript
// src/vector-store.ts

export interface VectorSearchOptions {
  limit?: number;
  collection?: string;
  projectHash?: string;
}

export interface VectorSearchResult {
  hashSeq: string;       // "hash:seq" format
  score: number;         // 0-1 normalized similarity
  hash: string;          // document hash
  seq: number;           // chunk sequence
}

export interface VectorPoint {
  id: string;            // "hash:seq"
  embedding: number[];   // float32 array
  metadata: {
    hash: string;
    seq: number;
    pos: number;
    model: string;
    collection?: string;
    projectHash?: string;
  };
}

export interface VectorStoreHealth {
  ok: boolean;
  provider: string;
  vectorCount: number;
  dimensions?: number;
  error?: string;
}

export interface VectorStore {
  search(embedding: number[], options?: VectorSearchOptions): Promise<VectorSearchResult[]>;
  upsert(point: VectorPoint): Promise<void>;
  batchUpsert(points: VectorPoint[]): Promise<void>;
  delete(id: string): Promise<void>;
  deleteByHash(hash: string): Promise<void>;
  health(): Promise<VectorStoreHealth>;
  close(): Promise<void>;
}
```

## Host Resolution Utility

```typescript
// src/host.ts

import { accessSync, readFileSync } from 'fs';

let _isContainer: boolean | null = null;

export function isInsideContainer(): boolean {
  if (_isContainer !== null) return _isContainer;
  try {
    accessSync('/.dockerenv');
    _isContainer = true;
  } catch {
    try {
      const cgroup = readFileSync('/proc/1/cgroup', 'utf-8');
      _isContainer = cgroup.includes('docker') || cgroup.includes('containerd');
    } catch {
      _isContainer = false;
    }
  }
  return _isContainer;
}

export function resolveHostUrl(url: string): string {
  if (!isInsideContainer()) return url;
  return url
    .replace(/\blocalhost\b/, 'host.docker.internal')
    .replace(/\b127\.0\.0\.1\b/, 'host.docker.internal');
}
```

## Config Schema

```yaml
# ~/.nano-brain/config.yml
vector:
  provider: sqlite-vec       # sqlite-vec | qdrant | (future: pinecone, chroma)
  # Qdrant-specific (ignored for sqlite-vec):
  url: http://localhost:6333  # auto-resolves in containers
  apiKey: ""                  # for Qdrant Cloud or auth-enabled instances
  collection: nano-brain      # Qdrant collection name
```

## Provider: QdrantVecStore

```typescript
// src/providers/qdrant.ts

import { QdrantClient } from '@qdrant/js-client-rest';
import { resolveHostUrl } from '../host.js';
import type { VectorStore, VectorPoint, VectorSearchResult, VectorSearchOptions, VectorStoreHealth } from '../vector-store.js';

export class QdrantVecStore implements VectorStore {
  private client: QdrantClient;
  private collectionName: string;
  private dimensions: number;

  constructor(config: { url: string; apiKey?: string; collection?: string; dimensions?: number }) {
    const resolvedUrl = resolveHostUrl(config.url);
    this.client = new QdrantClient({
      url: resolvedUrl,
      apiKey: config.apiKey || undefined,
    });
    this.collectionName = config.collection || 'nano-brain';
    this.dimensions = config.dimensions || 768;
  }

  async ensureCollection(): Promise<void> {
    // Create collection if not exists, with cosine distance
  }

  async search(embedding: number[], options?: VectorSearchOptions): Promise<VectorSearchResult[]> {
    // POST /collections/{name}/points/query with filters
  }

  async upsert(point: VectorPoint): Promise<void> {
    // Single point upsert
  }

  async batchUpsert(points: VectorPoint[]): Promise<void> {
    // Chunked batch upsert (500 points/batch)
  }

  async delete(id: string): Promise<void> {
    // Delete by point ID
  }

  async deleteByHash(hash: string): Promise<void> {
    // Delete all points where metadata.hash matches
  }

  async health(): Promise<VectorStoreHealth> {
    // GET /collections/{name} → count, status
  }

  async close(): Promise<void> {
    // No-op for REST client
  }
}
```

## Provider: SqliteVecStore

Wraps existing sqlite-vec logic from `store.ts` behind the VectorStore interface. No behavior change — just extraction.

## Factory

```typescript
// src/vector-store.ts

export function createVectorStore(config: VectorConfig, db?: Database): VectorStore {
  switch (config.provider) {
    case 'qdrant':
      return new QdrantVecStore(config);
    case 'sqlite-vec':
    default:
      return new SqliteVecStore(db);
  }
}
```

## Integration Points

### search.ts
- `searchVec()` calls `vectorStore.search(embedding, { limit, collection, projectHash })`
- Results mapped back to `SearchResult` format (JOIN with SQLite documents table for metadata)

### codebase.ts
- `embedPendingCodebase()` calls `vectorStore.batchUpsert(points)` instead of `store.insertEmbedding()`
- Batch size: 500 for Qdrant, existing size for sqlite-vec

### store.ts
- Vector-specific code extracted to `SqliteVecStore`
- `Store` class receives `VectorStore` as constructor dependency
- `searchVec()` on Store delegates to `VectorStore.search()` + metadata JOIN

### embeddings.ts
- `detectOllamaUrl()` refactored to: `resolveHostUrl('http://localhost:11434')`

## Docker Compose

```yaml
# docker-compose.qdrant.yml
# Shipped with nano-brain, stored at ~/.nano-brain/docker-compose.qdrant.yml

services:
  qdrant:
    image: qdrant/qdrant:latest
    container_name: nano-brain-qdrant
    restart: unless-stopped
    ports:
      - "6333:6333"   # REST API
      - "6334:6334"   # gRPC (optional, for future use)
    volumes:
      - nano-brain-qdrant-data:/qdrant/storage
    environment:
      - QDRANT__SERVICE__GRPC_PORT=6334

volumes:
  nano-brain-qdrant-data:
    driver: local
```

## CLI: `qdrant` Subcommand

```
npx nano-brain qdrant <subcommand>

  up        Start Qdrant container (docker compose up -d)
  down      Stop Qdrant container (docker compose down)
  status    Show Qdrant health, collection info, vector count
  migrate   Export vectors from SQLite → Qdrant (zero API cost)
              --workspace=<path>  Migrate specific workspace (default: all)
              --batch-size=<n>    Points per batch (default: 500)
              --dry-run           Show what would be migrated without writing
```

### `qdrant up` Flow
1. Copy `docker-compose.qdrant.yml` to `~/.nano-brain/` if not exists
2. Run `docker compose -f ~/.nano-brain/docker-compose.qdrant.yml up -d`
3. Wait for health check: `GET http://localhost:6333/healthz`
4. Auto-update config.yml: set `vector.provider: qdrant`

### `qdrant down` Flow
1. Run `docker compose -f ~/.nano-brain/docker-compose.qdrant.yml down`
2. Auto-update config.yml: set `vector.provider: sqlite-vec` (fallback)
3. Data persists in Docker volume `nano-brain-qdrant-data`

### `qdrant migrate` Flow
1. Verify Qdrant is running (health check)
2. Create collection if not exists (1024-dim, cosine distance)
3. For each workspace SQLite DB:
   a. Load sqlite-vec extension
   b. Read vectors from `vectors_vec` JOIN `content_vectors` (hash, seq, pos, model, embedding)
   c. Batch upsert into Qdrant (500 points/batch) with metadata payload
   d. Report progress: `[workspace] 21714/21714 vectors migrated`
4. Total: ~49K vectors across 4 DBs, ~100 batches, takes ~30 seconds

### Migration Data Flow
```
SQLite (per workspace DB)                    Qdrant
┌─────────────────────────┐                  ┌──────────────────────────┐
│ vectors_vec             │                  │ Collection: nano-brain   │
│   hash_seq (PK)         │──→ point.id      │   id: "hash:seq"         │
│   embedding float[1024] │──→ point.vector   │   vector: [1024 floats]  │
├─────────────────────────┤                  │   payload:               │
│ content_vectors         │                  │     hash: string         │
│   hash                  │──→ payload.hash   │     seq: number          │
│   seq                   │──→ payload.seq    │     pos: number          │
│   pos                   │──→ payload.pos    │     model: string        │
│   model                 │──→ payload.model  │     projectHash: string  │
│   project_hash          │──→ payload.pHash  │     collection: string   │
└─────────────────────────┘                  └──────────────────────────┘
```

## Migration Path

1. **Start**: `npx nano-brain qdrant up` — starts Qdrant Docker container
2. **Migrate**: `npx nano-brain qdrant migrate` — exports 49K vectors from SQLite → Qdrant (zero API cost, ~30s)
3. **Verify**: `npx nano-brain qdrant status` — shows vector count, health
4. **Use**: Config auto-updated to `vector.provider: qdrant` — search now uses Qdrant
5. **Fallback**: `npx nano-brain qdrant down` — stops Qdrant, auto-falls back to sqlite-vec
6. **Future providers**: Implement `VectorStore` interface, add to factory switch
