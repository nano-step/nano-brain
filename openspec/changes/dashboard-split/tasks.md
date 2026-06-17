## 1. Phase 0 — De-risk (gate)

- [x] 1.1 Run nano-brain locally: `docker compose up -d postgres`, build + run API, verify `/api/status` returns `pg_status: healthy`
- [x] 1.2 Create throwaway Vite spike to prove proxy model: fetch `/api/status` through `server.proxy['/api']='http://localhost:3100'`, confirm no CORS error, delete spike
- [x] 1.3 SSE proxy spike: verify `EventSource` reconnects after proxy restart, confirm no buffering (60s+ long-lived connection)
- [x] 1.4 CSRF proxy spike: send POST through proxy with `X-Requested-With` header, verify 200 (not 403)

## 2. Phase 1 — Backend API additions (nano-brain repo)

- [x] 2.1 Add `nodes[]`/`edges[]` to flow response in `internal/server/handlers/flow.go`, populate from `Flow.Nodes/Edges` (role, kind, conditional, line), keep `mermaid`/`chain`/`externals`
- [x] 2.2 Mirror `nodes[]`/`edges[]` in MCP `memory_flow` tool (`internal/mcp/tools.go`)
- [x] 2.3 Add tests asserting new `nodes`/`edges` fields in flow response
- [x] 2.4 Add `/api/version` endpoint returning `{version, migration_version, api_min, api_max}`
- [x] 2.5 Add CSRF rule for loopback-any-port (allow any loopback port, not exact match)
- [x] 2.6 Verify: `go build ./... && go test -race -short ./internal/...` green

## 3. Phase 2 — New repo scaffold

- [x] 3.1 Create `nano-step/nano-brain-dashboard` repo, initialize Vite React-TS project
- [x] 3.2 Install dependencies: `@tanstack/react-query`, `@tanstack/react-router`, `@tanstack/react-table`, `zustand`, `react-hook-form`, `zod`, `@hookform/resolvers`, `react-markdown`, `remark-gfm`, `rehype-raw`, `rehype-sanitize`, `lucide-react`, `cmdk`, `fuse.js`, `date-fns`
- [x] 3.3 Install dev dependencies: `vitest`, `@testing-library/react`, `@testing-library/jest-dom`, `jsdom`, `eslint`, `prettier`
- [x] 3.4 Configure Vite proxy: `server.proxy['/api'] = env.VITE_API_BASE || 'http://localhost:3100'`
- [x] 3.5 Create `server.js`: Express static server + `http-proxy-middleware` proxying `/api/*`, `/sse`, `/mcp`; accept `--api-base`/`--port`/`--api-token` flags; set security headers (nosniff, X-Frame-Options, Referrer-Policy); port conflict fallback (4321-4326); backend health check on startup
- [x] 3.6 Add `bin` field to `package.json` for `npx` entry point
- [x] 3.7 Port API client: copy `web/src/api/client.ts` and `web/src/api/types.ts`, change base to relative `/api`, remove `X-Requested-With` reliance
- [x] 3.8 Set up TanStack QueryClient + Zustand connection store (API base, status, version)
- [x] 3.9 Port app shell: `src/app/router.tsx` (change `basepath` from `/ui` to `/`), `layout.tsx`, `theme.ts`, `styles/tokens.css`
- [x] 3.10 Change `vite.config.ts` `base` from `'/ui/'` to `'/'`
- [x] 3.11 Add CSP `<meta>` tag to `index.html`
- [x] 3.12 Add connection status + version-compat banner: call `/api/version`, compare `SUPPORTED_API_RANGE`, show warning on mismatch
- [x] 3.13 Handle localStorage workspace loss: on first load, if no workspace selected, prompt user with workspace picker (`GET /api/v1/workspaces`)
- [x] 3.14 Verify: `npm run dev` serves app, `/api/status` works through proxy, app boots and shows nav

## 4. Phase 3 — Panel parity (batch into 2-3 PRs)

