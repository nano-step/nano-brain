## 1. Phase 0 — De-risk (gate)

- [ ] 1.1 Run nano-brain locally: `docker compose up -d postgres`, build + run API, verify `/api/status` returns `pg_status: healthy`
- [ ] 1.2 Create throwaway Vite spike to prove proxy model: fetch `/api/status` through `server.proxy['/api']='http://localhost:3100'`, confirm no CORS error, delete spike
- [ ] 1.3 SSE proxy spike: verify `EventSource` reconnects after proxy restart, confirm no buffering (60s+ long-lived connection)
- [ ] 1.4 CSRF proxy spike: send POST through proxy with `X-Requested-With` header, verify 200 (not 403)

## 2. Phase 1 — Backend API additions (nano-brain repo)

- [ ] 2.1 Add `nodes[]`/`edges[]` to flow response in `internal/server/handlers/flow.go`, populate from `Flow.Nodes/Edges` (role, kind, conditional, line), keep `mermaid`/`chain`/`externals`
- [ ] 2.2 Mirror `nodes[]`/`edges[]` in MCP `memory_flow` tool (`internal/mcp/tools.go`)
- [ ] 2.3 Add tests asserting new `nodes`/`edges` fields in flow response
- [ ] 2.4 Add `/api/version` endpoint returning `{version, migration_version, api_min, api_max}`
- [ ] 2.5 Add CSRF rule for loopback-any-port (allow any loopback port, not exact match)
- [ ] 2.6 Verify: `go build ./... && go test -race -short ./internal/...` green

## 3. Phase 2 — New repo scaffold

- [ ] 3.1 Create `nano-step/nano-brain-dashboard` repo, initialize Vite React-TS project
- [ ] 3.2 Install dependencies: `@tanstack/react-query`, `@tanstack/react-router`, `@tanstack/react-table`, `zustand`, `react-hook-form`, `zod`, `@hookform/resolvers`, `react-markdown`, `remark-gfm`, `rehype-raw`, `rehype-sanitize`, `lucide-react`, `cmdk`, `fuse.js`, `date-fns`
- [ ] 3.3 Install dev dependencies: `vitest`, `@testing-library/react`, `@testing-library/jest-dom`, `jsdom`, `eslint`, `prettier`
- [ ] 3.4 Configure Vite proxy: `server.proxy['/api'] = env.VITE_API_BASE || 'http://localhost:3100'`
- [ ] 3.5 Create `server.js`: Express static server + `http-proxy-middleware` proxying `/api/*`, `/sse`, `/mcp`; accept `--api-base`/`--port`/`--api-token` flags; set security headers (nosniff, X-Frame-Options, Referrer-Policy); port conflict fallback (4321-4326); backend health check on startup
- [ ] 3.6 Add `bin` field to `package.json` for `npx` entry point
- [ ] 3.7 Port API client: copy `web/src/api/client.ts` and `web/src/api/types.ts`, change base to relative `/api`, remove `X-Requested-With` reliance
- [ ] 3.8 Set up TanStack QueryClient + Zustand connection store (API base, status, version)
- [ ] 3.9 Port app shell: `src/app/router.tsx` (change `basepath` from `/ui` to `/`), `layout.tsx`, `theme.ts`, `styles/tokens.css`
- [x] 3.10 Change `vite.config.ts` `base` from `'/ui/'` to `'/'`
- [x] 3.11 Add CSP `<meta>` tag to `index.html`
- [x] 3.12 Add connection status + version-compat banner: call `/api/version`, compare `SUPPORTED_API_RANGE`, show warning on mismatch
- [x] 3.13 Handle localStorage workspace loss: on first load, if no workspace selected, prompt user with workspace picker (`GET /api/v1/workspaces`)
- [x] 3.14 Verify: `npm run dev` serves app, `/api/status` works through proxy, app boots and shows nav

## 4. Phase 3 — Panel parity (batch into 2-3 PRs)

**PR 1 — Simple panels (no graph deps, no SSE):**
- [ ] 4.1 Port Settings panel: copy from `web/src/panels/Settings.tsx`, adapt imports, port test
- [ ] 4.2 Port Workspaces panel: copy from `web/src/panels/Workspaces.tsx`, adapt imports, port test
- [ ] 4.3 Port Symbols panel: copy from `web/src/panels/Symbols.tsx`, adapt imports, port test

**PR 2 — Data panels (search, stats, SSE):**
- [ ] 4.4 Port Memory panel: copy from `web/src/panels/Memory.tsx`, adapt search/results/DocDrawer, port test
- [ ] 4.5 Port Dashboard panel: copy from `web/src/panels/Dashboard.tsx`, keep sparklines, port test
- [ ] 4.6 Port Harvest panel (uses SSE): copy from `web/src/panels/Harvest.tsx`, verify `useEvents` works through proxy, port test
- [ ] 4.7 Port CodeSummarize panel: copy from `web/src/panels/CodeSummarize.tsx`, adapt imports, port test

**PR 3 — Graph + Flow (deferred to separate proposal):**
- [ ] 4.8 Port Graph panel with existing Sigma.js (keep as-is, no React Flow yet)
- [ ] 4.9 Port Flow panel with existing Mermaid (keep as-is, no GraphCanvas yet)

- [ ] 4.10 Verify each panel: functional parity with old `/ui` per spec scenarios, `npm run typecheck && npm run lint && npm run test`

## 5. Phase 4 — Package + release

- [ ] 5.1 Finalize `npx` packaging: flags/help/URL print, smoke test on clean machine
- [ ] 5.2 Create dashboard CI: typecheck/lint/test/build workflow
- [ ] 5.3 Publish to npm as `@nano-step/nano-brain-dashboard`
- [ ] 5.4 Write install + connect documentation, document supported API version range
- [ ] 5.5 Document fallback paths: Docker alternative, build from source, known limitations (Node.js requirement)
- [ ] 5.6 Verify: `npx nano-brain-dashboard` on clean machine serves app and connects to running nano-brain

## 6. Phase 5 — Remove /ui from nano-brain (🔒 senior review, 2-3 days)

- [ ] 6.1 Create rollback anchor: tag `vYYYY.M.D.N-last-ui`, push tag
- [ ] 6.2 Delete `internal/server/webui/` directory (embed.go, handler.go, fallback.go, tests)
- [ ] 6.3 Remove `webui` import + `RegisterUIRoutes(... SecurityHeaders())` from `internal/server/routes.go`
- [ ] 6.4 Add `/ui` deprecation redirect handler (serve 200 with link to new dashboard, NOT 301)
- [ ] 6.5 Delete or repurpose `internal/server/middleware/security_headers.go` + its test
- [ ] 6.6 `git rm -r internal/server/webui/dist/` (committed assets)
- [ ] 6.7 `git rm -r web/` (entire React app directory)
- [ ] 6.8 Remove `web-*` targets + `.PHONY` entries from Makefile
- [ ] 6.9 Delete `scripts/smoke-ui.sh`, remove `smoke:ui` from harness validation ladder
- [ ] 6.10 Search and fix stale references: `grep -rn "webui\|/ui\|smoke-ui\|web-build"` → no live refs
- [ ] 6.11 Run `go mod tidy` to remove orphaned dependencies
- [ ] 6.12 Verify: `go build ./...` succeeds, `go test -race -short ./...` green, binary smaller, CHANGELOG updated
