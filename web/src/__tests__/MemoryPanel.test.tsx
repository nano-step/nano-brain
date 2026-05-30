import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { MemoryPanel } from '../panels/MemoryPanel'
import type { Document } from '../api/types'

let mockSearchParams: { tags?: string; doc?: string } = {}
const mockNavigate = vi.fn((opts: { search?: ((p: typeof mockSearchParams) => typeof mockSearchParams) | typeof mockSearchParams }) => {
  if (typeof opts?.search === 'function') {
    mockSearchParams = opts.search(mockSearchParams)
  } else if (opts?.search) {
    mockSearchParams = { ...mockSearchParams, ...opts.search }
  }
})

vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => mockNavigate,
  useSearch: () => mockSearchParams,
}))

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
    mockNavigate.mockReset()
    mockSearchParams = {}
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

  it('tag chip click calls navigate with tags param', async () => {
    wrap(<MemoryPanel />)
    fireEvent.click(screen.getByRole('button', { name: 'auth' }))
    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith(
        expect.objectContaining({ to: '/memory' }),
      )
    })
    const callArg = mockNavigate.mock.calls[0][0] as { search: (p: typeof mockSearchParams) => typeof mockSearchParams }
    const result = callArg.search({})
    expect(result.tags).toContain('auth')
  })

  it('shows clear button when tags URL param is pre-set', () => {
    mockSearchParams = { tags: 'db' }
    wrap(<MemoryPanel />)
    expect(screen.getByRole('button', { name: 'Clear tag filters' })).toBeTruthy()
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

  it('tags from URL search param are pre-selected as active chips', () => {
    mockSearchParams = { tags: 'decision,db' }
    wrap(<MemoryPanel />)
    const dbChip = screen.getByRole('button', { name: 'db' })
    expect(dbChip.getAttribute('aria-pressed')).toBe('true')
  })

  it('clicking a tag chip navigates with updated tags param', async () => {
    mockSearchParams = {}
    wrap(<MemoryPanel />)
    fireEvent.click(screen.getByRole('button', { name: 'auth' }))
    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith(
        expect.objectContaining({ to: '/memory' }),
      )
    })
  })
})
