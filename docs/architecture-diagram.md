# nano-brain v2 — System Architecture Diagram

```mermaid
flowchart TD
    subgraph Clients
        A1[AI Agent<br/>OpenCode / Claude Code]
        A2[CLI<br/>nano-brain query]
        A3[MCP Client<br/>SSE / Streamable HTTP]
    end

    subgraph Entry["Entry Layer (Echo v4)"]
        MW[Workspace Middleware<br/>extract + validate workspace_hash]
        HTTP[HTTP Handlers<br/>/api/*]
        MCP[MCP Adapter<br/>/mcp/*]
    end

    subgraph Services["Service Layer"]
        SEARCH[Search Service<br/>hybrid / bm25 / vector]
        HARVEST[Session Harvester<br/>scan + parse sessions]
        WATCHER[File Watcher<br/>fsnotify collections]
        CHUNK[Chunker<br/>900 tokens, 15% overlap]
        EMBED[Embedding Queue<br/>buffered channel 10K]
        COLLECT[Collection Manager<br/>add / remove / list]
    end

    subgraph Data["Data Layer (PostgreSQL 17 + pgvector)"]
        DOCS[(documents)]
        CHUNKS[(chunks)]
        VECTORS[(embeddings<br/>HNSW index)]
        TELEM[(telemetry)]
        META[(collections<br/>metadata)]
    end

    subgraph External["External Services"]
        OLLAMA[Ollama<br/>local embeddings]
        VOYAGE[VoyageAI<br/>cloud embeddings]
    end

    %% Client → Entry
    A1 -->|HTTP POST| MW
    A2 -->|HTTP POST| MW
    A3 -->|SSE / Streamable| MCP

    MW --> HTTP
    MW --> MCP
    MCP --> HTTP

    %% Query flow
    HTTP -->|query request| SEARCH
    SEARCH -->|BM25 fulltext| DOCS
    SEARCH -->|vector similarity| VECTORS
    SEARCH -->|record metrics| TELEM

    %% Ingestion flow
    HTTP -->|write request| CHUNK
    HARVEST -->|scan sessions| CHUNK
    WATCHER -->|file changed| CHUNK
    CHUNK -->|store in tx| DOCS
    CHUNK -->|store in tx| CHUNKS
    CHUNKS -->|chunk_id via channel| EMBED
    EMBED -->|generate vector| OLLAMA
    EMBED -->|generate vector| VOYAGE
    EMBED -->|store vector| VECTORS

    %% Collection management
    HTTP -->|collection CRUD| COLLECT
    COLLECT --> META
    COLLECT -->|configure paths| WATCHER
```

## Flow Summary

### ① Query (Read Path)

```
Client → Middleware (validate workspace) → Search Service
  ├── BM25 fulltext search (tsvector) → documents table
  ├── Vector similarity (pgvector) → embeddings table
  └── RRF merge → response + telemetry recording
```

### ② Ingestion (Write Path)

```
Source (harvest / watcher / API write)
  → Chunker (split 900 tokens, 15% overlap)
  → PostgreSQL transaction (documents + chunks)
  → Embedding queue (async buffered channel 10K)
  → Ollama or VoyageAI → embeddings table
```

### ③ Collection Management

```
CLI / API → Collection Manager → metadata table + configure watcher paths
```

### Key Design Points

- **Every request** passes through Workspace Middleware — no workspace = HTTP 400
- **Query** runs BM25 + Vector in **parallel**, merged via Reciprocal Rank Fusion (RRF)
- **Ingestion** is synchronous (chunk + store) but **embedding is async** via buffered channel
- **3 background goroutines** (harvester, watcher, embed queue) managed by errgroup + context
- **All SQL queries** include `WHERE workspace_hash = $1` — enforced at architecture level
