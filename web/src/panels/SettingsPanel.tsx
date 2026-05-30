import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { z } from 'zod'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { apiFetch, apiGetJSON } from '../api/client'
import { getCurrentWorkspace } from '../api/workspace'
import type { ConfigResponse, DoctorResponse, DoctorCheck } from '../api/types'
import { ConfirmDialog } from '../components/ConfirmDialog'

const settingsSchema = z.object({
  search_rrf_k: z.number().min(1).max(1000),
  search_recency_weight: z.number().min(0).max(1),
  search_recency_half_life_days: z.number().int().min(1).max(3650),
  search_limit: z.number().int().min(1).max(500),
  watcher_debounce_ms: z.number().int().min(100).max(60000),
  watcher_reindex_interval: z.number().int().min(10).max(86400),
  summarization_enabled: z.boolean(),
  summarization_model: z.string().min(1),
  summarization_max_tokens: z.number().int().min(100).max(200000),
  summarization_concurrency: z.number().int().min(1).max(32),
  summarization_requests_per_second: z.number().min(0).max(100),
  logging_level: z.enum(['trace', 'debug', 'info', 'warn', 'error']),
})

type SettingsForm = z.infer<typeof settingsSchema>

function statusIcon(status: string): string {
  if (status === 'ok') return '✅'
  if (status === 'fail') return '❌'
  if (status === 'skip') return '—'
  return '⚠️'
}

function DoctorSection() {
  const { data, isLoading } = useQuery({
    queryKey: ['doctor'],
    queryFn: () => apiGetJSON<DoctorResponse>('/api/v1/doctor'),
    staleTime: 30_000,
  })
  const [expanded, setExpanded] = useState<string | null>(null)

  if (isLoading) return <div className="form-section"><div className="skel" style={{ height: 80 }} /></div>

  const checks: DoctorCheck[] = data?.checks ?? []

  return (
    <div className="form-section">
      <div className="section-title">System checks</div>
      {checks.map((ch) => (
        <div key={ch.name} className="doctor-row">
          <span className={`doctor-badge doctor-badge--${ch.status === 'ok' ? 'ok' : ch.status === 'fail' ? 'err' : 'warn'}`}>
            {statusIcon(ch.status)}
          </span>
          <span className="doctor-name">{ch.name}</span>
          <span className="doctor-detail">{ch.detail}</span>
          {ch.hint && (
            <button
              className="doctor-expand"
              onClick={() => setExpanded(expanded === ch.name ? null : ch.name)}
              aria-expanded={expanded === ch.name}
            >
              {expanded === ch.name ? '▲ hide' : '▼ hint'}
            </button>
          )}
          {expanded === ch.name && ch.hint && (
            <div className="doctor-hint">{ch.hint}</div>
          )}
        </div>
      ))}
    </div>
  )
}

interface DestructiveActionsProps {
  workspace: string
}

function DestructiveActions({ workspace }: DestructiveActionsProps) {
  const [dialog, setDialog] = useState<null | 'reset' | 'remove' | 'embeddings'>(null)
  const [toast, setToast] = useState('')

  function showToast(msg: string) {
    setToast(msg)
    setTimeout(() => setToast(''), 3000)
  }

  async function handleResetWorkspace() {
    setDialog(null)
    try {
      const r = await apiFetch('/api/v1/reset-workspace', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ workspace }),
      })
      showToast(r.ok ? 'Workspace reset successfully' : 'Reset failed')
    } catch {
      showToast('Reset request failed')
    }
  }

  async function handleRemoveWorkspace() {
    setDialog(null)
    try {
      const r = await apiFetch(`/api/v1/workspaces/${workspace}`, { method: 'DELETE' })
      showToast(r.ok ? 'Workspace removed' : 'Remove failed')
    } catch {
      showToast('Remove request failed')
    }
  }

  async function handleResetEmbeddings() {
    setDialog(null)
    try {
      const r = await apiFetch('/api/v1/reset-embeddings', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ workspace }),
      })
      showToast(r.ok ? 'Embeddings reset' : 'Embeddings reset failed')
    } catch {
      showToast('Embeddings reset request failed')
    }
  }

  return (
    <div className="form-section">
      <div className="section-title">Destructive actions</div>
      {toast && <div className="settings-toast settings-toast--warn">{toast}</div>}
      <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
        <button className="btn btn-danger-outline" onClick={() => setDialog('reset')}>
          Reset workspace — delete all documents &amp; embeddings
        </button>
        <button className="btn btn-danger-outline" onClick={() => setDialog('remove')}>
          Remove workspace — unregister and delete all data
        </button>
        <button className="btn btn-danger-outline" onClick={() => setDialog('embeddings')}>
          Reset embeddings — re-embed from scratch
        </button>
      </div>

      <ConfirmDialog
        open={dialog === 'reset'}
        title="Reset workspace"
        description="This will delete ALL documents, chunks, and embeddings for this workspace. The workspace registration is preserved."
        confirmText={workspace}
        confirmLabel="Reset workspace"
        danger
        onConfirm={handleResetWorkspace}
        onCancel={() => setDialog(null)}
      />
      <ConfirmDialog
        open={dialog === 'remove'}
        title="Remove workspace"
        description={`This will permanently delete workspace ${workspace} and ALL its data. This cannot be undone.`}
        confirmText={workspace}
        confirmLabel="Remove workspace"
        danger
        onConfirm={handleRemoveWorkspace}
        onCancel={() => setDialog(null)}
      />
      <ConfirmDialog
        open={dialog === 'embeddings'}
        title="Reset embeddings"
        description="This will delete all embeddings for this workspace. They will be re-generated on next indexing run."
        confirmText={workspace}
        confirmLabel="Reset embeddings"
        danger
        onConfirm={handleResetEmbeddings}
        onCancel={() => setDialog(null)}
      />
    </div>
  )
}

