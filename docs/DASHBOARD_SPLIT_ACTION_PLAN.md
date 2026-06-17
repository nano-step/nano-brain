# Action plan — dashboard split (executable backlog)

Companion to `DASHBOARD_SPLIT_PLAN.md` (why) and `DASHBOARD_SPLIT_JUNIOR_GUIDE.md` (how).
This file is the **work breakdown**: tickets in dependency order. Each becomes one
GitHub issue + one PR. Est = junior-days. 🔒 = senior review required.

Legend: `[ ]` todo · `dep:` blocking ticket(s) · "Done when" = acceptance.

---

## Milestone M0 — De-risk (gate) · ~2d
Ship nothing until M0 passes; it proves the architecture.

### DASH-001 · Run nano-brain locally · est 0.5 · dep: none
- [ ] `docker compose up -d postgres`
- [ ] build + run API; `curl localhost:3100/api/status`
- [ ] write `nanobrain_test` setup note for later test isolation
- Done when: `/api/status` returns `pg_status: healthy`.

### DASH-002 · Prove the proxy model · est 0.5 · dep: 001
- [ ] throwaway Vite app with `server.proxy['/api']='http://localhost:3100'`
- [ ] fetch `/api/status` from the page, render it, confirm **no CORS error**
- Done when: status JSON shows via proxy; spike deleted; 1-line result in issue.

### DASH-003 · Chart lib spike (React Flow vs G6) · est 1 · dep: 001
- [ ] render `/api/v1/graph/overview` once in React Flow, once in G6 v5
- [ ] compare ease/layout/bundle; write decision note
- Done when: chosen lib recorded in the issue. (Default rec: React Flow.)

**M0 exit:** proxy model proven (002) + chart lib chosen (003).

---

## Milestone M1 — Backend API is dashboard-ready (nano-brain repo) · ~2–3d
Additive, non-breaking. Each ticket = issue + OpenSpec + PR.

### DASH-010 · Add `nodes[]`/`edges[]` to flow API + MCP · est 1.5 · dep: 001
- [ ] extend `flowResponse` in `internal/server/handlers/flow.go`
- [ ] populate from `Flow.Nodes/Edges` (role, kind, conditional, line)
- [ ] mirror in MCP `memory_flow` (`internal/mcp/tools.go`)
- [ ] keep `mermaid`/`chain`/`externals`
- [ ] test asserting new fields
- Done when: `POST /api/v1/graph/flow` returns `nodes`+`edges`; `go test -race -short ./internal/...` green (paste output).

### DASH-011 · `/api/version` endpoint · est 0.5 · dep: none
- [ ] handler → `{version, migration_version, api_min, api_max}`
- [ ] register route; test
- Done when: `curl /api/version` returns JSON.

### DASH-012 🔒 · (HOLD) cross-origin security set · est 0 (deferred) · dep: none
- Not needed for the local-served/proxy model. Only open if a hosted build is later approved: CORS allowlist + PNA preflight + CSRF rewrite (`middleware/csrf.go`) + startup gate (CORS-without-auth refuses boot). Keep as a parked issue.

**M1 exit:** 010 + 011 merged; 012 parked.

---

## Milestone M2 — New repo skeleton · ~2–3d

### DASH-020 · Create `nano-brain-dashboard` repo + scaffold · est 0.5 · dep: 003
- [ ] `gh repo create`, Vite React-TS, install deps (see guide §2.1), add chosen chart lib
- Done when: `npm run dev` serves the default app.

### DASH-021 · Local server + proxy (`npx` entry) · est 1 · dep: 020
- [ ] `vite.config.ts` proxy `/api` → `VITE_API_BASE||localhost:3100`
- [ ] `bin/` Node server (sirv + http-proxy) serving `dist/` + proxying `/api`, flags `--api-base`/`--port`
- [ ] `package.json` `bin` field
- Done when: `npm run dev` and the built `bin` both reach `/api/status`.

### DASH-022 · Port API client + types · est 1 · dep: 020
- [ ] copy `web/src/api/{client.ts,types.ts}`; base → relative `/api`
- [ ] remove `X-Requested-With` reliance
- [ ] TanStack QueryClient + Zustand connection store
- Done when: a hook renders live data; typecheck green.

### DASH-023 · App shell (router/layout/theme) + connection/version banner · est 1 · dep: 022, 011
- [ ] port `app/router.tsx`, `layout.tsx`, `theme.ts`, tokens
- [ ] banner calls `/api/version`, compares `SUPPORTED_API_RANGE`, warns on mismatch
- Done when: app boots, nav renders, shows "connected to API vX" or clear error.

**M2 exit:** app boots, connects, navigable.

---

## Milestone M3 — Panel parity (one PR per panel) · ~4–6d
dep (all): 023. Order easiest→hardest. Each: copy panel+hooks from `web/src/panels/`, adapt imports, restyle, port test.

