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
import { Placeholder } from '../components/Placeholder'

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
  component: () => <Placeholder story="9.8" />,
})

const routeTree = rootRoute.addChildren([
  indexRoute,
  dashboardRoute,
  memoryRoute,
  graphRoute,
  symbolsRoute,
  harvestRoute,
  settingsRoute,
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
