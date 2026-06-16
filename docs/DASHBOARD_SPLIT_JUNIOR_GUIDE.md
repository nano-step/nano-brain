# Step-by-step guide: split the dashboard out of nano-brain (for a junior dev)

This is the build runbook. Read `docs/DASHBOARD_SPLIT_PLAN.md` §13 first for *why*
the approach differs from the original idea. Follow phases in order. **Do not skip
the acceptance check at the end of each step.** Tasks marked 🔒 must be reviewed by
a senior (security-sensitive).

## What you're building (mental model)

- **nano-brain** = a headless Go API + MCP server on `http://localhost:3100`. It will
  stop shipping a web UI.
- **nano-brain-dashboard** = a new React SPA in its own repo. A user runs
  `npx nano-brain-dashboard`, which starts a tiny local server that (a) serves the
  built SPA and (b) **proxies `/api/*` to `http://localhost:3100`**. Because the
  browser only ever talks to the local proxy origin, there is **no CORS, no
  mixed-content, no Private Network Access** problem. This is the whole reason we
  chose local-served over a hosted site.

```
browser ── http://localhost:4321 ──► npx dashboard server ──► proxy /api/* ──► http://localhost:3100 (nano-brain)
            (SPA + same-origin API)
```

## Skills & tools you need

- Go 1.23 (`go`, `gofmt`), Node 20+, npm, Docker (for Postgres), `git`, `gh`.
- Comfortable with React + TypeScript + Vite. You'll learn TanStack Query/Router and one graph library.
- Read `AGENTS.md` / `docs/HARNESS.md` — this repo has a **strict process** (below).

## Non-negotiable ground rules (from the harness)

1. **Create a GitHub issue before any work** (`gh issue create --repo nano-step/nano-brain`).
2. **Never commit to `master`.** Branch as `feat/NNN-...`, open a PR.
3. **Multi-file changes go through OpenSpec** (`/opsx-propose`) — this whole project does.
4. **Tests must pass and you must paste output.** `go build ./... && go test -race -short ./...`.
5. **Never use the dev database for tests.** Use `nanobrain_test` / port `3199` (see `AGENTS.md`).
6. **Don't `pkill -f` broadly** — capture exact PIDs.
7. 🔒 **Security tasks need a senior reviewer.** You are not the sole owner of auth/CORS code.

---

# Phase 0 — Setup + prove the local-served model (1–2 days)

**Goal:** get the repo building/running locally and prove the proxy model end-to-end with a throwaway page. This de-risks the whole architecture before you write real code.

### Step 0.1 — Run nano-brain locally
```bash
# Postgres (Docker)
docker compose up -d postgres      # or: docker compose -f docker/docker-compose.yml up -d
# build + run the API
CGO_ENABLED=0 go build -o ./bin/nano-brain ./cmd/nano-brain
DATABASE_URL="postgres://nanobrain:nanobrain@localhost:5432/nanobrain_dev" ./bin/nano-brain
curl -s localhost:3100/api/status          # expect JSON
```
✅ **Accept:** `/api/status` returns JSON with `pg_status: healthy`.

### Step 0.2 — Prove the proxy idea with Vite
Scratch project, throwaway:
```bash
npm create vite@latest proxy-spike -- --template react-ts && cd proxy-spike && npm i
```
Add to `vite.config.ts`:
```ts
server: { proxy: { '/api': 'http://localhost:3100' } }
```
In `App.tsx`, `fetch('/api/status').then(r=>r.json())` and render it. `npm run dev`, open the page.
✅ **Accept:** the page shows the API status JSON, fetched through the Vite proxy (same-origin). No CORS error in the console. **This proves the model.** Delete the spike.

