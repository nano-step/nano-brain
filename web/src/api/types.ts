export interface Workspace {
  hash: string
  name: string
  root_path: string
  doc_count: number
  chunk_count: number
  created_at: string
}

export interface WorkspacesResponse {
  workspaces: Workspace[]
}

export interface EmbeddingInfo {
  provider: string
  model: string
  dim: number
}

export interface ChunksByEmbedStatus {
  pending: number
  embedded: number
  embed_failed: number
}

export interface GraphEdgesByType {
  contains?: number
  imports?: number
  calls?: number
  references?: number
  [key: string]: number | undefined
}

export interface Collection {
  name: string
  doc_count: number
}

export interface TagCount {
  tag: string
  count: number
}

export interface HarvestInfo {
  mode: string
  last_at: string
  sessions_seen: number
}

export interface WatcherInfo {
  collections_watched: number
  debounce_ms: number
  poll_interval_sec: number
  dirty: number
}

export interface RecentDoc {
  id: string
  title: string
  collection: string
  tags: string[]
  updated_at: string
  supersedes: string | null
  superseded_by: string | null
}

export interface StatsResponse {
  server_version: string
  uptime_sec: number
  embedding: EmbeddingInfo
  migration_version: number
  docs_total: number
  chunks_total: number
  chunks_by_embed_status: ChunksByEmbedStatus
  embeddings_total: number
  graph_edges_by_type: GraphEdgesByType
  collections: Collection[]
  tags_top_20: TagCount[]
  harvest: HarvestInfo
  watcher: WatcherInfo
  recent_docs: RecentDoc[]
}

export interface SSEEvent {
  type: string
  workspace: string
  payload: unknown
  ts: string
}

// ---- Graph types ----

/** Which kind of node to return from /api/v1/graph/neighborhood */
export type NodeKind = 'symbol' | 'doc'

/** Direction filter for neighborhood traversal */
export type GraphDirection = 'in' | 'out' | 'both'

/** Valid edge types */
export type EdgeType = 'contains' | 'imports' | 'calls' | 'references'

/** Request body for POST /api/v1/graph/neighborhood */
export interface GraphNeighborhoodRequest {
  /** Focus node identifier (symbol name or doc UUID) */
  focus: string
  /** Traversal depth (1–5) */
  depth: number
  /** Edge traversal direction */
  direction: GraphDirection
  /** Edge types to include */
  edge_types: EdgeType[]
  /** Workspace hash */
  workspace: string
  /** Determines which collection of nodes to return */
  node_kind: NodeKind
}

/** A single node in the neighborhood graph */
export interface GraphNode {
  /** Unique identifier: symbol name (Code) or doc UUID (Knowledge) */
  id: string
  /** Node kind discriminator */
  kind: NodeKind
  /** Source location — Code mode: "file:line", Knowledge mode: empty */
  source_file?: string
  /** Symbol kind (function/method/type/etc.) — Code mode only */
  symbol_kind?: string
  /** Document title — Knowledge mode only */
  title?: string
  /** Document collection — Knowledge mode only */
  collection?: string
  /** ISO timestamp — Knowledge mode only */
  updated_at?: string
  /** True when this is a hull node (at the boundary of the depth limit) */
  is_frontier?: boolean
}

/** A single directed edge in the neighborhood graph */
export interface GraphEdge {
  /** Source node ID */
  source: string
  /** Target node ID */
  target: string
  /** Edge type */
  edge_type: EdgeType
}

/** Response from POST /api/v1/graph/neighborhood */
export interface GraphNeighborhoodResponse {
  /** node_kind echo'd back */
  node_kind: NodeKind
  /** Nodes in the neighborhood (max 500) */
  nodes: GraphNode[]
  /** Edges between the returned nodes */
  edges: GraphEdge[]
  /** True when the result was truncated at the 500-node cap */
  truncated: boolean
  /** Node IDs sitting on the hull (visible when truncated=true) */
  frontier_nodes: string[]
}

// ---- Memory/Symbols types ----

/** A memory document stored in nano-brain. */
export interface Document {
  id: string
  title: string
  collection: string
  tags: string[]
  updated_at: string
  created_at: string
  supersedes_id: string | null
  superseded_by_id: string | null
  content: string
  metadata: Record<string, unknown>
}

/** A code symbol indexed by nano-brain. */
export interface Symbol {
  name: string
  kind: 'function' | 'method' | 'type' | 'interface' | 'struct' | 'const' | 'var'
  language: string
  source_path: string
  line: number
  signature: string
  impact: number
}

/** A document that references another document via a wikilink. */
export interface Backlink {
  id: string
  title: string
  collection: string
  updated_at: string
  tags: string[]
  snippet: string
}

/** Response from the wikilink resolve endpoint. */
export interface ResolveResponse {
  matched: string[]
  ambiguous: boolean
  kind: 'id' | 'title'
}

export interface EmbedQueuePayload {
  depth: number
  processing: number
}

// ---- Config types ----

/** Resolved config returned by GET /api/v1/config (secrets redacted). */
export interface Config {
  server: {
    host: string
    port: number
    auth?: {
      enabled: boolean
      realm?: string
      bypass_paths?: string[]
    }
  }
  database: {
    url: string
  }
  embedding: {
    provider: string
    url: string
    model: string
    dimension: number
    concurrency: number
    voyage_api_key: string
  }
  harvester: {
    opencode: {
      session_dir: string
      db_path: string
      db_root: string
    }
    claudecode: {
      enabled: boolean
      session_dir: string
    }
  }
  watcher: {
    debounce_ms: number
    reindex_interval: number
  }
  search: {
    rrf_k: number
    recency_weight: number
    recency_half_life_days: number
    limit: number
  }
  storage: {
    max_file_size: number
    max_size: number
  }
  telemetry: {
    retention_days: number
  }
  logging: {
    level: string
    file: string
  }
  summarization: {
    enabled: boolean
    provider_url: string
    api_key: string
    model: string
    max_tokens: number
    concurrency: number
    requests_per_second: number
  }
}

/** Response from GET /api/v1/config. */
export interface ConfigResponse {
  config: Config
  source?: string
}

// ---- Doctor types ----

/** A single prerequisite check result from GET /api/v1/doctor. */
export interface DoctorCheck {
  name: string
  /** "ok" | "fail" | "warn" | "skip" */
  status: string
  detail: string
  hint?: string
}

/** Response from GET /api/v1/doctor. */
export interface DoctorResponse {
  checks: DoctorCheck[]
  all_passed: boolean
}
