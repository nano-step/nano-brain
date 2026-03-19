const BASE = '/api/v1';

async function requestJson<T>(path: string): Promise<T> {
  const res = await fetch(`${BASE}${path}`);
  if (!res.ok) {
    const text = await res.text();
    throw new Error(text || `Request failed: ${res.status}`);
  }
  return res.json() as Promise<T>;
}

export type StatusResponse = {
  version: string;
  uptime: number;
  documents: number;
  embeddings: number;
  workspaces: Array<{ path: string; name: string }>;
  primaryWorkspace: string;
};

export type GraphEntity = {
  id: number;
  name: string;
  type: string;
  description?: string | null;
  firstLearnedAt?: string | null;
  lastConfirmedAt?: string | null;
  contradictedAt?: string | null;
};

export type GraphEdge = {
  id: number;
  sourceId: number;
  targetId: number;
  edgeType: string;
  createdAt: string;
};

export type GraphEntitiesResponse = {
  nodes: GraphEntity[];
  edges: GraphEdge[];
  stats: {
    nodeCount: number;
    edgeCount: number;
    typeDistribution: Record<string, number>;
  };
};

export type CodeDependencyResponse = {
  files: Array<{ path: string; centrality: number; clusterId: number | null }>;
  edges: Array<{ source: string; target: string }>;
};

export type TelemetryResponse = {
  queryCount: number;
  banditStats: Record<string, { success: number; failure: number }>;
  preferenceWeights: Record<string, number>;
  expandRate: number;
  importanceStats: {
    min: number;
    max: number;
    mean: number;
    median: number;
  };
};

export type SearchResult = {
  id: string;
  docid: string;
  title: string;
  path: string;
  score: number;
  snippet: string;
  collection: string;
};

export type SearchResponse = {
  results: SearchResult[];
  query: string;
  executionMs: number;
};

export type WorkspacesResponse = {
  workspaces: Array<{ path: string; name: string; hash: string; documentCount: number }>;
};

export async function fetchStatus() {
  return requestJson<StatusResponse>('/status');
}

export async function fetchGraphEntities(workspace?: string) {
  const params = workspace ? `?workspace=${workspace}` : '';
  return requestJson<GraphEntitiesResponse>(`/graph/entities${params}`);
}

export async function fetchCodeDependencies(workspace?: string) {
  const params = workspace ? `?workspace=${workspace}` : '';
  return requestJson<CodeDependencyResponse>(`/code/dependencies${params}`);
}

export async function fetchSearch(query: string, limit = 20, workspace?: string) {
  const params = new URLSearchParams({ q: query, limit: String(limit) });
  if (workspace) {
    params.set('workspace', workspace);
  }
  return requestJson<SearchResponse>(`/search?${params.toString()}`);
}

export async function fetchTelemetry(workspace?: string) {
  const params = workspace ? `?workspace=${workspace}` : '';
  return requestJson<TelemetryResponse>(`/telemetry${params}`);
}

export async function fetchWorkspaces() {
  return requestJson<WorkspacesResponse>('/workspaces');
}
