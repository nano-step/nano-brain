export const typeColorMap: Record<string, string> = {
  tool: '#3b82f6',
  service: '#10b981',
  concept: '#8b5cf6',
  decision: '#f59e0b',
  person: '#ec4899',
  file: '#6366f1',
  library: '#14b8a6',
};

export const fallbackColors = [
  '#38bdf8',
  '#34d399',
  '#f472b6',
  '#facc15',
  '#a78bfa',
  '#f87171',
  '#22d3ee',
  '#c4b5fd',
  '#fb7185',
  '#4ade80',
];

export const symbolKindColorMap: Record<string, string> = {
  function: '#3b82f6',
  method: '#06b6d4',
  class: '#10b981',
  interface: '#8b5cf6',
  type: '#a855f7',
  enum: '#f59e0b',
  variable: '#64748b',
  property: '#94a3b8',
};

export const edgeTypeColorMap: Record<string, string> = {
  CALLS: 'rgba(148, 163, 184, 0.4)',
  INHERITS: 'rgba(249, 115, 22, 0.6)',
  IMPLEMENTS: 'rgba(20, 184, 166, 0.6)',
  REFERENCES: 'rgba(100, 116, 139, 0.3)',
};

export const relationshipColorMap: Record<string, string> = {
  supports: '#10b981',
  contradicts: '#ef4444',
  extends: '#3b82f6',
  supersedes: '#f97316',
  related: '#64748b',
  caused_by: '#eab308',
  refines: '#8b5cf6',
  implements: '#14b8a6',
};

export const infraTypeColorMap: Record<string, string> = {
  redis_key: '#ef4444',
  pubsub_channel: '#f97316',
  mysql_table: '#3b82f6',
  api_endpoint: '#10b981',
  http_call: '#8b5cf6',
  bull_queue: '#eab308',
};
