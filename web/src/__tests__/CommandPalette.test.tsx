import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { CommandPalette } from '../components/CommandPalette'

vi.mock('@tanstack/react-router', () => ({
  useRouter: () => ({ navigate: vi.fn() }),
}))

vi.mock('../api/client', () => ({
  apiFetch: vi.fn().mockResolvedValue({ ok: true, json: async () => ({}) }),
  apiGetJSON: vi.fn().mockResolvedValue({ workspaces: [] }),
}))

vi.mock('../api/workspace', () => ({
  getCurrentWorkspace: () => 'ws-abc',
}))

function Wrapper({ children }: { children: React.ReactNode }) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return <QueryClientProvider client={qc}>{children}</QueryClientProvider>
}

function renderPalette() {
  return render(
    <Wrapper>
      <CommandPalette />
    </Wrapper>,
  )
}

describe('CommandPalette', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('does not render when closed', () => {
    renderPalette()
    expect(screen.queryByPlaceholderText(/Type a command/)).toBeNull()
  })

  it('opens on Cmd+K', () => {
    renderPalette()
    fireEvent.keyDown(document, { key: 'k', metaKey: true })
    expect(screen.getByPlaceholderText(/Type a command/)).toBeTruthy()
  })

  it('opens on Ctrl+K', () => {
    renderPalette()
    fireEvent.keyDown(document, { key: 'k', ctrlKey: true })
    expect(screen.getByPlaceholderText(/Type a command/)).toBeTruthy()
  })

  it('closes on Escape', async () => {
    renderPalette()
    fireEvent.keyDown(document, { key: 'k', metaKey: true })
    expect(screen.getByPlaceholderText(/Type a command/)).toBeTruthy()
    fireEvent.keyDown(document, { key: 'Escape' })
    await waitFor(() => {
      expect(screen.queryByPlaceholderText(/Type a command/)).toBeNull()
    })
  })

  it('shows navigation items when open', () => {
    renderPalette()
    fireEvent.keyDown(document, { key: 'k', metaKey: true })
    expect(screen.getByText('Dashboard')).toBeTruthy()
    expect(screen.getByText('Memory')).toBeTruthy()
    expect(screen.getByText('Settings')).toBeTruthy()
  })

  it('shows action items when open', () => {
    renderPalette()
    fireEvent.keyDown(document, { key: 'k', metaKey: true })
    expect(screen.getByText('Trigger reindex')).toBeTruthy()
    expect(screen.getByText('Trigger harvest')).toBeTruthy()
    expect(screen.getByText('Reload config')).toBeTruthy()
  })

  it('filters results when searching', async () => {
    renderPalette()
    fireEvent.keyDown(document, { key: 'k', metaKey: true })
    const input = screen.getByPlaceholderText(/Type a command/)
    fireEvent.change(input, { target: { value: 'reindex' } })
    await waitFor(() => {
      expect(screen.getByText('Trigger reindex')).toBeTruthy()
    })
  })

  it('debounces symbol search — does not call API immediately on keystroke', async () => {
    vi.useFakeTimers()
    const { apiGetJSON } = await import('../api/client')
    const mockGet = vi.mocked(apiGetJSON)
    mockGet.mockClear()

    renderPalette()
    fireEvent.keyDown(document, { key: 'k', metaKey: true })
    const input = screen.getByPlaceholderText(/Type a command/)

    // Type a query — symbol search should NOT fire immediately
    fireEvent.change(input, { target: { value: 'myFunc' } })
    const callsBefore = mockGet.mock.calls.filter((c) => String(c[0]).includes('symbols')).length
    expect(callsBefore).toBe(0)

    // After 150ms debounce window, query client triggers the fetch
    vi.advanceTimersByTime(150)
    vi.useRealTimers()
  })
})
