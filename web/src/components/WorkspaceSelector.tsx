import { useState, useEffect, useRef, useCallback, useMemo } from 'react'
import { useWorkspaces } from '../hooks/useWorkspaces'
import { getCurrentWorkspace, setCurrentWorkspace } from '../api/workspace'
import type { Workspace } from '../api/types'

function useCurrentWorkspaceHash(): string | null {
  const [hash, setHash] = useState<string | null>(() => getCurrentWorkspace())

  useEffect(() => {
    const params = new URLSearchParams(window.location.search)
    const fromUrl = params.get('workspace')
    if (fromUrl) {
      setCurrentWorkspace(fromUrl)
      setHash(fromUrl)
    }
  }, [])

  return hash
}

export function WorkspaceSelector() {
  const [open, setOpen] = useState(false)
  const dropdownRef = useRef<HTMLDivElement>(null)
  const { data, isLoading } = useWorkspaces()
  const currentHash = useCurrentWorkspaceHash()

  const workspaces: Workspace[] = useMemo(() => data?.workspaces ?? [], [data])
  const current = workspaces.find((w) => w.hash === currentHash) ?? workspaces[0] ?? null

  useEffect(() => {
    if (workspaces.length > 0 && !currentHash) {
      const first = workspaces[0]
      setCurrentWorkspace(first.hash)
    }
  }, [workspaces, currentHash])

  useEffect(() => {
    if (!open) return
    function onClickOutside(e: MouseEvent) {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    document.addEventListener('mousedown', onClickOutside)
    return () => document.removeEventListener('mousedown', onClickOutside)
  }, [open])

  const select = useCallback((ws: Workspace) => {
    setCurrentWorkspace(ws.hash)
    setOpen(false)
    const params = new URLSearchParams(window.location.search)
    params.set('workspace', ws.hash)
    const newUrl = window.location.pathname + '?' + params.toString()
    window.history.pushState({}, '', newUrl)
    window.dispatchEvent(new PopStateEvent('popstate'))
  }, [])

  if (isLoading) {
    return (
      <div className="ws-picker">
        <div className="ws-picker-label">Workspace</div>
        <div
          className="skel"
          style={{ height: '32px', width: '100%' }}
          aria-label="Loading workspaces"
        />
      </div>
    )
  }

  return (
    <div className="ws-picker" ref={dropdownRef}>
      <div className="ws-picker-label">Workspace</div>
      <button
        className="ws-picker-value"
        onClick={() => setOpen((o) => !o)}
        aria-haspopup="listbox"
        aria-expanded={open}
        aria-label={current ? `Current workspace: ${current.name}` : 'Select workspace'}
      >
        <span className="ws-picker-name">{current?.name ?? 'Select workspace'}</span>
        <span className="ws-picker-hash">{current ? current.hash.slice(0, 6) : '—'}</span>
        <span className="ws-picker-chevron" aria-hidden="true">
          {open ? '▴' : '▾'}
        </span>
      </button>

      {open && workspaces.length > 0 && (
        <div className="ws-dropdown" role="listbox" aria-label="Workspaces">
          {workspaces.map((ws) => (
            <button
              key={ws.hash}
              className="ws-dropdown-item"
              role="option"
              aria-selected={ws.hash === current?.hash}
              data-active={ws.hash === current?.hash ? 'true' : 'false'}
              onClick={() => select(ws)}
            >
              <span className="ws-dropdown-item-name">{ws.name}</span>
              <span className="ws-dropdown-item-meta">
                {ws.hash.slice(0, 8)} · {ws.doc_count.toLocaleString()} docs
              </span>
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
