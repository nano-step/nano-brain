import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { SymbolsPanel } from '../panels/SymbolsPanel'
import type { Symbol } from '../api/types'

const mockSymbols: Symbol[] = [
  { name: 'processQuery', kind: 'function', language: 'go', source_path: 'internal/handlers/query.go', line: 42, signature: 'func processQuery(...)', impact: 18 },
  { name: 'Handler', kind: 'interface', language: 'go', source_path: 'internal/handlers/types.go', line: 11, signature: 'type Handler interface{}', impact: 32 },
  { name: 'useEvents', kind: 'function', language: 'typescript', source_path: 'web/src/hooks/useEvents.ts', line: 12, signature: 'export function useEvents(...)', impact: 5 },
]

vi.mock('../hooks/useSymbols', () => ({
  useSymbols: (params: { kind?: string; language?: string }) => {
    let result = mockSymbols
    if (params.kind && params.kind !== 'all') result = result.filter((s) => s.kind === params.kind)
    if (params.language && params.language !== 'all') result = result.filter((s) => s.language === params.language)
    return { data: result, isLoading: false, error: null }
  },
}))

vi.mock('../api/workspace', () => ({
  getCurrentWorkspace: () => 'ws-abc',
}))

function wrap(ui: React.ReactElement) {
  const qc = new QueryClient()
  return render(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>)
}

describe('SymbolsPanel', () => {
  it('renders symbol table', () => {
    wrap(<SymbolsPanel />)
    expect(screen.getByText('processQuery')).toBeTruthy()
    expect(screen.getByText('Handler')).toBeTruthy()
  })

  it('renders kind chips', () => {
    wrap(<SymbolsPanel />)
    expect(screen.getByRole('button', { name: 'function' })).toBeTruthy()
    expect(screen.getByRole('button', { name: 'interface' })).toBeTruthy()
  })

  it('kind chip "all" is active by default', () => {
    wrap(<SymbolsPanel />)
    const allChip = screen.getAllByText('all')[0]
    expect(allChip.getAttribute('aria-pressed')).toBe('true')
  })

  it('clicking kind chip updates active state', async () => {
    wrap(<SymbolsPanel />)
    const funcChip = screen.getByRole('button', { name: 'function' })
    fireEvent.click(funcChip)
    await waitFor(() => {
      expect(funcChip.getAttribute('aria-pressed')).toBe('true')
    })
  })

  it('sorts by impact desc (Handler 32 > processQuery 18)', () => {
    wrap(<SymbolsPanel />)
    const rows = screen.getAllByRole('row')
    const firstDataRow = rows[1]
    expect(firstDataRow.textContent).toContain('Handler')
  })
})
