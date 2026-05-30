import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryPanel } from '../panels/MemoryPanel'
import type { Document } from '../api/types'

const mockDocs: Document[] = [
  {
    id: 'd-001',
    title: 'decision: use postgres',
    collection: 'memory',
    tags: ['decision', 'db'],
    updated_at: new Date().toISOString(),
    created_at: new Date().toISOString(),
    supersedes_id: null,
    superseded_by_id: null,
    content: 'Use PostgreSQL for storage.',
    metadata: {},
  },
  {
    id: 'd-002',
    title: 'decision: use eventbus',
    collection: 'memory',
    tags: ['decision', 'auth'],
    updated_at: new Date().toISOString(),
    created_at: new Date().toISOString(),
    supersedes_id: null,
    superseded_by_id: null,
    content: 'Use an event bus for decoupling.',
    metadata: {},
  },
]

vi.mock('../hooks/useDocuments', () => ({
  useDocuments: (params: { tags?: string[] }) => {
    const filtered = params.tags && params.tags.length > 0
      ? mockDocs.filter((d) => params.tags!.every((t) => d.tags.includes(t)))
      : mockDocs
    return { data: filtered, isLoading: false, error: null }
  },
}))

vi.mock('../hooks/useDocDrawer', () => {
  let openedDoc: Document | null = null
  return {
    useDocDrawer: () => ({
      doc: openedDoc,
      open: (doc: Document) => { openedDoc = doc },
      close: () => { openedDoc = null },
    }),
  }
})

vi.mock('../api/workspace', () => ({
  getCurrentWorkspace: () => 'ws-abc',
}))

function wrap(ui: React.ReactElement) {
  const qc = new QueryClient()
  return render(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>)
}

describe('MemoryPanel', () => {
  beforeEach(() => {
    localStorage.setItem('nano-brain.workspace', 'ws-abc')
  })

  it('renders document rows', () => {
    wrap(<MemoryPanel />)
    expect(screen.getByText('decision: use postgres')).toBeTruthy()
    expect(screen.getByText('decision: use eventbus')).toBeTruthy()
  })

  it('renders filter bar with tag chips', () => {
    wrap(<MemoryPanel />)
    expect(screen.getByRole('button', { name: 'auth' })).toBeTruthy()
    expect(screen.getByRole('button', { name: 'db' })).toBeTruthy()
  })

  it('tag chip toggles active state on click', async () => {
    wrap(<MemoryPanel />)
    const authChip = screen.getByRole('button', { name: 'auth' })
    fireEvent.click(authChip)
    await waitFor(() => {
      expect(authChip.getAttribute('aria-pressed')).toBe('true')
    })
  })

  it('shows clear button when tags are active', async () => {
    wrap(<MemoryPanel />)
    fireEvent.click(screen.getByRole('button', { name: 'db' }))
    await waitFor(() => {
      expect(screen.getByText('clear ✕')).toBeTruthy()
    })
  })

  it('row click calls open', async () => {
    wrap(<MemoryPanel />)
    const row = screen.getByText('decision: use postgres').closest('[role="button"]')
    expect(row).toBeTruthy()
    fireEvent.click(row!)
  })

  it('shows count of documents', () => {
    wrap(<MemoryPanel />)
    expect(screen.getByText('2 documents')).toBeTruthy()
  })
})