### Step 0.3 — Pick the chart library (timeboxed, ½ day)
Load the largest workspace's `/api/v1/graph/overview` JSON. Render it once in **React Flow** and once in **G6 v5**. Note: ease of use, layout quality, bundle size.
- Recommendation for a junior: **React Flow** (declarative, React nodes, gentle). G6 is the stated target but is imperative/canvas with sparse docs.
- Whatever you pick, you'll hide it behind a `GraphCanvas` interface (Phase 4) so it can be swapped.
✅ **Accept:** a one-paragraph decision note in the PR/issue with the chosen lib.

---

# Phase 1 — Backend: make nano-brain a clean API (2–3 days)

These are small, additive, non-breaking changes in the **nano-brain** repo.

### Step 1.1 — Add `nodes[]`/`edges[]` to the flow response
`internal/server/handlers/flow.go` — the `Flow` struct already has `Nodes` and `Edges`
(`internal/flow/builder.go`) with `Role`, `Kind`, `Conditional`, `Line`. Add two fields
to `flowResponse` and populate them (keep `mermaid`, `chain`, `externals` for back-compat).
Do the same in the MCP `memory_flow` result (`internal/mcp/tools.go`).
✅ **Accept:** `POST /api/v1/graph/flow {entry,format:"mermaid"}` returns `nodes` + `edges`; `go test -race -short ./internal/flow/... ./internal/server/handlers/...` green. Add a test asserting the new fields.

