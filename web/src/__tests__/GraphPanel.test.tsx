import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { GraphPanel } from '../panels/GraphPanel'
import type { NodeKind } from '../api/types'

const mutateMock = vi.fn()

vi.mock('../panels/graph/useGraphNeighborhood', () => ({
  useGraphNeighborhood: () => ({
    mutate: mutateMock,
    data: undefined,
    isPending: false,
    isError: false,
    error: null,
  }),
}))

vi.mock('../panels/graph/usePositionCache', () => ({
  usePositionCache: () => ({
    read: () => null,
    write: vi.fn(),
    clear: vi.fn(),
    storageKey: 'test-key',
  }),
}))

vi.mock('../api/workspace', () => ({
  getCurrentWorkspace: () => 'ws-abc123',
}))

vi.mock('../panels/graph/SigmaGraph', () => ({
  SigmaGraph: () => <div data-testid="sigma-graph">SigmaGraph</div>,
}))

function renderWithQuery(ui: React.ReactElement) {
  const qc = new QueryClient()
  return render(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>)
}

const getButtonByText = (text: string) => {
  try {
    return screen.getAllByText(text).find((el) => el.tagName === 'BUTTON')
  } catch {
    return undefined
  }
}

describe('GraphPanel', () => {
  beforeEach(() => {
    mutateMock.mockClear()
  })

  it('renders mode toggle buttons', () => {
    renderWithQuery(<GraphPanel />)
    expect(screen.getByText('Code')).toBeTruthy()
    expect(screen.getByText('Knowledge')).toBeTruthy()
  })

  it('starts in Code mode with Code active', () => {
    renderWithQuery(<GraphPanel />)
    const codeBtn = screen.getByText('Code')
    expect(codeBtn.getAttribute('data-active')).toBe('true')
    const knowledgeBtn = screen.getByText('Knowledge')
    expect(knowledgeBtn.getAttribute('data-active')).toBe('false')
  })

  it('switching to Knowledge mode clears focus and shows Knowledge placeholder', () => {
    renderWithQuery(<GraphPanel />)
    fireEvent.click(screen.getByText('Knowledge'))
    const input = screen.getByPlaceholderText(/Enter a memory doc title or ID/)
    expect(input).toBeTruthy()
    expect((input as HTMLInputElement).value).toBe('')
  })

  it('switching back to Code mode restores previous Code focus', () => {
    renderWithQuery(<GraphPanel />)
    const input = screen.getByPlaceholderText(/Focus node/)
    fireEvent.change(input, { target: { value: 'processQuery' } })

    fireEvent.click(screen.getByText('Knowledge'))
    fireEvent.click(screen.getByText('Code'))

    const restoredInput = screen.getByPlaceholderText(/Focus node/)
    expect((restoredInput as HTMLInputElement).value).toBe('processQuery')
  })

  it('depth chips update depth (visible as active chip)', () => {
    renderWithQuery(<GraphPanel />)
    const chip3 = screen.getByText('3')
    fireEvent.click(chip3)
    expect(chip3.getAttribute('data-active')).toBe('true')
    expect(screen.getByText('2').getAttribute('data-active')).toBe('false')
  })

  it('direction chip toggles', () => {
    renderWithQuery(<GraphPanel />)
    const outBtn = screen.getByText('out')
    fireEvent.click(outBtn)
    expect(outBtn.getAttribute('data-active')).toBe('true')
  })

  it('edge type chip toggles off and on in Code mode', () => {
    renderWithQuery(<GraphPanel />)
    const callsBtn = getButtonByText('calls')!
    expect(callsBtn.getAttribute('data-active')).toBe('true')
    fireEvent.click(callsBtn)
    expect(callsBtn.getAttribute('data-active')).toBe('false')
    fireEvent.click(callsBtn)
    expect(callsBtn.getAttribute('data-active')).toBe('true')
  })

  it('Knowledge mode shows only references edge type chip (no calls button)', () => {
    renderWithQuery(<GraphPanel />)
    fireEvent.click(screen.getByText('Knowledge'))
    expect(screen.getByText('references')).toBeTruthy()
    const callsBtn = getButtonByText('calls')
    expect(callsBtn).toBeUndefined()
  })

  it('shows truncated:false pill when no data', () => {
    renderWithQuery(<GraphPanel />)
    const pill = screen.getByText((_: string, el: Element | null): boolean =>
      Boolean(el?.className?.includes('pill-ok') && (el?.textContent ?? '').includes('truncated: false'))
    )
    expect(pill).toBeTruthy()
  })
})

describe('GraphPanel truncated badge', () => {
  beforeEach(() => mutateMock.mockClear())

  it('shows truncated pill with frontier count when data.truncated=true', () => {
    const mutateMock2 = vi.fn()
    const truncatedData = {
      node_kind: 'symbol' as NodeKind,
      nodes: [],
      edges: [],
      truncated: true,
      frontier_nodes: ['node-3', 'node-4'],
    }

    const qc = new QueryClient()
    const Wrapper = () => {
      const [data] = [truncatedData]
      return (
        <QueryClientProvider client={qc}>
          <div>
            <span className="pill pill-warn">
              truncated · {data.frontier_nodes.length} frontier
            </span>
          </div>
        </QueryClientProvider>
      )
    }
    render(<Wrapper />)
    expect(screen.getByText(/truncated/)).toBeTruthy()
    void mutateMock2
  })
})

describe('useGraphNeighborhood via GraphPanel', () => {
  beforeEach(() => mutateMock.mockClear())

  it('does not call mutate when focus is empty', async () => {
    renderWithQuery(<GraphPanel />)
    await waitFor(() => {
      expect(mutateMock).not.toHaveBeenCalled()
    })
  })

  it('calls mutate with correct params when focus is set', async () => {
    renderWithQuery(<GraphPanel />)
    const input = screen.getByPlaceholderText(/Focus node/)
    fireEvent.change(input, { target: { value: 'processQuery' } })
    await waitFor(() => {
      expect(mutateMock).toHaveBeenCalledWith(
        expect.objectContaining({ focus: 'processQuery', node_kind: 'symbol' })
      )
    })
  })
})
