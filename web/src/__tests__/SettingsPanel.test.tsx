import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { SettingsPanel } from '../panels/SettingsPanel'
import type { ConfigResponse, DoctorResponse } from '../api/types'

const mockConfig: ConfigResponse = {
  config: {
    server: { host: 'localhost', port: 3100 },
    database: { url: '<redacted>' },
    embedding: { provider: 'ollama', url: 'http://localhost:11434', model: 'nomic-embed-text', dimension: 768, concurrency: 3, voyage_api_key: '<redacted>' },
    harvester: { opencode: { session_dir: '', db_path: '', db_root: '' }, claudecode: { enabled: false, session_dir: '' } },
    watcher: { debounce_ms: 2000, reindex_interval: 300 },
    search: { rrf_k: 60, recency_weight: 0.3, recency_half_life_days: 180, limit: 20 },
    storage: { max_file_size: 314572800, max_size: 10737418240 },
    telemetry: { retention_days: 90 },
    logging: { level: 'info', file: '' },
    summarization: { enabled: false, provider_url: '', api_key: '<redacted>', model: 'nano-brain', max_tokens: 8000, concurrency: 3, requests_per_second: 0 },
  },
}

const mockDoctor: DoctorResponse = {
  checks: [
    { name: 'Config', status: 'ok', detail: '~/.nano-brain/config.yml' },
    { name: 'PostgreSQL', status: 'ok', detail: 'connected' },
    { name: 'pgvector', status: 'fail', detail: 'extension not found', hint: 'Run CREATE EXTENSION vector' },
  ],
  all_passed: false,
}

const mockApiFetch = vi.fn().mockResolvedValue({ ok: true, json: async () => ({ status: 'patched' }) })
const mockApiGetJSON = vi.fn()

vi.mock('../api/client', () => ({
  apiGetJSON: (...args: unknown[]) => mockApiGetJSON(...args),
  apiFetch: (...args: unknown[]) => mockApiFetch(...args),
}))

vi.mock('../api/workspace', () => ({
  getCurrentWorkspace: () => 'ws-test-hash',
}))

function renderPanel() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <SettingsPanel />
    </QueryClientProvider>,
  )
}

describe('SettingsPanel', () => {
  beforeEach(() => {
    mockApiGetJSON.mockImplementation((path: string) => {
      if (path === '/api/v1/config') return Promise.resolve(mockConfig)
      if (path === '/api/v1/doctor') return Promise.resolve(mockDoctor)
      return Promise.resolve({})
    })
    mockApiFetch.mockResolvedValue({ ok: true, json: async () => ({ status: 'patched' }) })
  })

  it('renders settings title', async () => {
    renderPanel()
    expect(await screen.findByText('Settings')).toBeTruthy()
  })

  it('renders form with config values', async () => {
    renderPanel()
    const input = await screen.findByDisplayValue('60')
    expect(input).toBeTruthy()
  })

  it('renders secrets as read-only redacted', async () => {
    renderPanel()
    const redacted = await screen.findAllByDisplayValue('<redacted>')
    expect(redacted.length).toBeGreaterThan(0)
    redacted.forEach((el) => expect(el).toBeDisabled())
  })

  it('renders doctor checks', async () => {
    renderPanel()
    expect(await screen.findByText('Config')).toBeTruthy()
    expect(await screen.findByText('PostgreSQL')).toBeTruthy()
    expect(await screen.findByText('pgvector')).toBeTruthy()
  })

  it('shows hint button for failed check', async () => {
    renderPanel()
    const btn = await screen.findByRole('button', { name: /hint/ })
    expect(btn).toBeTruthy()
  })

  it('shows hint text when expanded', async () => {
    renderPanel()
    const btn = await screen.findByRole('button', { name: /hint/ })
    fireEvent.click(btn)
    expect(screen.getByText('Run CREATE EXTENSION vector')).toBeTruthy()
  })

  it('save button is disabled when form is pristine', async () => {
    renderPanel()
    const saveBtn = await screen.findByRole('button', { name: 'Save config' })
    expect(saveBtn).toBeDisabled()
  })

  it('save button enables after changing a field', async () => {
    renderPanel()
    const rrfInput = await screen.findByLabelText('RRF k')
    fireEvent.change(rrfInput, { target: { value: '80' } })
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Save config' })).not.toBeDisabled()
    })
  })

  it('calls POST /api/v1/config on save', async () => {
    renderPanel()
    const rrfInput = await screen.findByLabelText('RRF k')
    fireEvent.change(rrfInput, { target: { value: '80' } })
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Save config' })).not.toBeDisabled()
    })
    fireEvent.click(screen.getByRole('button', { name: 'Save config' }))
    await waitFor(() => {
      expect(mockApiFetch).toHaveBeenCalledWith(
        '/api/v1/config',
        expect.objectContaining({ method: 'POST' }),
      )
    })
  })

  it('renders destructive action buttons', async () => {
    renderPanel()
    expect(await screen.findByText(/Reset workspace/)).toBeTruthy()
    expect(await screen.findByText(/Remove workspace/)).toBeTruthy()
    expect(await screen.findByText(/Reset embeddings/)).toBeTruthy()
  })
})
