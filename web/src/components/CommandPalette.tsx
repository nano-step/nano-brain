import React, { useCallback, useEffect, useRef, useState } from 'react'
import { Command } from 'cmdk'
import { createPortal } from 'react-dom'
import { useRouter } from '@tanstack/react-router'
import Fuse from 'fuse.js'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { apiFetch, apiGetJSON } from '../api/client'
import { getCurrentWorkspace } from '../api/workspace'
import type { WorkspacesResponse } from '../api/types'
import { useFocusTrap } from '../hooks/useFocusTrap'

const RECENT_KEY = 'nano-brain.recent-searches'
const MAX_RECENT = 20

interface NavItem {
  type: 'nav'
  id: string
  label: string
  path: string
}

interface ActionItem {
  type: 'action'
  id: string
  label: string
  endpoint: string
  method: 'POST'
}

interface WorkspaceItem {
  type: 'workspace'
  id: string
  label: string
  hash: string
}

interface RecentItem {
  type: 'recent'
  id: string
  label: string
}

interface SymbolItem {
  type: 'symbol'
  id: string
  label: string
  symbolId: string
  collection: string
}

type PaletteItem = NavItem | ActionItem | WorkspaceItem | RecentItem | SymbolItem

const NAV_ITEMS: NavItem[] = [
  { type: 'nav', id: 'nav-dashboard', label: 'Dashboard', path: '/dashboard' },
  { type: 'nav', id: 'nav-memory', label: 'Memory', path: '/memory' },
  { type: 'nav', id: 'nav-graph', label: 'Graph', path: '/graph' },
  { type: 'nav', id: 'nav-symbols', label: 'Symbols', path: '/symbols' },
  { type: 'nav', id: 'nav-harvest', label: 'Harvest', path: '/harvest' },
  { type: 'nav', id: 'nav-settings', label: 'Settings', path: '/settings' },
]

const ACTION_ITEMS: ActionItem[] = [
  { type: 'action', id: 'action-reindex', label: 'Trigger reindex', endpoint: '/api/v1/reindex', method: 'POST' },
  { type: 'action', id: 'action-harvest', label: 'Trigger harvest', endpoint: '/api/harvest', method: 'POST' },
  { type: 'action', id: 'action-reload', label: 'Reload config', endpoint: '/api/reload-config', method: 'POST' },
]

function getRecent(): string[] {
  try {
    return JSON.parse(localStorage.getItem(RECENT_KEY) ?? '[]') as string[]
  } catch {
    return []
  }
}

function addRecent(query: string) {
  const items = getRecent().filter((x) => x !== query)
  items.unshift(query)
  localStorage.setItem(RECENT_KEY, JSON.stringify(items.slice(0, MAX_RECENT)))
}

function useSymbolSearch(query: string, enabled: boolean) {
  const workspace = getCurrentWorkspace()
  return useQuery({
    queryKey: ['palette-symbols', query, workspace],
    queryFn: async () => {
      if (!query || !workspace) return []
      const r = await apiFetch(`/api/v1/symbols?q=${encodeURIComponent(query)}&workspace=${workspace}&limit=8`)
      if (!r.ok) return []
      const json = (await r.json()) as { symbols?: Array<{ id: string; name: string; collection?: string }> }
      return (json.symbols ?? []).map<SymbolItem>((s) => ({
        type: 'symbol',
        id: 'sym-' + s.id,
        label: s.name,
        symbolId: s.id,
        collection: s.collection ?? '',
      }))
    },
    enabled: enabled && !!query,
    staleTime: 5_000,
  })
}

