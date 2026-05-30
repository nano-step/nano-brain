import { getCurrentWorkspace } from './workspace'

export async function apiFetch(path: string, init?: RequestInit): Promise<Response> {
  const ws = getCurrentWorkspace()
  const headers = new Headers(init?.headers)
  if (ws) headers.set('X-Workspace-Hash', ws)
  headers.set('X-Requested-With', 'nano-brain-ui')
  return fetch(path, { ...init, headers })
}

export async function apiGetJSON<T>(path: string): Promise<T> {
  const r = await apiFetch(path)
  if (!r.ok) throw new Error(`${r.status} ${r.statusText}`)
  return r.json() as Promise<T>
}
