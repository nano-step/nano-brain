import { useEffect, useRef } from 'react'
import { useRouter } from '@tanstack/react-router'

const TIMEOUT_MS = 800

const SEQUENCES: Record<string, string> = {
  'd': '/dashboard',
  'm': '/memory',
  'g': '/graph',
  's': '/symbols',
  'h': '/harvest',
  ',': '/settings',
}

function isEditable(el: Element | null): boolean {
  if (!el) return false
  const tag = (el as HTMLElement).tagName.toLowerCase()
  if (tag === 'input' || tag === 'textarea' || tag === 'select') return true
  if ((el as HTMLElement).isContentEditable) return true
  return false
}

export function useMnemonicNav() {
  const router = useRouter()
  const pendingRef = useRef<string | null>(null)
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      if (e.metaKey || e.ctrlKey || e.altKey) return
      if (isEditable(document.activeElement)) return

      const key = e.key

      if (pendingRef.current === 'g') {
        if (timerRef.current) clearTimeout(timerRef.current)
        pendingRef.current = null
        const path = SEQUENCES[key]
        if (path) {
          e.preventDefault()
          void router.navigate({ to: path })
        }
        return
      }

      if (key === 'g') {
        pendingRef.current = 'g'
        timerRef.current = setTimeout(() => {
          pendingRef.current = null
        }, TIMEOUT_MS)
        return
      }
    }

    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('keydown', onKey)
      if (timerRef.current) clearTimeout(timerRef.current)
    }
  }, [router])
}
