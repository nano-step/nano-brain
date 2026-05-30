import { useEffect, useRef } from 'react'

const FOCUSABLE = [
  'a[href]',
  'button:not([disabled])',
  'input:not([disabled])',
  'select:not([disabled])',
  'textarea:not([disabled])',
  '[tabindex]:not([tabindex="-1"])',
].join(',')

export function useFocusTrap(active: boolean) {
  const ref = useRef<HTMLElement | null>(null)

  useEffect(() => {
    if (!active || !ref.current) return
    const el = ref.current

    const prev = document.activeElement as HTMLElement | null

    const focusable = (): HTMLElement[] => Array.from(el.querySelectorAll<HTMLElement>(FOCUSABLE))

    const first = () => focusable()[0]
    const last = () => focusable().at(-1)

    first()?.focus()

    function onKey(e: KeyboardEvent) {
      if (e.key !== 'Tab') return
      const items = focusable()
      if (items.length === 0) { e.preventDefault(); return }
      if (e.shiftKey) {
        if (document.activeElement === items[0]) { e.preventDefault(); items.at(-1)?.focus() }
      } else {
        if (document.activeElement === last()) { e.preventDefault(); items[0]?.focus() }
      }
    }

    el.addEventListener('keydown', onKey)
    return () => {
      el.removeEventListener('keydown', onKey)
      prev?.focus()
    }
  }, [active])

  return ref
}
