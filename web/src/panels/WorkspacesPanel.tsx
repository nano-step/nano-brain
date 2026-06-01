import { useState } from 'react'
import { ConfirmDialog } from '../components/ConfirmDialog'
import { useWorkspaces } from '../hooks/useWorkspaces'
import { useRemoveWorkspace } from './workspaces/useRemoveWorkspace'
import { clearCurrentWorkspace, getCurrentWorkspace, setCurrentWorkspace } from '../api/workspace'
import type { Workspace } from '../api/types'

function fmtNum(n: number | undefined | null): string {
  return n?.toLocaleString() ?? '0'
}

function truncHash(hash: string | undefined | null): string {
  if (!hash) return ''
  return hash.length > 16 ? hash.slice(0, 16) + '…' : hash
}

export function WorkspacesPanel() {
  return <WorkspacesPanelBody />
}

function WorkspacesPanelBody() {
  const { data, isLoading, isError, error, refetch } = useWorkspaces()
  const removeMut = useRemoveWorkspace()
  const [pending, setPending] = useState<Workspace | null>(null)
  const [errorMsg, setErrorMsg] = useState<string | null>(null)
  const currentHash = getCurrentWorkspace()

  const workspaces: Workspace[] = data?.workspaces ?? []

  function onSwitch(w: Workspace) {
    setCurrentWorkspace(w.hash)
    window.location.reload()
  }

  function onAskRemove(w: Workspace) {
    setErrorMsg(null)
    setPending(w)
  }

  function onCancelRemove() {
    setPending(null)
    setErrorMsg(null)
  }

  function onConfirmRemove() {
    if (!pending) return
    const target = pending
    removeMut.mutate(target.hash, {
      onSuccess: () => {
        setPending(null)
        setErrorMsg(null)
        if (target.hash === currentHash) {
          const remaining = workspaces.filter((w) => w.hash !== target.hash)
          if (remaining.length > 0) {
            setCurrentWorkspace(remaining[0].hash)
          } else {
            clearCurrentWorkspace()
          }
          window.location.reload()
        }
      },
      onError: (err) => {
        setErrorMsg(err instanceof Error ? err.message : 'Failed to remove workspace')
      },
    })
  }

  return (
    <div className="panel">
      <div className="surface">
        <div className="surface-head">
          <span>Workspaces</span>
          <span style={{ color: 'var(--text-3)', fontSize: 11 }}>
            {workspaces.length > 0 ? `${workspaces.length} registered` : ''}
          </span>
        </div>
        <div className="surface-pad">
          {isLoading && <div className="skel" style={{ height: 120 }} />}

          {isError && !isLoading && (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 8, alignItems: 'flex-start' }}>
              <div style={{ color: 'var(--err)', fontSize: 12 }}>
                Failed to load workspaces: {error instanceof Error ? error.message : 'unknown error'}
              </div>
              <button className="btn" onClick={() => void refetch()}>
                Retry
              </button>
            </div>
          )}

          {!isLoading && !isError && workspaces.length === 0 && (
            <div style={{ color: 'var(--text-3)', fontSize: 13, lineHeight: 1.6 }}>
              <p>No workspaces registered.</p>
              <p>
                Register one via CLI:{' '}
                <code className="mono">nano-brain init --root=&lt;path&gt;</code>
              </p>
              <p>
                Or POST to <code className="mono">/api/v1/init</code> with{' '}
                <code className="mono">{'{"root_path":"<path>","name":"<name>"}'}</code>.
              </p>
            </div>
          )}

          {!isLoading && !isError && workspaces.length > 0 && (
            <table className="table" style={{ fontSize: 13 }}>
              <thead>
                <tr>
                  <th style={{ textAlign: 'left' }}>Name</th>
                  <th style={{ textAlign: 'left' }}>Hash</th>
                  <th style={{ textAlign: 'right' }}>Docs</th>
                  <th style={{ textAlign: 'right' }}>Chunks</th>
                  <th style={{ textAlign: 'right' }}>Actions</th>
                </tr>
              </thead>
              <tbody>
                {workspaces.map((w) => {
                  const isCurrent = w.hash === currentHash
                  return (
                    <tr key={w.hash} data-current={isCurrent ? 'true' : 'false'}>
                      <td>
                        <strong>{w.name}</strong>
                        {isCurrent && (
                          <span
                            className="pill pill-accent"
                            style={{ marginLeft: 8, fontSize: 10 }}
                          >
                            current
                          </span>
                        )}
                      </td>
                      <td className="mono" title={w.hash} style={{ fontSize: 11 }}>
                        {truncHash(w.hash)}
                      </td>
                      <td className="num">{fmtNum(w.doc_count)}</td>
                      <td className="num">{fmtNum(w.chunk_count)}</td>
                      <td style={{ textAlign: 'right' }}>
                        <button
                          className="btn"
                          onClick={() => onSwitch(w)}
                          disabled={isCurrent}
                          style={{ marginRight: 8 }}
                        >
                          {isCurrent ? 'Current' : 'Switch'}
                        </button>
                        <button className="btn btn-danger" onClick={() => onAskRemove(w)}>
                          Remove
                        </button>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          )}
        </div>
      </div>

      {pending && (
        <ConfirmDialog
          open={true}
          danger
          title="Remove workspace"
          description={`This will permanently delete workspace "${pending.name}" (${fmtNum(pending.doc_count)} docs, ${fmtNum(pending.chunk_count)} chunks) and ALL its data. This cannot be undone.${errorMsg ? `\n\nError: ${errorMsg}` : ''}`}
          confirmText={pending.name}
          confirmLabel={removeMut.isPending ? 'Removing…' : 'Remove workspace'}
          onConfirm={onConfirmRemove}
          onCancel={onCancelRemove}
        />
      )}
    </div>
  )
}
