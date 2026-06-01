import React from 'react'
import { Link, useRouterState } from '@tanstack/react-router'
import { useTheme } from './theme'
import { WorkspaceSelector } from '../components/WorkspaceSelector'
import { CommandPalette } from '../components/CommandPalette'
import { NonLoopbackBindBanner } from '../components/NonLoopbackBindBanner'
import { useMnemonicNav } from '../hooks/useMnemonicNav'
import '../styles/tokens.css'
import '../styles/layout.css'
import '../styles/workspace.css'

const NAV_ITEMS = [
  { path: '/dashboard', label: 'Dashboard', key: 'g d' },
  { path: '/memory', label: 'Memory', key: 'g m' },
  { path: '/graph', label: 'Graph', key: 'g g' },
  { path: '/symbols', label: 'Symbols', key: 'g s' },
  { path: '/harvest', label: 'Harvest', key: 'g h' },
  { path: '/workspaces', label: 'Workspaces', key: 'g w' },
  { path: '/settings', label: 'Settings', key: 'g ,' },
] as const

function NavItem({ path, label, navKey }: { path: string; label: string; navKey: string }) {
  const routerState = useRouterState()
  const isActive = routerState.location.pathname === `/ui${path}`

  return (
    <Link to={path} className="nav-item" data-active={isActive ? 'true' : 'false'}>
      <span className="nav-item-icon">
        <NavIcon name={label.toLowerCase()} />
      </span>
      {label}
      <span className="nav-item-key">{navKey}</span>
    </Link>
  )
}

function NavIcon({ name }: { name: string }) {
  const icons: Record<string, string> = {
    dashboard: 'M3 3h7v7H3V3zm0 11h7v7H3v-7zm11-11h7v7h-7V3zm0 11h7v7h-7v-7z',
    memory: 'M21 3H3v6h18V3zm0 8H3v10h18V11zM7 15h6v2H7v-2z',
    graph: 'M5 9a3 3 0 116 0 3 3 0 01-6 0zm8 6a3 3 0 116 0 3 3 0 01-6 0zM9 9l4 6',
    symbols: 'M4 6h16M4 12h10M4 18h16',
    harvest: 'M4 4l8 16 4-8 4-4-4-4z',
    settings: 'M12 8a4 4 0 100 8 4 4 0 000-8zM3 12l2 .8M19 12l2 .8M12 3v2M12 19v2',
    workspaces: 'M3 7h7v7H3V7zm11 0h7v7h-7V7zM3 16h18v5H3v-5z',
  }
  return (
    <svg
      width="14"
      height="14"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.6"
      strokeLinecap="square"
      strokeLinejoin="miter"
      aria-hidden="true"
    >
      <path d={icons[name] ?? ''} />
    </svg>
  )
}

export function Layout({ children }: { children: React.ReactNode }) {
  const { theme, toggleTheme } = useTheme()
  useMnemonicNav()

  return (
    <div style={{ display: 'flex', flexDirection: 'column', minHeight: '100vh' }}>
      <NonLoopbackBindBanner />
      <CommandPalette />
      <div className="app" style={{ flex: 1 }}>
      <aside className="sidebar" role="navigation" aria-label="Main navigation">
        <div className="brand">
          <div className="brand-name">nano-brain</div>
          <div className="brand-sub">memory · code · harvest</div>
        </div>

        <WorkspaceSelector />

        <nav className="nav">
          {NAV_ITEMS.map((item) => (
            <NavItem key={item.path} path={item.path} label={item.label} navKey={item.key} />
          ))}
        </nav>

        <div className="sidebar-footer">
          <button
            className="theme-toggle"
            onClick={toggleTheme}
            aria-label={`Switch to ${theme === 'dark' ? 'light' : 'dark'} theme`}
          >
            {theme === 'dark' ? '☀ light' : '☾ dark'}
          </button>
          <div className="cmdk-hint">
            <span className="kbd">⌘K</span>
          </div>
        </div>
      </aside>

      <main className="main" role="main">
        {children}
      </main>
      </div>
    </div>
  )
}