### Step 1.2 — Add a `/api/version` JSON endpoint
A tiny handler returning `{version, migration_version, api_min, api_max}` so the dashboard can check compatibility before authed calls. (`X-Nano-Brain-Version` header already exists in `internal/server/middleware.go` but a header alone isn't enough.)
✅ **Accept:** `curl localhost:3100/api/version` returns the JSON.

### Step 1.3 — (Optional, only if you'll ever expose beyond the proxy) token auth on 🔒
Token auth already exists: `middleware/auth.go` (Bearer), `nano-brain auth token` generates one, `server.auth.enabled` toggles it. For the **local-served proxy model you do NOT need CORS or new auth** — skip the CORS/PNA/CSRF rewrite entirely. Only if a senior later decides to support a hosted origin do you add: CORS allowlist middleware, PNA preflight (`Access-Control-Allow-Private-Network`), a CSRF rewrite (remove the `X-Requested-With: nano-brain-ui` bypass in `middleware/csrf.go`), and a **startup gate that refuses to boot if CORS origins are set but auth is off**. All 🔒.
✅ **Accept (default path):** nothing to do; document that the proxy model needs no CORS.

---

# Phase 2 — New repo scaffold (2–3 days)

### Step 2.1 — Create the repo
```bash
gh repo create nano-step/nano-brain-dashboard --private --clone
cd nano-brain-dashboard
npm create vite@latest . -- --template react-ts
npm i @tanstack/react-query @tanstack/react-router @tanstack/react-table zustand \
      react-hook-form zod @hookform/resolvers react-markdown remark-gfm rehype-raw \
      rehype-sanitize lucide-react cmdk fuse.js date-fns
npm i -D vitest @testing-library/react @testing-library/jest-dom jsdom \
      eslint prettier typescript @types/react @types/react-dom
# chart lib from Phase 0.3:
npm i reactflow      # OR: npm i @antv/g6
```

### Step 2.2 — Wire the proxy + dev server
`vite.config.ts`: `server.proxy['/api'] = env.VITE_API_BASE || 'http://localhost:3100'`.
Add a `bin/` entry (a ~30-line Node server using `sirv` + `http-proxy`) so
`npx nano-brain-dashboard` serves `dist/` and proxies `/api/*` in production too.
Add `bin` to `package.json`.
✅ **Accept:** `npm run dev` serves the app and `/api/status` works through the proxy.

### Step 2.3 — Port the API client + types (do NOT rewrite from scratch)
Copy `web/src/api/client.ts` and `web/src/api/types.ts` from nano-brain into
`src/api/`. Change the client base to relative `/api` (proxy handles the rest).
Remove the `X-Requested-With` header dependence if present. Set up a TanStack Query
`QueryClient` and a Zustand store for connection state (API base, status, version).
✅ **Accept:** typecheck passes; a `useStats`-style hook returns live data on screen.

### Step 2.4 — App shell: router, layout, theme, connection/health banner
Port `src/app/router.tsx`, `layout.tsx`, `theme.ts`, `styles/tokens.css`. Add a
top-level **connection status + version-compat banner** (calls `/api/version`,
compares to a `SUPPORTED_API_RANGE` constant, warns on mismatch).
✅ **Accept:** app boots, shows nav, shows "connected to API vX" or a clear error.

---

# Phase 3 — Port the non-graph panels for parity (4–6 days)

Port these one at a time; each is a PR. Reuse the existing panel logic from
`web/src/panels/` — the hooks and API calls are already correct; you're rebuilding
the presentation, not the data layer.

Order (easiest → hardest): **Settings → Workspaces → Symbols → Harvest → Memory →
Dashboard → CodeSummarize.**

Per panel:
1. Copy the matching `web/src/panels/<Name>.tsx` + its hook(s) under `src/`.
2. Adapt imports to the new client/types.
3. Re-style with the new design tokens.
4. Port (or rewrite) its test from `web/src/__tests__/<Name>.test.tsx`.

⚠️ **Live events (SSE):** `web/src/hooks/useEvents.ts` uses native `EventSource`.
Through the proxy this works as-is (same origin). Keep it. (Only if you ever go
cross-origin would you need to switch to fetch-streaming — see plan §13.)
✅ **Accept (per panel):** functional parity with the old `/ui`, panel test green, `npm run lint && npm run typecheck && npm run test`.

---

# Phase 4 — Graph + Flow charts (4–8 days; the hard part)

### Step 4.1 — Define a renderer-agnostic model + `GraphCanvas`
Create `src/components/graph/model.ts`:
```ts
export type GNode = { id: string; label: string; role: string; meta?: Record<string, unknown> }
export type GEdge = { id: string; source: string; target: string; kind: string; dashed?: boolean }
export type GraphModel = { nodes: GNode[]; edges: GEdge[] }
```
Create `src/components/graph/GraphCanvas.tsx` with props `{ model, layout, onNodeClick }`.
Implement it with your Phase-0 lib **behind this interface** so the panels never import the lib directly.

### Step 4.2 — Adapters (pure functions — unit-test these heavily)
- `adapters/flowToModel.ts`: `flowResponse.nodes/edges` → `GraphModel` (color by `role`, dash conditional/middleware edges).
- `adapters/apiGraphToModel.ts`: `GraphNode[]/GraphEdge[]` from `/graph/overview` + `/graph/neighborhood` → `GraphModel`.
✅ **Accept:** adapter unit tests cover empty graph, duplicate edges, all roles/kinds.

### Step 4.3 — GraphPanel on `GraphCanvas`
Replace the Sigma/graphology renderer. Port the legend, node coloring, neighborhood
expand, and position cache (`usePositionCache`). Use a force/`force`-style layout.
✅ **Accept:** overview + neighborhood render; click opens the DocDrawer; expand works.

### Step 4.4 — FlowPanel on `GraphCanvas`
Use the new `nodes/edges` from Step 1.1 (not the Mermaid string). Directed layout
(dagre-style). Keep a "Copy Mermaid" button that copies the still-present `mermaid`
field for pasting into docs.
✅ **Accept:** `GET /balance` and a few endpoints render interactively, no parse-error fallback, no duplicate arrows.

### Step 4.5 — Remove old chart deps
`npm rm mermaid sigma graphology graphology-layout-forceatlas2` (in the new repo these were never added — just confirm they're absent).

---

# Phase 5 — Package, host, document (1–2 days)

- Finish the `npx nano-brain-dashboard` local server (Step 2.2): serve `dist/`, proxy `/api`, accept `--api-base` and `--port` flags, print the URL.
- Publish to npm as `@nano-step/nano-brain-dashboard` (mirror the existing release flow).
- (Optional, Chromium-only) a hosted static build — **only** with the senior-owned CORS/PNA/auth work from Step 1.3. Document its limitations.
- Docs: install + connect instructions; supported API version range.
✅ **Accept:** `npx nano-brain-dashboard` on a clean machine serves the app and connects to a running nano-brain.

---

# Phase 6 — Remove `/ui` from nano-brain (1 day) 🔒 review

**First, create a rollback anchor:** tag a nano-brain release that still has `/ui`
(`git tag vYYYY.M.D.N-last-ui && git push --tags`) before deleting anything.

Then delete/adjust **all** of these (verified against the tree):

1. `internal/server/webui/` — the whole package: `embed.go`, `handler.go`,
   `fallback.go`, and tests `embed_debug_test.go`, `handler_test.go`.
2. `internal/server/routes.go` — remove the `webui` import (line ~12) and the
   `webui.RegisterUIRoutes(s.echo, webui.EmbedFS, middleware.SecurityHeaders())`
   call (line ~137).
3. `internal/server/middleware/security_headers.go` (+ its test) — now **orphaned**
   (it was "/ui only"). Delete it, or repurpose for API responses (decide with senior).
4. `internal/server/webui/dist/` — **committed to git**; `git rm -r` it.
5. `web/` — the entire React app directory (`git rm -r web`).
6. `web/vite.config.ts` had `outDir` → `internal/server/webui/dist` — gone with `web/`.
7. `Makefile` — remove `web-install/web-dev/web-build/web-check` targets (lines ~23–33)
   and drop them from the `.PHONY` line (line 3).
8. `scripts/smoke-ui.sh` — delete it, and remove the `smoke:ui` step from the harness
   validation ladder / `docs/HARNESS*.md`.
9. Search and fix stale references: `grep -rn "webui\|/ui\|smoke-ui\|web-build" --include='*.go' --include='*.md' --include='Makefile' .` (ignore `.opencode/`).

**Note:** CI (`.github/workflows/ci.yml`, `release.yml`) has **no web-build step** —
nothing to remove there. The binary embeds the committed `dist/`, which is why
removing the embed shrinks the binary (~4 MB) and simplifies the build.

✅ **Accept:** `CGO_ENABLED=0 go build ./...` succeeds with no `web/` and no `webui`
package; `go test -race -short ./...` green; `grep` finds no live `/ui`/`webui`
references; binary size dropped. Update `CHANGELOG.md` and point docs at the new dashboard.

---

## Per-phase verification ladder (map to the harness)

| Phase | Run |
|---|---|
| every backend change | `go build ./... && go test -race -short ./...` (paste output) |
| every frontend change | `npm run typecheck && npm run lint && npm run test` |
| cutover phases (3,4,6) | integration tests + manual smoke (load each panel, compare to old `/ui`) |
| Phase 6 | full `go test -race -tags=integration ./...` + `./nano-brain status` |

## When you're stuck (common traps)

- **CORS errors in dev** → you broke the proxy; the client must call relative `/api`, not an absolute URL.
- **SSE not updating** → confirm `/api/v1/events` goes through the proxy (same origin).
- **G6 won't render / blank canvas** → this is why we hid it behind `GraphCanvas`; if it's eating days, switch the `GraphCanvas` impl to React Flow. Don't fight it.
- **A panel needs data the API doesn't expose** → check `web/src/api/types.ts`; the endpoint likely exists. Don't add backend endpoints without an issue + OpenSpec.
- **Security task (CORS/auth/CSRF)** → stop and pair with a senior. Do not ship it solo.

## Definition of done

Old `/ui` is deleted from nano-brain; `npx nano-brain-dashboard` gives full parity
plus interactive Graph/Flow charts; all tests green; docs updated; a `-last-ui`
release tag exists as rollback.
