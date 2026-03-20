const BASE = '/api/v1';

async function requestJson<T>(path: string): Promise<T> {
  const url = `${BASE}${path}`;
  console.log(`[api] → GET ${url}`);
  const t0 = performance.now();
  try {
    const res = await fetch(url);
    const ms = Math.round(performance.now() - t0);
    if (!res.ok) {
      const text = await res.text();
      console.error(`[api] ✗ ${url} ${res.status} (${ms}ms)`, text);
      throw new Error(text || `Request failed: ${res.status}`);
    }
    const data = await res.json() as T;
    console.log(`[api] ✓ ${url} ${res.status} (${ms}ms)`, data);
    return data;
  } catch (err) {
    const ms = Math.round(performance.now() - t0);
    console.error(`[api] ✗ ${url} FAILED (${ms}ms)`, err);
    throw err;
  }
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

export type SymbolNode = {
  id: number;
  name: string;
  kind: string;
  file_path: string;
  start_line: number;
  end_line: number;
  exported: number;
  cluster_id: number | null;
};

export type SymbolEdge = {
  id: number;
  source_id: number;
  target_id: number;
  edge_type: string;
  confidence: number;
};

export type SymbolsResponse = {
  symbols: SymbolNode[];
  edges: SymbolEdge[];
  clusters: Array<{ cluster_id: number; member_count: number }>;
};

export type FlowStep = {
  step_index: number;
  symbol_id: number;
  name: string;
  kind: string;
  file_path: string;
  start_line: number;
};

export type Flow = {
  id: number;
  label: string;
  flow_type: string;
  step_count: number;
  entry_name: string;
  entry_file: string;
  terminal_name: string;
  terminal_file: string;
  steps: FlowStep[];
};

export type FlowsResponse = { flows: Flow[] };

export type DocConnection = {
  id: number;
  from_doc_id: number;
  from_title: string;
  from_path: string;
  to_doc_id: number;
  to_title: string;
  to_path: string;
  relationship_type: string;
  strength: number;
  description: string | null;
  created_at: string;
};

export type ConnectionsResponse = { connections: DocConnection[] };

export type InfraSymbol = {
  type: string;
  pattern: string;
  operation: string;
  repo: string;
  file_path: string;
  line_number: number;
};

export type InfraGroup = {
  pattern: string;
  operations: Array<{ op: string; repo: string; file: string; line: number }>;
};

export type InfrastructureResponse = {
  symbols: InfraSymbol[];
  grouped: Record<string, InfraGroup[]>;
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

export async function fetchSymbols(workspace?: string) {
  const params = workspace ? `?workspace=${workspace}` : '';
  return requestJson<SymbolsResponse>(`/graph/symbols${params}`);
}

export async function fetchFlows(workspace?: string) {
  const params = workspace ? `?workspace=${workspace}` : '';
  return requestJson<FlowsResponse>(`/graph/flows${params}`);
}

export async function fetchConnections(workspace?: string) {
  const params = workspace ? `?workspace=${workspace}` : '';
  return requestJson<ConnectionsResponse>(`/graph/connections${params}`);
}

export async function fetchInfrastructure(workspace?: string) {
  const params = workspace ? `?workspace=${workspace}` : '';
  return requestJson<InfrastructureResponse>(`/graph/infrastructure${params}`);
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