export function CommandPalette() {
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState('')
  const [debouncedQuery, setDebouncedQuery] = useState('')
  const prevFocusRef = useRef<Element | null>(null)
  const trapRef = useFocusTrap(open)
  const router = useRouter()
  const qc = useQueryClient()

  const { data: workspacesData } = useQuery({
    queryKey: ['workspaces'],
    queryFn: () => apiGetJSON<WorkspacesResponse>('/api/v1/workspaces'),
    staleTime: 60_000,
    enabled: open,
  })

  useEffect(() => {
    const id = setTimeout(() => setDebouncedQuery(query), 150)
    return () => clearTimeout(id)
  }, [query])

  const { data: symbolResults } = useSymbolSearch(debouncedQuery, open)

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault()
        prevFocusRef.current = document.activeElement
        setOpen((v) => !v)
      }
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [])

  const close = useCallback(() => {
    setOpen(false)
    setQuery('')
    setTimeout(() => (prevFocusRef.current as HTMLElement | null)?.focus(), 10)
  }, [])

  useEffect(() => {
    if (!open) return
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') {
        e.preventDefault()
        close()
      }
    }
    document.addEventListener('keydown', onKey)
    return () => document.removeEventListener('keydown', onKey)
  }, [open, close])

  const workspaceItems: WorkspaceItem[] = (workspacesData?.workspaces ?? []).map((w) => ({
    type: 'workspace',
    id: 'ws-' + w.hash,
    label: w.name || w.root_path,
    hash: w.hash,
  }))

  const recentItems: RecentItem[] = getRecent().map((r, i) => ({
    type: 'recent',
    id: 'recent-' + i,
    label: r,
  }))

  const fuse = new Fuse<PaletteItem>(
    [...NAV_ITEMS, ...ACTION_ITEMS, ...workspaceItems] as PaletteItem[],
    { keys: ['label'], threshold: 0.4 },
  )

  const fuzzyResults: PaletteItem[] = query
    ? fuse.search(query).map((r) => r.item)
    : []

  async function handleSelect(item: PaletteItem) {
    close()
    if (item.type === 'nav') {
      void router.navigate({ to: item.path })
    } else if (item.type === 'action') {
      try {
        const r = await apiFetch(item.endpoint, { method: item.method })
        if (r.ok) {
          console.log(`[cmdk] ${item.label}: accepted`)
        }
      } catch {
        console.error('[cmdk] action failed', item.endpoint)
      }
    } else if (item.type === 'workspace') {
      localStorage.setItem('nano-brain.workspace', item.hash)
      window.location.reload()
    } else if (item.type === 'recent') {
      addRecent(item.label)
      void router.navigate({ to: '/memory', search: { q: item.label } as Record<string, string> })
    } else if (item.type === 'symbol') {
      void router.navigate({ to: '/graph', search: { focus: item.symbolId, mode: 'symbol' } as Record<string, string> })
    }
    qc.invalidateQueries({ queryKey: ['workspaces'] })
  }

  const grouped = query
    ? {
        nav: fuzzyResults.filter((i) => i.type === 'nav') as NavItem[],
        action: fuzzyResults.filter((i) => i.type === 'action') as ActionItem[],
        workspace: fuzzyResults.filter((i) => i.type === 'workspace') as WorkspaceItem[],
        symbol: symbolResults ?? [],
        recent: [] as RecentItem[],
      }
    : {
        nav: NAV_ITEMS,
        action: ACTION_ITEMS,
        workspace: workspaceItems,
        symbol: [],
        recent: recentItems,
      }

  if (!open) return null

  return createPortal(
    <div className="cmdk-overlay" onClick={close}>
      <div
        className="cmdk-modal"
        onClick={(e) => e.stopPropagation()}
        ref={trapRef as React.RefObject<HTMLDivElement>}
      >
        <Command aria-label="Command palette">
          <Command.Input
            className="cmdk-input"
            placeholder="Type a command, route, or symbol…"
            value={query}
            onValueChange={setQuery}
            autoFocus
          />
          <Command.List>
            <Command.Empty className="cmdk-empty">No results found.</Command.Empty>

            {grouped.nav.length > 0 && (
              <Command.Group heading="Navigation" className="cmdk-group">
                {grouped.nav.map((item) => (
                  <Command.Item
                    key={item.id}
                    className="cmdk-item"
                    onSelect={() => handleSelect(item)}
                  >
                    {item.label}
                  </Command.Item>
                ))}
              </Command.Group>
            )}

            {grouped.action.length > 0 && (
              <Command.Group heading="Actions" className="cmdk-group">
                {grouped.action.map((item) => (
                  <Command.Item
                    key={item.id}
                    className="cmdk-item"
                    onSelect={() => handleSelect(item)}
                  >
                    {item.label}
                  </Command.Item>
                ))}
              </Command.Group>
            )}

            {grouped.workspace.length > 0 && (
              <Command.Group heading="Workspaces" className="cmdk-group">
                {grouped.workspace.map((item) => (
                  <Command.Item
                    key={item.id}
                    className="cmdk-item"
                    onSelect={() => handleSelect(item)}
                  >
                    {item.label}
                  </Command.Item>
                ))}
              </Command.Group>
            )}

            {grouped.symbol.length > 0 && (
              <Command.Group heading="Symbols" className="cmdk-group">
                {grouped.symbol.map((item) => (
                  <Command.Item
                    key={item.id}
                    className="cmdk-item"
                    onSelect={() => handleSelect(item)}
                  >
                    {item.label}
                    {item.collection && (
                      <span className="cmdk-item-meta">{item.collection}</span>
                    )}
                  </Command.Item>
                ))}
              </Command.Group>
            )}

            {grouped.recent.length > 0 && (
              <Command.Group heading="Recent searches" className="cmdk-group">
                {grouped.recent.map((item) => (
                  <Command.Item
                    key={item.id}
                    className="cmdk-item"
                    onSelect={() => handleSelect(item)}
                  >
                    {item.label}
                  </Command.Item>
                ))}
              </Command.Group>
            )}
          </Command.List>
        </Command>
      </div>
    </div>,
    document.body,
  )
}
