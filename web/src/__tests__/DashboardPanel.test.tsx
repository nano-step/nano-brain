import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { DashboardPanel } from '../panels/DashboardPanel'
import type { StatsResponse } from '../api/types'

const mockStats: StatsResponse = {
  server_version: '0.42.0',
  uptime_sec: 3600,
  embedding: { provider: 'ollama', model: 'nomic-embed-text', dim: 768 },
  migration_version: 9,
  docs_total: 4218,
  chunks_total: 41277,
  chunks_by_embed_status: { pending: 134, embedded: 40923, embed_failed: 220 },
  embeddings_total: 40923,
  graph_edges_by_type: { contains: 8214, imports: 1907, calls: 24411 },
  collections: [{ name: 'memory', doc_count: 100 }],
  tags_top_20: [{ tag: 'decision', count: 10 }],
  harvest: { mode: 'db_root', last_at: new Date().toISOString(), sessions_seen: 5 },
  watcher: { collections_watched: 3, debounce_ms: 2000, poll_interval_sec: 300, dirty: 0 },
  recent_docs: [
    {
      id: 'd-001',
      title: 'decision: use postgres',
      collection: 'memory',
      tags: ['decision'],
      updated_at: new Date().toISOString(),
      supersedes: null,
      superseded_by: null,
    },
  ],
}

vi.mock('../hooks/useStats', () => ({
  useStats: () => ({ data: mockStats, isLoading: false, error: null }),
}))

vi.mock('../hooks/useEvents', () => ({
  useEvents: vi.fn(),
  useEventStore: (selector: (s: { embedQueueDepth: number }) => number) =>
    selector({ embedQueueDepth: 7 }),
}))

vi.mock('../api/workspace', () => ({
  getCurrentWorkspace: () => 'abc123',
}))

function renderWithQuery(ui: React.ReactElement) {
  const qc = new QueryClient()
  return render(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>)
}

describe('DashboardPanel', () => {
  beforeEach(() => {
    localStorage.setItem('nano-brain.workspace', 'abc123')
  })

  it('renders 6 stat cards', () => {
    renderWithQuery(<DashboardPanel />)
    expect(screen.getByText('Server')).toBeTruthy()
    expect(screen.getByText('Embeddings')).toBeTruthy()
    expect(screen.getByText('Documents')).toBeTruthy()
    expect(screen.getByText('Chunks')).toBeTruthy()
    expect(screen.getByText('Graph edges')).toBeTruthy()
    expect(screen.getByText('Embed queue')).toBeTruthy()
  })

  it('shows server version', () => {
    renderWithQuery(<DashboardPanel />)
    expect(screen.getByText('v0.42.0')).toBeTruthy()
  })

  it('shows live embed queue from SSE store (7)', () => {
    renderWithQuery(<DashboardPanel />)
    expect(screen.getByText('7')).toBeTruthy()
  })

  it('renders recent documents table', () => {
    renderWithQuery(<DashboardPanel />)
    expect(screen.getByText('decision: use postgres')).toBeTruthy()
  })

  it('renders embed status surface', () => {
    renderWithQuery(<DashboardPanel />)
    expect(screen.getByText('Embed status')).toBeTruthy()
    expect(screen.getByText('Graph cardinality')).toBeTruthy()
  })
})
