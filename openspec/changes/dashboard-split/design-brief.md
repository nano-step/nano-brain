## Design Brief: Dashboard Split

### Settled Decisions (HIGH confidence — both agents agree)

- **Local-served proxy model** — eliminates CORS/PNA entirely, same-origin from browser's view | Basis: Both agents confirmed, cross-critiques didn't challenge
- **React Flow / GraphCanvas deferred** — keep existing Sigma.js in the split; React Flow is a separate proposal | Basis: Both agents agreed, both cross-critiques confirmed
- **Port types.ts as-is** — 299 lines, stable, no OpenAPI codegen needed | Basis: Both agents agreed
- **Single server.js proxy** — Express + http-proxy-middleware, not a separate TS project | Basis: Oracle recommended, Metis didn't dispute
- **Two repos** — `nano-brain-dashboard` separate from `nano-brain` | Basis: Both agents agreed
- **No auth UI in dashboard** — auth stays server-side, proxy forwards headers transparently | Basis: Both agents agreed
- **One PR per panel** — incremental porting, not all-at-once | Basis: Both agents agreed on phasing

### Architecture Approach

The dashboard runs as `npx nano-brain-dashboard` — a local Node server that:
1. Serves the built SPA from `dist/`
2. Proxies `/api/*`, `/sse`, `/mcp` to `http://localhost:3100`
3. Sets security headers (CSP, nosniff, X-Frame-Options, Referrer-Policy)

The Go binary becomes API-only: remove `internal/server/webui/`, `web/`, Makefile web targets. Add a `/ui` deprecation page pointing users to `npx nano-brain-dashboard`.

**Key insight from Oracle cross-critique**: The SPA's `apiFetch()` already uses relative paths (`fetch('/api/v1/...')`). The only SPA config change needed is `vite.config.ts` `base` from `'/ui/'` to `'/'` and `router.tsx` `basepath` from `'/ui'` to `'/'`. Everything else — routes, panels, hooks, components — ships unchanged.

### Implementation Phases

**Phase 0 — Proxy Spike (gate, 1-2 days)**
Build a throwaway proxy server. Verify: SSE streaming through proxy, CSRF POST/PUT/DELETE with `X-Requested-With` header, security headers on responses, "nano-brain not running" error page. If any of these fail, the approach needs rework before any panel porting.

**Phase 1 — New Repo + 2-3 Panels (3-4 days)**
Create `nano-brain-dashboard` repo. Write `server.js` (~30 lines). Port Settings, Workspaces, Memory panels (simplest, no graph deps, no SSE). Change `vite.config.ts` `base` to `'/'`. Publish as `@nano-step/nano-brain-dashboard@0.1.0`.

**Phase 2 — Go Binary Cleanup (1 day, senior review)**
Delete `internal/server/webui/`, `web/`, Makefile web targets. Add `/ui` deprecation redirect handler. Remove `scripts/smoke-ui.sh`. Tag rollback anchor `vYYYY.M.D.N-last-ui`.

**Phase 3 — Remaining Panels (4-6 days, incremental)**
Port Harvest, Dashboard, CodeSummarize, Symbols panels one PR each. Each PR is independently shippable.

**Phase 4 — GraphCanvas + React Flow (separate proposal)**
Build GraphCanvas abstraction, swap Sigma.js for React Flow. This is a separate OpenSpec change with its own design.

### Conflict Resolution Log

| Topic | Metis | Oracle | Cross-critique result | Confidence | Decision |
|-------|-------|--------|----------------------|------------|----------|
| MVA scope | Ship all 8 panels | Ship all 8 panels | Metis HIGH: "all 8 panels" not minimal, risk of all-or-nothing | MED | **Ship 2-3 panels first** — iterate. Metis's scope concern valid even though Oracle's architecture is sound |
| CSRF middleware | Not mentioned | Not mentioned | Metis HIGH: bypass fragile with proxy port mismatch | HIGH | **Add loopback-any-port CSRF rule** — one-line fix, prevents silent 403s |
| localStorage workspace hash | Not mentioned | Not mentioned | Oracle HIGH: every user loses workspace selection on port change | HIGH | **First-run workspace prompt** — detect empty selection, show workspace picker |
| Security headers | Missing requirement | CSP to meta tag | Metis MED: X-Content-Type-Options can't be meta tag | HIGH | **Proxy sets all headers explicitly** — not just CSP meta tag |
| API version contract | Mentioned missing | Port types.ts | Oracle HIGH: API contract drift is ongoing risk | MED | **Add /api/version endpoint + document compatibility range** — no OpenAPI yet |
| SSE streaming | Missing requirement | "Must test" | Both MEDIUM: non-trivial, needs explicit verification | HIGH | **Phase 0 spike must verify SSE** — blocker for everything else |
| Proxy server tech | Didn't specify | Express + http-proxy-middleware | Metis MED: flagged sirv vs express discrepancy | MED | **Use Express** — more proven for SSE, aligns with Oracle |
| Two-repo cost | Scope risk | Two repos recommended | Metis LOW: costs not weighed | LOW | **Accept two repos** — stated decision, cost is manageable |

### Key Risks & Mitigations

- **CSRF 403 through proxy** (source: Metis cross-critique, HIGH) → Add loopback-any-port rule to `middleware/csrf.go`. Test POST through proxy with and without `X-Requested-With`.
- **localStorage workspace loss** (source: Oracle cross-critique, HIGH) → On first load, if no workspace selected, prompt user with workspace picker. Call `GET /api/v1/workspaces` and render selection UI.
- **SSE proxy buffering** (source: both agents, HIGH) → Phase 0 spike must verify EventSource reconnects. Use `http-proxy-middleware` with `selfHandleResponse: false` (default) and test long-lived connections.
- **Security header regression** (source: Metis cross-critique, MEDIUM) → Proxy server sets `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: same-origin` on all responses. CSP as `<meta>` tag in `index.html`.
- **API contract drift** (source: Oracle cross-critique, HIGH) → Add `/api/version` endpoint. Dashboard displays version banner. Document supported API range in README.
- **Embed transition confusion** (source: Oracle cross-critique, MEDIUM) → During Phase 2, `/ui` serves deprecation page pointing to new dashboard. Don't leave stale embedded UI accessible.

### Open Questions for User (LOW confidence — need your input)

1. **Dashboard port default**: 5173 (matching Vite dev) or 5173 with auto-allocate? Oracle recommended 5173, Metis suggested auto-detect from 3101+.
2. **npm package name**: `@nano-step/nano-brain-dashboard` only, or also `nano-brain-dashboard` unscoped alias? Current pattern has both for `nano-brain`.
3. **Node.js requirement acceptance**: The dashboard requires Node.js even though nano-brain is a Go binary. Is this acceptable, or should we explore a bundled binary approach?

### Security & Performance Notes

- **Proxy overhead**: ~1ms per request on localhost — negligible
- **CSP**: Port to `<meta>` tag in `index.html` + proxy sets `X-Content-Type-Options` and `X-Frame-Options` as HTTP headers
- **SSE**: Must verify through proxy — test with 60s+ long-lived connections, verify reconnection
- **Auth**: Proxy forwards `Authorization` headers transparently. No dashboard-side auth needed.
- **No new network surface**: Dashboard runs on localhost only. Same threat model as current Vite dev server.
