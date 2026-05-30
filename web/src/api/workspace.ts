const WORKSPACE_KEY = 'nano-brain.workspace'

export function getCurrentWorkspace(): string | null {
  return localStorage.getItem(WORKSPACE_KEY)
}

export function setCurrentWorkspace(hash: string): void {
  localStorage.setItem(WORKSPACE_KEY, hash)
}
