import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { BacklinksList } from '../components/BacklinksList'
import type { Backlink } from '../api/types'

const backlinksData: { items: Backlink[] } = { items: [] }

vi.mock('../hooks/useBacklinks', () => ({
  useBacklinks: () => ({ data: backlinksData.items, isLoading: false, error: null }),
}))

function wrap(ui: React.ReactElement) {
  const qc = new QueryClient()
  return render(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>)
}

describe('BacklinksList', () => {
  beforeEach(() => {
    backlinksData.items = []
  })

  it('shows empty state message when no backlinks', () => {
    backlinksData.items = []
    wrap(<BacklinksList workspace="ws-abc" docId="d-1042" onOpenDoc={vi.fn()} />)
    expect(screen.getByText(/No other documents reference this yet/)).toBeTruthy()
  })

  it('renders backlink rows when populated', () => {
    backlinksData.items = [
      {
        id: 'd-999',
        title: 'related decision',
        collection: 'memory',
        updated_at: new Date().toISOString(),
        tags: ['decision'],
        snippet: '…mentioned [[d-1042]] in this context…',
      },
    ]
    wrap(<BacklinksList workspace="ws-abc" docId="d-1042" onOpenDoc={vi.fn()} />)
    expect(screen.getByText('related decision')).toBeTruthy()
  })

  it('row click calls onOpenDoc', () => {
    backlinksData.items = [
      {
        id: 'd-999',
        title: 'related decision',
        collection: 'memory',
        updated_at: new Date().toISOString(),
        tags: ['decision'],
        snippet: '…mentioned [[d-1042]] in this context…',
      },
    ]
    const onOpenDoc = vi.fn()
    wrap(<BacklinksList workspace="ws-abc" docId="d-1042" onOpenDoc={onOpenDoc} />)
    const row = screen.getByText('related decision').closest('[role="button"]')
    if (row) fireEvent.click(row)
    expect(onOpenDoc).toHaveBeenCalled()
  })
})


