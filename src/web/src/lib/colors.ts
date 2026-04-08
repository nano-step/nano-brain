export const typeColorMap: Record<string, string> = {
  tool: '#3b82f6',
  service: '#10b981',
  concept: '#8b5cf6',
  decision: '#f59e0b',
  person: '#ec4899',
  file: '#6366f1',
  library: '#14b8a6',
};

/** Dual-theme palette for entity node pills — separate dark/light values for readability */
export interface TypeColor {
  border: string;
  dark: { bg: string; text: string };
  light: { bg: string; text: string };
}

export const TYPE_COLORS: Record<string, TypeColor> = {
  person:       { border: '#E85D24', dark: { bg: 'rgba(232,93,36,0.15)', text: '#f4a574' },  light: { bg: '#fde8d8', text: '#7a2610' } },
  project:      { border: '#22c55e', dark: { bg: 'rgba(34,197,94,0.15)', text: '#86efac' },  light: { bg: '#dcfce7', text: '#166534' } },
  tool:         { border: '#3b82f6', dark: { bg: 'rgba(59,130,246,0.15)', text: '#93c5fd' },  light: { bg: '#dbeafe', text: '#1e40af' } },
  concept:      { border: '#a78bfa', dark: { bg: 'rgba(167,139,250,0.12)', text: '#c4b5fd' }, light: { bg: '#ede9fe', text: '#5b21b6' } },
  decision:     { border: '#f59e0b', dark: { bg: 'rgba(245,158,11,0.15)', text: '#fcd34d' },  light: { bg: '#fef3c7', text: '#92400e' } },
  service:      { border: '#14b8a6', dark: { bg: 'rgba(20,184,166,0.15)', text: '#5eead4' },  light: { bg: '#ccfbf1', text: '#115e59' } },
  library:      { border: '#ef4444', dark: { bg: 'rgba(239,68,68,0.15)', text: '#fca5a5' },  light: { bg: '#fee2e2', text: '#991b1b' } },
};

export const DEFAULT_TYPE_COLOR: TypeColor = {
  border: '#64748b',
  dark: { bg: 'rgba(100,116,139,0.12)', text: '#cbd5e1' },
  light: { bg: '#f1f5f9', text: '#334155' },
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
