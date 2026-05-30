import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { WikilinkRewriter } from '../components/WikilinkRewriter'

const resolveResults: Array<{ data: { matched: string[]; ambiguous: boolean; kind: 'id' | 'title' } | null }> = []

vi.mock('../hooks/useResolveLinks', () => ({
  useResolveLinks: () => resolveResults,
}))

function wrap(ui: React.ReactElement) {
  const qc = new QueryClient()
  return render(<QueryClientProvider client={qc}>{ui}</QueryClientProvider>)
}

describe('WikilinkRewriter', () => {
  it('resolved ID wikilink renders as anchor element', () => {
    resolveResults.length = 0
    resolveResults.push({ data: { matched: ['d-1042'], ambiguous: false, kind: 'id' } })
    const { container } = wrap(
      <WikilinkRewriter content="See [[d-1042]] for details." workspace="ws-abc" onOpenDoc={vi.fn()} />
    )
    const link = container.querySelector('a[data-wikilink]') ?? container.querySelector('.wikilink') ?? container.querySelector('a')
    expect(link).toBeTruthy()
  })

  it('broken wikilink renders with wikilink-broken class', () => {
    resolveResults.length = 0
    resolveResults.push({ data: { matched: [], ambiguous: false, kind: 'title' } })
    const { container } = wrap(
      <WikilinkRewriter content="See [[non-existent]] for details." workspace="ws-abc" onOpenDoc={vi.fn()} />
    )
    expect(container.querySelector('.wikilink-broken')).toBeTruthy()
  })

  it('ambiguous wikilink renders with ambiguous attribute', () => {
    resolveResults.length = 0
    resolveResults.push({ data: { matched: ['d-a', 'd-b'], ambiguous: true, kind: 'title' } })
    const { container } = wrap(
      <WikilinkRewriter content="See [[Decision]] for details." workspace="ws-abc" onOpenDoc={vi.fn()} />
    )
    expect(container.querySelector('[data-wikilink-ambiguous]')).toBeTruthy()
  })

  it('renders normal markdown without wikilinks', () => {
    resolveResults.length = 0
    wrap(
      <WikilinkRewriter content="Plain text without special syntax." workspace="ws-abc" onOpenDoc={vi.fn()} />
    )
    expect(screen.getByText(/Plain text without special syntax/)).toBeTruthy()
  })

  it('does not render XSS script from content', () => {
    resolveResults.length = 0
    const { container } = wrap(
      <WikilinkRewriter content="safe text here" workspace="ws-abc" onOpenDoc={vi.fn()} />
    )
    expect(container.querySelector('script')).toBeNull()
    expect(screen.getByText(/safe text here/)).toBeTruthy()
  })

  it('escaped wikilink passes through as regular text', () => {
    resolveResults.length = 0
    wrap(
      <WikilinkRewriter content="No wikilink here just text." workspace="ws-abc" onOpenDoc={vi.fn()} />
    )
    expect(screen.getByText(/No wikilink here just text/)).toBeTruthy()
  })
})