**PR 1 — Simple panels (no graph deps, no SSE):**
- [x] 4.1 Port Settings panel: copy from `web/src/panels/Settings.tsx`, adapt imports, port test
- [x] 4.2 Port Workspaces panel: copy from `web/src/panels/Workspaces.tsx`, adapt imports, port test
- [x] 4.3 Port Symbols panel: copy from `web/src/panels/Symbols.tsx`, adapt imports, port test

**PR 2 — Data panels (search, stats, SSE):**
- [x] 4.4 Port Memory panel: copy from `web/src/panels/Memory.tsx`, adapt search/results/DocDrawer, port test
- [x] 4.5 Port Dashboard panel: copy from `web/src/panels/Dashboard.tsx`, keep sparklines, port test
- [x] 4.6 Port Harvest panel (uses SSE): copy from `web/src/panels/Harvest.tsx`, verify `useEvents` works through proxy, port test
- [x] 4.7 Port CodeSummarize panel: copy from `web/src/panels/CodeSummarize.tsx`, adapt imports, port test

**PR 3 — Graph + Flow (lift-and-shift, G6 deferred):**
- [x] 4.8 Port Graph panel with existing Sigma.js (lift-and-shift, G6 GraphCanvas deferred to DASH-040→043)
- [x] 4.9 Port Flow panel with existing Mermaid (lift-and-shift, G6 GraphCanvas deferred to DASH-040→043)

- [x] 4.10 Verify each panel: functional parity with old `/ui` per spec scenarios, `npm run typecheck && npm run lint && npm run test`

## 5. Phase 4 — Package + release

- [x] 5.1 Finalize `npx` packaging: flags/help/URL print, smoke test on clean machine
- [x] 5.2 Create dashboard CI: typecheck/lint/test/build workflow
- [ ] 5.3 Publish to npm as `@nano-step/nano-brain-dashboard`
- [x] 5.4 Write install + connect documentation, document supported API version range
- [ ] 5.5 Document fallback paths: Docker alternative, build from source, known limitations (Node.js requirement)
- [ ] 5.6 Verify: `npx nano-brain-dashboard` on clean machine serves app and connects to running nano-brain

## 6. Phase 5 — Remove /ui from nano-brain

- [ ] 6.1 Create rollback anchor: tag `vYYYY.M.D.N-last-ui`, push tag
- [x] 6.2 Delete `internal/server/webui/` directory (embed.go, handler.go, fallback.go, tests)
- [x] 6.3 Remove `webui` import + `RegisterUIRoutes(... SecurityHeaders())` from `internal/server/routes.go`
- [ ] 6.4 Add `/ui` deprecation redirect handler (serve 200 with link to new dashboard, NOT 301)
- [ ] 6.5 Delete or repurpose `internal/server/middleware/security_headers.go` + its test
- [x] 6.6 `git rm -r internal/server/webui/dist/` (committed assets)
- [x] 6.7 `git rm -r web/` (entire React app directory)
- [x] 6.8 Remove `web-*` targets + `.PHONY` entries from Makefile
- [x] 6.9 Delete `scripts/smoke-ui.sh`, remove `smoke:ui` from harness validation ladder
- [ ] 6.10 Search and fix stale references: `grep -rn "webui\|/ui\|smoke-ui\|web-build"` → no live refs
- [ ] 6.11 Run `go mod tidy` to remove orphaned dependencies
- [x] 6.12 Verify: `go build ./...` succeeds, `go test -race -short ./...` green, binary smaller, CHANGELOG updated

## 7. Deferred — G6 GraphCanvas migration (DASH-040→043)

**Status**: Deferred. Current dashboard uses Sigma.js + Mermaid (lift-and-shift from legacy). G6 migration will replace these with a unified `GraphCanvas` abstraction.

- [ ] 7.1 Install `@antv/g6` in dashboard repo
- [ ] 7.2 Create `src/components/GraphCanvas.tsx` abstraction
- [ ] 7.3 Migrate GraphPanel from Sigma.js to GraphCanvas
- [ ] 7.4 Migrate FlowPanel from Mermaid to GraphCanvas
- [ ] 7.5 Remove sigma, graphology, graphology-layout-forceatlas2, mermaid dependencies
- [ ] 7.6 Verify graph rendering parity
