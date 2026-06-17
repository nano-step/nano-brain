## Why

The dashboard UI is embedded in the Go binary via `//go:embed all:dist` (4.2 MB), coupling UI releases to the Go build/release cycle. This prevents independent frontend iteration, adds complexity to the Go build, and locks the dashboard to the existing Mermaid/Sigma chart stack. Splitting into a separate repo enables independent release cadence, a modern chart stack (G6 or React Flow), and a cleaner API boundary.

## What Changes

- **Remove `/ui` from nano-brain**: Delete embedded `dist/`, `web/` directory, `webui` package, related middleware, Makefile targets, and smoke tests. The Go binary becomes API-only (~4 MB smaller).
- **Create `nano-brain-dashboard` repo**: New React SPA with Vite, TanStack, zustand. Uses a local-served proxy model (`npx nano-brain-dashboard`) to avoid CORS/PNA/mixed-content issues.
- **Backend API additions**: Add `nodes[]/edges[]` to flow response, add `/api/version` endpoint for compatibility checking.
- **No CORS/PNA work in this phase**: The local-served proxy model (same-origin from browser's view) eliminates cross-origin security concerns. CORS/PNA/CSRF changes are deferred to a future hosted-HTTPS phase if pursued.

## Capabilities

### New Capabilities

- `dashboard-repo`: New `nano-brain-dashboard` repository with Vite+React+TanStack stack, `npx` entry point with local server + API proxy, connection/auth UX, and panel parity with current `/ui`.
- `dashboard-graph-renderer`: Renderer-agnostic `GraphCanvas` component with adapters for flow and knowledge graph views, replacing Mermaid and Sigma/graphology.
- `dashboard-npx-distribution`: npm package `@nano-step/nano-brain-dashboard` that serves the SPA locally and proxies `/api/*` to the nano-brain backend.

### Modified Capabilities

- `web-ui-server`: **REMOVED** — the embedded web UI server (`internal/server/webui/`) is deleted. The `/ui` route registration and `SecurityHeaders()` middleware are removed. The API server continues without UI serving.

## Impact

- **Go binary**: ~4 MB smaller (embedded `dist/` removed). Build simplifies (no web toolchain dependency).
- **Two-repo coordination**: Dashboard and API must coordinate on API version compatibility. Mitigated by `/api/version` endpoint and `SUPPORTED_API_RANGE` constant in dashboard.
- **Existing `/ui` users**: Must migrate to `npx nano-brain-dashboard`. A rollback anchor tag (`-last-ui`) is created before removal.
- **CI/CD**: Dashboard gets its own CI pipeline (typecheck/lint/test/build/deploy). Nano-brain CI has no web-build step to remove.
- **Dependencies removed from nano-brain**: `mermaid`, `sigma`, `graphology`, `graphology-layout-forceatlas2` (never in Go deps, but in committed `web/`).
- **Dependencies added to dashboard**: `reactflow` or `@antv/g6`, `@tanstack/react-query`, `@tanstack/react-router`, `zustand`, `lucide-react`, `cmdk`, `fuse.js`.
