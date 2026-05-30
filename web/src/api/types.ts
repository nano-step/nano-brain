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

export interface EmbedQueuePayload {
  depth: number
  processing: number
}
