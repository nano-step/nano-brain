## Context

nano-brain is a Go-based persistent memory server for AI agents. The dashboard UI is currently embedded in the Go binary via `//go:embed all:dist` (4.2 MB of committed `dist/`), served at `/ui` with `SecurityHeaders()` middleware. The frontend stack is React 18, Vite 5, TanStack Query/Router/Table, zustand, with Mermaid for flow visualization and Sigma/graphology for the knowledge graph.

The UI and API share a release cycle, repository, and build pipeline. This coupling prevents independent frontend iteration and locks the dashboard to the current chart libraries.

**Current state:**
- `internal/server/webui/` — embed.go, handler.go, fallback.go + tests
- `internal/server/routes.go:137` — `webui.RegisterUIRoutes(... SecurityHeaders())`
- `internal/server/middleware/security_headers.go` — CSP, X-Frame-Options, etc.
- `web/` — full React app (vite.config.ts with `outDir: ../internal/server/webui/dist`)
- `scripts/smoke-ui.sh` — harness smoke test
- `Makefile` — web-install/dev/build/check targets
- CI has **no web-build step** (dist is committed)

## Goals / Non-Goals

**Goals:**
- Decouple UI from Go binary: remove embedded `dist`, `web/` directory, and all UI-related code from nano-brain
- Create `nano-brain-dashboard` as a standalone repo with independent CI/CD
- Use local-served proxy model (`npx nano-brain-dashboard`) to avoid CORS/PNA/mixed-content issues
- Port existing panels to new repo with functional parity
- Replace Mermaid/Sigma with a renderer-agnostic `GraphCanvas` component
- Add `/api/version` endpoint for dashboard↔API compatibility checking
- Add `nodes[]/edges[]` to flow API response for direct consumption by graph renderer

**Non-Goals:**
- Hosted HTTPS deployment (Chromium-only, PNA complexity — future phase if pursued)
- CORS/PNA/CSRF middleware changes (not needed for local-served proxy model)
- OpenAPI spec generation (port existing `types.ts` instead)
- Rewrite of existing API endpoints or data models
- Auth changes (token auth already exists; proxy model needs no new auth)

## Decisions

### D1: Local-served proxy model (not hosted HTTPS)

**Decision:** Dashboard runs as `npx nano-brain-dashboard` — a local Node server that serves the SPA and proxies `/api/*` to `http://localhost:3100`.

**Why:** Hosted HTTPS → `http://localhost` fails on Safari/Firefox (no PNA support), relies on in-flux Chromium features, and mixed-content errors are undiagnosable in JS. The proxy model gives same-origin from the browser's view — no CORS, no PNA, no mixed content, SSE works as-is.

**Alternatives considered:**
- Hosted HTTPS with CORS+PNA: Chromium-only, fragile, breaks SSE (`EventSource` can't send `Authorization` header)
- Local TLS cert for API: Painful cert UX on localhost, still needs CORS

### D2: React Flow for chart library (with G6 as swap target)

**Decision:** Use React Flow for graph/flow visualization in the initial build. Hide behind `GraphCanvas` interface so G6 can be swapped later.

**Why:** React Flow is declarative, React-native, well-documented. G6 v5 is imperative/canvas with sparse docs — high risk for the initial build. The `GraphCanvas` abstraction means the swap is mechanical.

**Alternatives considered:**
- G6 from day one: Higher quality output but 3-5x implementation effort, sparse docs
- Mermaid for flow only: No interactivity, no zoom/pan/click

### D3: Port existing `types.ts` (not OpenAPI codegen)

**Decision:** Copy `web/src/api/client.ts` and `web/src/api/types.ts` as the API contract. OpenAPI/codegen is optional later.

**Why:** Existing types are battle-tested. OpenAPI adds build complexity and is not needed when both sides are under our control.

### D4: Rollback anchor before `/ui` removal

**Decision:** Tag `vYYYY.M.D.N-last-ui` before deleting any UI code. This is the rollback point.

**Why:** No overlap window — the new dashboard must be at parity and published before removal. The tag gives a safe rollback.

### D5: One PR per panel in Phase 3

**Decision:** Port panels individually (Settings → Workspaces → Symbols → Harvest → Memory → Dashboard → CodeSummarize), one PR each.

**Why:** Smaller PRs are easier to review, test, and revert if something breaks. Easiest-first order builds confidence.

## Risks / Trade-offs

| Risk | Mitigation |
|------|------------|
| **Two-repo coordination overhead** | `/api/version` endpoint + `SUPPORTED_API_RANGE` constant in dashboard; documented API version matrix |
| **Panel parity gap** | Functional comparison checklist per panel; manual smoke test against old `/ui` |
| **GraphCanvas abstraction leaks** | Unit tests for adapters; integration test rendering sample graphs |
| **`npx` startup latency** | Acceptable for dev tool; could add `--cached` flag later |
| **Breaking existing `/ui` users** | Rollback tag; docs migration guide; deprecation notice in old UI before removal |
| **SSE through proxy** | Works as-is (same origin); only breaks if hosted approach is added later |
