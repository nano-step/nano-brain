import {
  createRouter,
  createRootRoute,
  createRoute,
  Outlet,
  Navigate,
} from '@tanstack/react-router'
import { Layout } from './layout'
import { DashboardPanel } from '../panels/DashboardPanel'
import { GraphPanel } from '../panels/GraphPanel'
import { MemoryPanel } from '../panels/MemoryPanel'
import { SymbolsPanel } from '../panels/SymbolsPanel'
import { HarvestPanel } from '../panels/HarvestPanel'
import { SettingsPanel } from '../panels/SettingsPanel'
import { WorkspacesPanel } from '../panels/WorkspacesPanel'
import { CodeSummarizePanel } from '../panels/CodeSummarizePanel'
import { FlowsPanel } from '../panels/FlowsPanel'

const rootRoute = createRootRoute({
  component: () => (
    <Layout>
      <Outlet />
    </Layout>
  ),
})

const indexRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/',
  component: () => <Navigate to="/dashboard" />,
})

const dashboardRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/dashboard',
  component: DashboardPanel,
})

const memoryRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/memory',
  validateSearch: (search: Record<string, unknown>) => ({
    tags: typeof search.tags === 'string' ? search.tags : undefined,
    doc: typeof search.doc === 'string' ? search.doc : undefined,
  }),
  component: MemoryPanel,
})

const graphRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/graph',
  component: GraphPanel,
})

const symbolsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/symbols',
  component: SymbolsPanel,
})

const harvestRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/harvest',
  component: HarvestPanel,
})

const settingsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/settings',
  component: SettingsPanel,
})

const workspacesRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/workspaces',
  component: WorkspacesPanel,
})

const codeSummarizeRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/code-summarize',
  component: CodeSummarizePanel,
})

const flowsRoute = createRoute({
  getParentRoute: () => rootRoute,
  path: '/flows',
  component: FlowsPanel,
})

const routeTree = rootRoute.addChildren([
  indexRoute,
  dashboardRoute,
  memoryRoute,
  graphRoute,
  flowsRoute,
  symbolsRoute,
  harvestRoute,
  settingsRoute,
  workspacesRoute,
  codeSummarizeRoute,
])

export const router = createRouter({
  routeTree,
  basepath: '/ui',
})

declare module '@tanstack/react-router' {
  interface Register {
    router: typeof router
  }
}
