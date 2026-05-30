import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { HarvestPanel } from '../panels/HarvestPanel'
import type { Document } from '../api/types'

const mockSessions: Document[] = [
  {
    id: 's-001',
    title: 'web-ui plan',
    collection: 'session-summary:opencode',
    tags: ['session', 'opencode'],
    updated_at: new Date().toISOString(),
    created_at: new Date().toISOString(),
    supersedes_id: null,
    superseded_by_id: null,
    content: 'User asked for a Web UI plan. Discovery → OpenSpec → Oracle review.',
    metadata: {},
  },
]

vi.mock('../hooks/useDocuments', () => ({
  useDocuments: () => ({ data: mockSessions, isLoading: false, error: null }),
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

vi.mock('../api/client', () => ({
  apiFetch: vi.fn().mockResolvedValue({ ok: true, json: async () => ({}) }),
}))

function wrap(ui: React.ReactElement) {
  const qc = new QueryClient()
  return render(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>)
}

describe('HarvestPanel', () => {
  it('renders session rows', () => {
    wrap(<HarvestPanel />)
    expect(screen.getByText('web-ui plan')).toBeTruthy()
  })

  it('renders Trigger harvest button', () => {
    wrap(<HarvestPanel />)
    expect(screen.getByText('Trigger harvest')).toBeTruthy()
  })

  it('trigger button calls POST /api/harvest', async () => {
    const { apiFetch } = await import('../api/client')
    wrap(<HarvestPanel />)
    fireEvent.click(screen.getByText('Trigger harvest'))
    await waitFor(() => {
      expect(apiFetch).toHaveBeenCalledWith('/api/harvest', expect.objectContaining({ method: 'POST' }))
    })
  })

  it('button shows Harvesting… when running', async () => {
    wrap(<HarvestPanel />)
    fireEvent.click(screen.getByText('Trigger harvest'))
    await waitFor(() => {
      expect(screen.getByText('Harvesting…')).toBeTruthy()
    })
  })
})
