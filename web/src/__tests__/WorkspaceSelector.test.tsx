import { describe, it, expect, beforeEach, vi } from 'vitest'
import { render, screen, fireEvent, waitFor, act } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { WorkspaceSelector } from '../components/WorkspaceSelector'

vi.mock('../hooks/useWorkspaces', () => ({
  useWorkspaces: () => ({
    data: {
      workspaces: [
        { hash: 'abc123', name: 'nano-brain', root_path: '/projects/nb', doc_count: 100, chunk_count: 1000, created_at: '2026-01-01T00:00:00Z' },
        { hash: 'def456', name: 'frontier', root_path: '/projects/frontier', doc_count: 50, chunk_count: 500, created_at: '2026-01-02T00:00:00Z' },
      ],
    },
    isLoading: false,
  }),
}))

function renderWithQuery(ui: React.ReactElement) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>)
}

describe('WorkspaceSelector', () => {
  beforeEach(() => {
    localStorage.clear()
    localStorage.setItem('nano-brain.workspace', 'abc123')
  })

  it('renders current workspace name', async () => {
    renderWithQuery(<WorkspaceSelector />)
    await waitFor(() => expect(screen.getByText('nano-brain')).toBeTruthy())
  })

  it('shows hash truncated', async () => {
    renderWithQuery(<WorkspaceSelector />)
    await waitFor(() => expect(screen.getByText('abc123')).toBeTruthy())
  })

  it('opens dropdown on click and shows other workspaces', async () => {
    renderWithQuery(<WorkspaceSelector />)
    await waitFor(() => screen.getByRole('button', { name: /Current workspace/i }))

    act(() => fireEvent.click(screen.getByRole('button', { name: /Current workspace/i })))
    await waitFor(() => expect(screen.getByRole('listbox')).toBeTruthy())
    expect(screen.getByText('frontier')).toBeTruthy()
  })

  it('persists selection to localStorage on dropdown item click', async () => {
    renderWithQuery(<WorkspaceSelector />)
    await waitFor(() => screen.getByRole('button', { name: /Current workspace/i }))

    act(() => fireEvent.click(screen.getByRole('button', { name: /Current workspace/i })))
    await waitFor(() => screen.getByRole('listbox'))

    act(() => fireEvent.click(screen.getAllByRole('option')[1]))
    expect(localStorage.getItem('nano-brain.workspace')).toBe('def456')
  })
})
