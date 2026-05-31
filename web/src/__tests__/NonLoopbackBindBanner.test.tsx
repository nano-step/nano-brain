import { describe, it, expect, vi } from 'vitest'
import { render } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { NonLoopbackBindBanner } from '../components/NonLoopbackBindBanner'
import type { ConfigResponse } from '../api/types'

function makeConfig(host: string, port = 3100, authEnabled = false): ConfigResponse {
  return {
    config: {
      server: { host, port, auth: { enabled: authEnabled } },
      database: { url: '<redacted>' },
      embedding: { provider: 'ollama', url: '', model: '', dimension: 0, concurrency: 3, voyage_api_key: '<redacted>' },
      harvester: { opencode: { session_dir: '', db_path: '', db_root: '' }, claudecode: { enabled: false, session_dir: '' } },
      watcher: { debounce_ms: 2000, reindex_interval: 300 },
      search: { rrf_k: 60, recency_weight: 0.3, recency_half_life_days: 180, limit: 20 },
      storage: { max_file_size: 0, max_size: 0 },
      telemetry: { retention_days: 90 },
      logging: { level: 'info', file: '' },
      summarization: { enabled: false, provider_url: '', api_key: '<redacted>', model: '', max_tokens: 8000, concurrency: 3, requests_per_second: 0 },
    },
  }
}

const mockApiGetJSON = vi.fn()

vi.mock('../api/client', () => ({
  apiGetJSON: (...args: unknown[]) => mockApiGetJSON(...args),
  apiFetch: vi.fn(),
}))

function renderBanner(cfg: ConfigResponse) {
  mockApiGetJSON.mockResolvedValue(cfg)
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <NonLoopbackBindBanner />
    </QueryClientProvider>,
  )
}

describe('NonLoopbackBindBanner', () => {
  it('shows banner for non-loopback host', async () => {
    const { findByRole } = renderBanner(makeConfig('192.168.1.100', 3100))
    const banner = await findByRole('alert')
    expect(banner.textContent).toContain('192.168.1.100:3100')
    expect(banner.textContent).toContain('without authentication')
  })

  it('shows banner for 0.0.0.0 (public bind)', async () => {
    const { findByRole } = renderBanner(makeConfig('0.0.0.0', 3100))
    const banner = await findByRole('alert')
    expect(banner.textContent).toContain('0.0.0.0:3100')
  })

  it('hides banner for localhost', async () => {
    const { container } = renderBanner(makeConfig('localhost'))
    await new Promise((r) => setTimeout(r, 100))
    expect(container.querySelector('[role="alert"]')).toBeNull()
  })

  it('hides banner for 127.0.0.1', async () => {
    const { container } = renderBanner(makeConfig('127.0.0.1'))
    await new Promise((r) => setTimeout(r, 100))
    expect(container.querySelector('[role="alert"]')).toBeNull()
  })

  it('hides banner for ::1', async () => {
    const { container } = renderBanner(makeConfig('::1'))
    await new Promise((r) => setTimeout(r, 100))
    expect(container.querySelector('[role="alert"]')).toBeNull()
  })

  it('hides banner when auth.enabled=true and host is non-loopback', async () => {
    const { container } = renderBanner(makeConfig('0.0.0.0', 3100, true))
    await new Promise((r) => setTimeout(r, 100))
    expect(container.querySelector('[role="alert"]')).toBeNull()
  })
})