function SettingsForm({ config }: { config: ConfigResponse['config'] }) {
  const qc = useQueryClient()
  const [toast, setToast] = useState('')

  function showToast(msg: string) {
    setToast(msg)
    setTimeout(() => setToast(''), 3000)
  }

  const {
    register,
    handleSubmit,
    formState: { errors, isDirty },
  } = useForm<SettingsForm>({
    resolver: zodResolver(settingsSchema),
    defaultValues: {
      search_rrf_k: config.search.rrf_k,
      search_recency_weight: config.search.recency_weight,
      search_recency_half_life_days: config.search.recency_half_life_days,
      search_limit: config.search.limit,
      watcher_debounce_ms: config.watcher.debounce_ms,
      watcher_reindex_interval: config.watcher.reindex_interval,
      summarization_enabled: config.summarization.enabled,
      summarization_model: config.summarization.model,
      summarization_max_tokens: config.summarization.max_tokens,
      summarization_concurrency: config.summarization.concurrency,
      summarization_requests_per_second: config.summarization.requests_per_second,
      logging_level: (config.logging.level as SettingsForm['logging_level']) ?? 'info',
    },
  })

  async function patchField(path: string, value: unknown) {
    const r = await apiFetch('/api/v1/config', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ path, value }),
    })
    if (!r.ok) throw new Error(`${r.status}`)
  }

  const onSubmit = async (data: SettingsForm) => {
    const patches: Array<{ path: string; value: unknown }> = [
      { path: 'search.rrf_k', value: data.search_rrf_k },
      { path: 'search.recency_weight', value: data.search_recency_weight },
      { path: 'search.recency_half_life_days', value: data.search_recency_half_life_days },
      { path: 'search.limit', value: data.search_limit },
      { path: 'watcher.debounce_ms', value: data.watcher_debounce_ms },
      { path: 'watcher.reindex_interval', value: data.watcher_reindex_interval },
      { path: 'summarization.enabled', value: data.summarization_enabled },
      { path: 'summarization.model', value: data.summarization_model },
      { path: 'summarization.max_tokens', value: data.summarization_max_tokens },
      { path: 'summarization.concurrency', value: data.summarization_concurrency },
      { path: 'summarization.requests_per_second', value: data.summarization_requests_per_second },
      { path: 'logging.level', value: data.logging_level },
    ]

    try {
      await Promise.all(patches.map((p) => patchField(p.path, p.value)))
      showToast('Config saved + reloaded')
      qc.invalidateQueries({ queryKey: ['config'] })
    } catch {
      showToast('Save failed — check server logs')
    }
  }

  return (
    <form onSubmit={handleSubmit(onSubmit)} noValidate>
      {toast && <div className="settings-toast">{toast}</div>}

      <div className="form-section">
        <div className="section-title">Secrets (read-only)</div>
        <div className="form-row">
          <label className="form-label">Database URL</label>
          <input className="form-input" value={'<redacted>'} disabled readOnly />
        </div>
        <div className="form-row">
          <label className="form-label">Embedding API key</label>
          <input className="form-input" value={'<redacted>'} disabled readOnly />
        </div>
        <div className="form-row">
          <label className="form-label">Summarization API key</label>
          <input className="form-input" value={'<redacted>'} disabled readOnly />
        </div>
      </div>

      <div className="form-divider" />

      <div className="form-section">
        <div className="section-title">Search</div>
        <div className="form-row">
          <label className="form-label" htmlFor="s-rrf-k">RRF k</label>
          <input id="s-rrf-k" className="form-input" type="number" step="1" {...register('search_rrf_k', { valueAsNumber: true })} />
          {errors.search_rrf_k && <div className="form-error">{errors.search_rrf_k.message}</div>}
        </div>
        <div className="form-row">
          <label className="form-label" htmlFor="s-rw">Recency weight</label>
          <input id="s-rw" className="form-input" type="number" step="0.01" {...register('search_recency_weight', { valueAsNumber: true })} />
          {errors.search_recency_weight && <div className="form-error">{errors.search_recency_weight.message}</div>}
        </div>
        <div className="form-row">
          <label className="form-label" htmlFor="s-rhl">Recency half-life (days)</label>
          <input id="s-rhl" className="form-input" type="number" step="1" {...register('search_recency_half_life_days', { valueAsNumber: true })} />
          {errors.search_recency_half_life_days && <div className="form-error">{errors.search_recency_half_life_days.message}</div>}
        </div>
        <div className="form-row">
          <label className="form-label" htmlFor="s-lim">Result limit</label>
          <input id="s-lim" className="form-input" type="number" step="1" {...register('search_limit', { valueAsNumber: true })} />
          {errors.search_limit && <div className="form-error">{errors.search_limit.message}</div>}
        </div>
      </div>

      <div className="form-divider" />

      <div className="form-section">
        <div className="section-title">Watcher</div>
        <div className="form-row">
          <label className="form-label" htmlFor="w-db">Debounce (ms)</label>
          <input id="w-db" className="form-input" type="number" step="100" {...register('watcher_debounce_ms', { valueAsNumber: true })} />
          {errors.watcher_debounce_ms && <div className="form-error">{errors.watcher_debounce_ms.message}</div>}
        </div>
        <div className="form-row">
          <label className="form-label" htmlFor="w-ri">Reindex interval (s)</label>
          <input id="w-ri" className="form-input" type="number" step="10" {...register('watcher_reindex_interval', { valueAsNumber: true })} />
          {errors.watcher_reindex_interval && <div className="form-error">{errors.watcher_reindex_interval.message}</div>}
        </div>
      </div>

      <div className="form-divider" />

      <div className="form-section">
        <div className="section-title">Summarization</div>
        <div className="form-row">
          <label className="form-label" htmlFor="sum-en">Enabled</label>
          <input id="sum-en" type="checkbox" {...register('summarization_enabled')} />
        </div>
        <div className="form-row">
          <label className="form-label" htmlFor="sum-model">Model</label>
          <input id="sum-model" className="form-input" type="text" {...register('summarization_model')} />
          {errors.summarization_model && <div className="form-error">{errors.summarization_model.message}</div>}
        </div>
        <div className="form-row">
          <label className="form-label" htmlFor="sum-tok">Max tokens</label>
          <input id="sum-tok" className="form-input" type="number" step="100" {...register('summarization_max_tokens', { valueAsNumber: true })} />
          {errors.summarization_max_tokens && <div className="form-error">{errors.summarization_max_tokens.message}</div>}
        </div>
        <div className="form-row">
          <label className="form-label" htmlFor="sum-conc">Concurrency</label>
          <input id="sum-conc" className="form-input" type="number" step="1" {...register('summarization_concurrency', { valueAsNumber: true })} />
          {errors.summarization_concurrency && <div className="form-error">{errors.summarization_concurrency.message}</div>}
        </div>
        <div className="form-row">
          <label className="form-label" htmlFor="sum-rps">Req/s</label>
          <input id="sum-rps" className="form-input" type="number" step="0.5" {...register('summarization_requests_per_second', { valueAsNumber: true })} />
          {errors.summarization_requests_per_second && <div className="form-error">{errors.summarization_requests_per_second.message}</div>}
        </div>
      </div>

      <div className="form-divider" />

      <div className="form-section">
        <div className="section-title">Logging</div>
        <div className="form-row">
          <label className="form-label" htmlFor="log-lvl">Level</label>
          <select id="log-lvl" className="form-input" {...register('logging_level')}>
            <option value="trace">trace</option>
            <option value="debug">debug</option>
            <option value="info">info</option>
            <option value="warn">warn</option>
            <option value="error">error</option>
          </select>
          {errors.logging_level && <div className="form-error">{errors.logging_level.message}</div>}
        </div>
      </div>

      <div className="form-divider" />

      <div style={{ padding: '16px 0' }}>
        <button type="submit" className="btn btn-primary" disabled={!isDirty}>
          Save config
        </button>
      </div>
    </form>
  )
}

export function SettingsPanel() {
  const workspace = getCurrentWorkspace()
  const { data, isLoading, error } = useQuery({
    queryKey: ['config'],
    queryFn: () => apiGetJSON<ConfigResponse>('/api/v1/config'),
    staleTime: 60_000,
  })

  return (
    <div className="panel">
      <div className="topbar">
        <span className="topbar-title">Settings</span>
      </div>
      <div style={{ maxWidth: 720 }}>
        {isLoading && <div className="skel" style={{ height: 200, margin: 24 }} />}
        {error && (
          <div style={{ padding: 24, color: 'var(--err)' }}>
            Failed to load config: {error instanceof Error ? error.message : 'unknown error'}
          </div>
        )}
        {data && <SettingsForm config={data.config} />}
        <div className="form-divider" />
        <DoctorSection />
        {workspace && (
          <>
            <div className="form-divider" />
            <DestructiveActions workspace={workspace} />
          </>
        )}
      </div>
    </div>
  )
}