- [ ] DASH-030 · Settings · est 0.5
- [ ] DASH-031 · Workspaces · est 0.5
- [ ] DASH-032 · Symbols · est 0.5
- [ ] DASH-033 · Harvest (uses events) · est 0.75
- [ ] DASH-034 · Memory (search/results/DocDrawer) · est 1
- [ ] DASH-035 · Dashboard (stats; keep its sparkline) · est 0.75
- [ ] DASH-036 · CodeSummarize · est 0.75
- [ ] DASH-037 · Events/SSE check — confirm `useEvents` works through proxy · est 0.25
- Done when (each): functional parity with old `/ui`, panel test green, lint+typecheck pass.

**M3 exit:** every non-graph panel at parity.

---

## Milestone M4 — Graph + Flow charts · ~4–8d
dep: 023 + chart lib (003).

### DASH-040 · `GraphModel` + `GraphCanvas` interface · est 1 · dep: 003
- [ ] `model.ts` (GNode/GEdge/GraphModel)
- [ ] `GraphCanvas.tsx` props `{model, layout, onNodeClick}`, chart lib hidden inside
- Done when: renders a hardcoded sample model.

### DASH-041 · Adapters (pure, unit-tested) · est 1 · dep: 040
- [ ] `flowToModel.ts`, `apiGraphToModel.ts`
- [ ] unit tests: empty, duplicate edges, all roles/kinds
- Done when: adapter tests green.

### DASH-042 · GraphPanel on GraphCanvas · est 2 · dep: 041
- [ ] replace Sigma; port legend, colors, neighborhood-expand, position cache
- Done when: overview+neighborhood render; node click → DocDrawer; expand works.

### DASH-043 · FlowPanel on GraphCanvas · est 2 · dep: 041, 010
- [ ] consume `nodes/edges` (not mermaid string); directed layout
- [ ] "Copy Mermaid" button (uses retained `mermaid` field)
- Done when: sample endpoints render interactively; no dup arrows; no parse fallback.

**M4 exit:** both graph views on the new renderer; no mermaid/sigma in repo.

---

## Milestone M5 — Package + release · ~1–2d

### DASH-050 · Finalize `npx` packaging · est 0.5 · dep: 021, M3, M4
- [ ] flags/help/URL print; smoke on clean machine
- Done when: `npx nano-brain-dashboard` serves+connects on a fresh machine.

### DASH-051 · npm publish + CI · est 1 · dep: 050
- [ ] dashboard repo CI (typecheck/lint/test/build); publish `@nano-step/nano-brain-dashboard`
- [ ] docs: install/connect + supported API range
- Done when: published; install docs verified.

**M5 exit:** dashboard installable + documented.

---

## Milestone M6 — Remove `/ui` from nano-brain · ~1d · 🔒
dep: M5 parity confirmed.

### DASH-060 · Rollback anchor · est 0.1 · dep: M5
- [ ] tag `vYYYY.M.D.N-last-ui`, push tag
- Done when: tag exists on a release that still has `/ui`.

### DASH-061 🔒 · Delete `/ui` surface · est 0.75 · dep: 060
- [ ] rm `internal/server/webui/` (embed.go, handler.go, fallback.go, *_test.go)
- [ ] routes.go: drop `webui` import + `RegisterUIRoutes(... SecurityHeaders())`
- [ ] decide `middleware/security_headers.go` (delete or repurpose) + its test
- [ ] `git rm -r internal/server/webui/dist/` (committed) and `web/`
- [ ] Makefile: remove `web-*` targets + `.PHONY` entries
- [ ] `scripts/smoke-ui.sh` delete + remove `smoke:ui` from harness ladder/docs
- [ ] `grep -rn "webui\|/ui\|smoke-ui\|web-build"` → no live refs (ignore `.opencode/`)
- Done when: `go build ./...` + `go test -race -tags=integration ./...` green; binary smaller; CHANGELOG updated. (CI has **no** web step to touch.)

**M6 exit:** nano-brain is API-only; docs point to the new dashboard.

---

## Execution order (critical path)
001→002→003 · 010,011 (parallel) · 020→021→022→023 · M3 panels (parallel after 023) · 040→041→{042,043} · 050→051 · 060→061

## Totals (junior)
M0 ~2d · M1 ~2–3d · M2 ~2–3d · M3 ~4–6d · M4 ~4–8d · M5 ~1–2d · M6 ~1d → **~16–25 working days**, longer with review latency. 🔒 tickets (012 parked, 061) need senior review.

## GitHub issues to create now (M0–M1)
| Issue | Title | Labels |
|---|---|---|
| DASH-001 | Run nano-brain locally (dev setup) | chore |
| DASH-002 | Spike: prove Vite proxy → API (no CORS) | spike |
| DASH-003 | Spike: React Flow vs G6 decision | spike |
| DASH-010 | Flow API+MCP return nodes/edges | feat, api |
| DASH-011 | Add /api/version endpoint | feat, api |
| DASH-012 | (parked) cross-origin CORS/PNA/auth | security, blocked |
