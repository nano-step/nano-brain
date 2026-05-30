import { useState, useEffect, useCallback } from 'react'

type Theme = 'dark' | 'light'
const THEME_KEY = 'nano-brain.theme'

function readTheme(): Theme {
  const stored = localStorage.getItem(THEME_KEY)
  return stored === 'light' ? 'light' : 'dark'
}

function applyTheme(theme: Theme): void {
  document.documentElement.setAttribute('data-theme', theme)
}

export function useTheme(): { theme: Theme; toggleTheme: () => void } {
  const [theme, setTheme] = useState<Theme>(() => {
    const t = readTheme()
    applyTheme(t)
    return t
  })

  useEffect(() => {
    applyTheme(theme)
    localStorage.setItem(THEME_KEY, theme)
  }, [theme])

  const toggleTheme = useCallback(() => {
    setTheme((t) => (t === 'dark' ? 'light' : 'dark'))
  }, [])

  return { theme, toggleTheme }
}
