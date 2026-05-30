import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { DocDrawer } from '../components/DocDrawer'
import type { Document, Backlink } from '../api/types'

const mockDoc: Document = {
  id: 'd-1042',
  title: 'decision: use eventbus pkg',
  collection: 'memory',
  tags: ['decision', 'web-ui'],
  updated_at: new Date().toISOString(),
  created_at: new Date().toISOString(),
  supersedes_id: 'd-0987',
  superseded_by_id: null,
  content: 'Use an eventbus package to avoid import cycles.',
  metadata: {},
}

const mockBacklinks: Backlink[] = [
  {
    id: 'd-999',
    title: 'related decision',
    collection: 'memory',
    updated_at: new Date().toISOString(),
    tags: ['decision'],
    snippet: '…mentioned [[d-1042]] in this context…',
  },
]

vi.mock('../hooks/useBacklinks', () => ({
  useBacklinks: () => ({ data: mockBacklinks, isLoading: false, error: null }),
}))

vi.mock('../hooks/useResolveLinks', () => ({
  useResolveLinks: () => [],
}))

vi.mock('../api/client', () => ({
  apiFetch: vi.fn(),
}))

function wrap(ui: React.ReactElement) {
  const qc = new QueryClient()
  return render(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>)
}

describe('DocDrawer', () => {
  it('renders document title', () => {
    wrap(<DocDrawer doc={mockDoc} workspace="ws-abc" onClose={vi.fn()} onOpenDoc={vi.fn()} />)
    expect(screen.getByText('decision: use eventbus pkg')).toBeTruthy()
  })

  it('renders metadata kv pairs', () => {
    wrap(<DocDrawer doc={mockDoc} workspace="ws-abc" onClose={vi.fn()} onOpenDoc={vi.fn()} />)
    expect(screen.getByText('d-1042')).toBeTruthy()
    expect(screen.getByText('memory')).toBeTruthy()
    expect(screen.getByText('[decision, web-ui]')).toBeTruthy()
  })

  it('renders supersession chain when supersedes_id set', () => {
    wrap(<DocDrawer doc={mockDoc} workspace="ws-abc" onClose={vi.fn()} onOpenDoc={vi.fn()} />)
    expect(screen.getByText('Supersession chain', { exact: false })).toBeTruthy()
    expect(screen.getAllByText(/d-0987/).length).toBeGreaterThan(0)
  })

  it('close button calls onClose', () => {
    const onClose = vi.fn()
    wrap(<DocDrawer doc={mockDoc} workspace="ws-abc" onClose={onClose} onOpenDoc={vi.fn()} />)
    fireEvent.click(screen.getByLabelText('Close drawer'))
    expect(onClose).toHaveBeenCalledOnce()
  })

  it('renders action buttons', () => {
    wrap(<DocDrawer doc={mockDoc} workspace="ws-abc" onClose={vi.fn()} onOpenDoc={vi.fn()} />)
    expect(screen.getByText('Edit (creates supersedes)')).toBeTruthy()
    expect(screen.getByText('Copy ID')).toBeTruthy()
    expect(screen.getByText('Delete…')).toBeTruthy()
  })

  it('renders backlinks section', () => {
    wrap(<DocDrawer doc={mockDoc} workspace="ws-abc" onClose={vi.fn()} onOpenDoc={vi.fn()} />)
    expect(screen.getByText('related decision')).toBeTruthy()
  })
})
