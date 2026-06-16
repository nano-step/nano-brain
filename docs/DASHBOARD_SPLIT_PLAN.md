# Plan — Split the dashboard into its own repo; nano-brain becomes API-only

Status: Proposal · Supersedes/absorbs `docs/G6_MIGRATION_PLAN.md` (G6 is now built into the new dashboard from day one).

## 0. Decisions locked in

| Decision | Choice |
|---|---|
| Distribution | **Hosted SPA** (HTTPS) that talks to the user's **`http://localhost:3100`** API |
| Rebuild approach | **Greenfield**, keep the proven stack (Vite + React 18 + TanStack) |
| Charts | **G6 from day one** (Flow + Graph); no Mermaid/Sigma in the new repo |
| `/ui` in nano-brain | **Hard-remove** once the new dashboard reaches parity |

## 1. Why / goals

- Decouple UI from the binary: drop the embedded `dist` (`//go:embed all:dist`, **4.2 MB**) and remove the entire web toolchain from the Go build/release. (The binary is ~58 MB and is large mostly from Go deps — expect ~4 MB off, plus a much simpler Go build, not a dramatic size cut.)
- Independent release cadence and a clean API boundary.
- Freedom to rebuild a better dashboard on G6.

## 2. Current state

- React app embedded via `internal/server/webui/embed.go` (`//go:embed all:dist`), mounted at `/ui` in `routes.go:137` with `SecurityHeaders()`.
- Stack (`web/package.json`): React 18, Vite 5, TanStack Query/Router/Table/Virtual, zustand, react-hook-form+zod, react-markdown, **mermaid** (Flow), **sigma + graphology + forceatlas2** (Graph), lucide, cmdk, fuse.
- Panels: Dashboard, Memory, Graph, Flow, Symbols, Harvest, Settings, Workspaces, CodeSummarize.
- API: same-origin today → **no CORS middleware**, CSRF on a subset, auth off by default, bind-safety gate. Hand-written types in `web/src/api/types.ts`.

## 3. Target architecture

```
┌─────────────────────────────┐         HTTPS page → http://localhost:3100 (loopback)
│ nano-brain-dashboard (repo)│  ───────────────────────────────────────────────►  ┌────────────────────┐
│ hosted HTTPS SPA            │   CORS + PNA preflight + API token (Authorization) │ nano-brain (API)   │
│ Vite+React+TanStack+G6      │  ◄───────────────────────────────────────────────  │ localhost only     │
└─────────────────────────────┘                                                    └────────────────────┘
```

- **nano-brain** = headless API + MCP. No `/ui`, no embedded assets, no `web/`.
- **nano-brain-dashboard** = static SPA hosted on a CDN (Vercel/Netlify/Pages); runs in the user's browser; connects to their local API.
- Contract between them: a **versioned REST API** with a generated typed client (OpenAPI), and the existing `X-Nano-Brain-Version` header for compatibility checks.

## 4. The hard part — hosted HTTPS SPA → `http://localhost` (gates the whole approach)

This is the riskiest decision and must be de-risked in **Phase 0** before anything else.

- **Mixed content:** a secure (HTTPS) page making a plaintext `http://` request is blocked. Loopback is special-cased, but reliably only via Chrome's **Private Network Access permission prompt (Chrome 124+)**. ([Chrome PNA permission prompt](https://developer.chrome.com/blog/pna-permission-prompt-ot-end))
- **Private Network Access preflight:** public→private requests send an `OPTIONS` preflight with `Access-Control-Request-Private-Network: true`; the API must answer `Access-Control-Allow-Private-Network: true` (+ standard CORS). ([PNA preflights](https://developer.chrome.com/blog/private-network-access-preflight))
- **Purpose = anti-CSRF on local devices.** PNA exists precisely to stop websites from attacking localhost services — which is the threat model we opt into by design. ([CORS for private networks](https://developer.chrome.com/blog/cors-rfc1918-feedback))
- **Cross-browser:** Safari/Firefox do not implement PNA the same way; behavior must be validated per target browser. **This may make the hosted approach Chromium-only in practice.**

**Mitigations to evaluate in Phase 0 (pick before committing):**

1. **Backend CORS + PNA support** (required regardless): strict origin allowlist (only the dashboard's exact origin + `http://localhost:*` for dev), respond to PNA preflights, support credentials via header token (not cookies).
2. **Optional local TLS for the API** (`https://localhost:3100` with a bundled/locally-trusted cert) → eliminates mixed content (https→https). Trade-off: cert UX on localhost is painful.
3. **Documented fallback:** ship the SPA build so it can also be run locally (`npx`/static) for users whose browser blocks the hosted→localhost path. Cheap insurance.

If Phase 0 shows the hosted→localhost UX is too fragile cross-browser, the fallback (run the static build locally) is the escape hatch — same code, different host.

## 5. Security model (non-negotiable given §4)

Opening the API to a hosted origin + reaching localhost is the exact scenario PNA defends against, so:

- **Mandatory API token** for cross-origin use. Use `Authorization: Bearer <token>` (header, not cookies → sidesteps SameSite/CSRF). The user generates a token (`nano-brain auth …`) and pastes it into the dashboard's connection settings (stored in the SPA, never transmitted to the dashboard's own host).
- **Strict CORS origin allowlist**, configurable; default = the official dashboard origin + localhost dev origins. Never `*`.
- **PNA preflight** handled explicitly.
- **Keep bind-safety**: the API still binds loopback only; the browser (on the user's machine) reaches it. Nothing is exposed to the network.
- Re-evaluate the existing partial CSRF middleware — with token auth + strict CORS it may be replaced by a uniform origin/token check.

## 6. Backend changes in nano-brain

1. **New middleware:** CORS (allowlist) + PNA (`Access-Control-Allow-Private-Network`) + token-auth enforcement for cross-origin requests. Config-driven (`server.cors.allowed_origins`, `server.auth`).
2. **Flow JSON payload:** add `nodes[]`/`edges[]` (roles, kind, conditional) to `GraphFlow` + MCP `memory_flow` so G6 can consume it. (Keep `mermaid` for MCP/markdown.)
3. **API contract:** publish an **OpenAPI spec** + generate a TS client for the dashboard (replaces hand-written `types.ts`). Add an API version/compat endpoint (header already exists).
4. **Remove `/ui`:** see the literal checklist in §13 (it's larger than first stated — committed `dist/`, `vite.config.ts` outDir, `scripts/smoke-ui.sh`, orphaned `SecurityHeaders()`). Note: **CI has no web-build step** to remove (the `dist/` is committed and embedded), correcting an earlier claim. Do this in the final phase.

## 7. New repo: `nano-brain-dashboard`

- **Stack:** Vite 5 + React 18 + TanStack Query/Router/Table + zustand + react-hook-form/zod + react-markdown + **@antv/g6 v5** + lucide + cmdk. (Drop mermaid, sigma, graphology.)
- **Structure:** `src/api` (generated client + connection/auth store), `src/components/graph/GraphCanvas` (shared G6 wrapper + adapters + layouts + styles), `src/panels/*` (rebuilt), `src/app` (router/layout/theme), design tokens.
- **Connection & auth UX:** first-run screen to set API base URL (default `http://localhost:3100`) + paste API token; live health check + version-compat banner; clear error states for blocked mixed-content/PNA.
- **Panels:** rebuild Dashboard, Memory, Workspaces, Symbols, Harvest, Settings, CodeSummarize (port working logic, new design system); **Graph + Flow on G6** (the consolidation — dagre layout for flow, force for the knowledge graph; legend, collapse/expand, filter-by-role, neighborhood focus, position cache).
- **Build/host:** static build to a CDN host (Vercel/Netlify/GitHub Pages); env `VITE_API_BASE`; also produce a portable build runnable locally (fallback from §4).
- **CI/versioning:** own pipeline (typecheck/lint/test/build/deploy); document the nano-brain API version range each dashboard release supports.

## 8. Phased rollout

| Phase | Deliverable | Exit criteria |
|---|---|---|
| **0 — Viability spike** (gating) | Prototype: hosted HTTPS page hitting `http://localhost` API with CORS+PNA+token, across Chrome/Safari/Firefox; G6-vs-React-Flow check | Hosted→localhost works (or fallback chosen); auth/CORS/PNA design signed off |
| **1 — API boundary** | nano-brain: CORS+PNA+token middleware; flow `nodes/edges`; OpenAPI + TS client; version/compat | Cross-origin request from spike SPA succeeds with token; existing clients unaffected |
| **2 — New repo scaffold** | `nano-brain-dashboard` repo: stack, routing, design system, API client, connection/auth UX, `GraphCanvas` skeleton | App boots, authenticates, shows health/version |
| **3 — Panel parity** | Port non-graph panels (Memory/Workspaces/Symbols/Harvest/Settings/Dashboard/CodeSummarize) | Functional parity with current `/ui` |
| **4 — Graph + Flow on G6** | `GraphCanvas` for both graph views; adapters; interactions | Parity + interactivity; no Mermaid/Sigma in repo |
| **5 — Host & release** | Deploy SPA; CI; docs; portable local-run fallback | Public dashboard reachable; install/connect docs |
| **6 — Remove `/ui`** | Tag a final nano-brain release **with** `/ui` (rollback anchor); then delete `webui/`, embed, `web/`, Makefile/CI web steps | Binary builds without web toolchain; docs point to new dashboard |

## 9. Risks & mitigations

- **Hosted→localhost blocked (mixed content / PNA / cross-browser)** → Phase 0 gate; CORS+PNA backend; optional local TLS; portable local-run fallback. *Highest risk.*
- **Security exposure (website reaching localhost)** → mandatory token auth + strict origin allowlist + PNA; loopback-only bind. *Treat as a security-review (high-risk) change.*
- **Hard-remove has no overlap window** → tag a "last release with `/ui`" as a rollback anchor; don't remove until the new dashboard is at parity and hosted.
- **API contract drift across two repos** → OpenAPI + generated client + version-compat header/banner.
- **Two-repo coordination overhead** → document supported API version ranges; CI compat check.

## 10. Effort (rough)

Phase 0 ≈ 2–4 days (spike is decisive). Phase 1 ≈ 2–3 days. Phase 2 ≈ 2–3 days. Phase 3 ≈ 4–6 days. Phase 4 ≈ 4–5 days (G6 for two views). Phase 5 ≈ 1–2 days. Phase 6 ≈ 1 day. **Total ≈ 16–24 dev-days.**

## 11. Harness / process

- Multi-repo + API-contract + auth/CORS changes → **OpenSpec-first**, **high-risk lane** (auth, public-api-contract, audit-security gates). GitHub issue → proposal → design (Phase 0 findings feed it) → implement → validation ladder.
- Rollback: feature-gate the new middleware; keep the tagged "last-with-`/ui`" release until parity is confirmed.

## 12. What I need from you to finalize

1. **Dashboard origin/host** (e.g. `dashboard.nano-brain.dev` on Vercel?) — needed for the CORS allowlist.
2. **Auth approach confirmation** — token-paste is the safe default; OK?
3. **Phase 0 owner / timebox** — the hosted→localhost spike decides viability; want me to scope it in detail next?

---

## 13. Review addendum — corrections from independent technical review

An independent review against the code changed several decisions. The
step-by-step build (`docs/DASHBOARD_SPLIT_JUNIOR_GUIDE.md`) follows the corrected
path below, not the original §0 choices, because the originals are not viable for
a junior to ship working software.

- **Distribution flipped to local-served (default).** Hosted HTTPS → `http://localhost`
  is **not viable on Safari/Firefox** (no PNA) and relies on an in-flux Chromium
  feature; mixed-content failures are also **not catchable in JS** (undiagnosable
  UX). Default instead to **`npx nano-brain-dashboard` serving the SPA on loopback
  with a built-in proxy to the API** → same-origin from the browser's view, so
  **no CORS, no PNA, no mixed content, and SSE keeps working**. Hosted-HTTPS becomes
  an optional, Chromium-only convenience built last (if at all).
- **SSE/auth gap (blocker).** Native `EventSource` (`web/src/hooks/useEvents.ts`)
  cannot send an `Authorization` header, so live events break under cross-origin
  token auth. The proxy/local-served model sidesteps this; if hosted is pursued,
  SSE must move to fetch-streaming.
- **Security work is three senior-reviewed tasks, not one bullet** (only needed if
  hosted is pursued): CORS+PNA middleware, a **rewrite of CSRF** (kill the
  `X-Requested-With: nano-brain-ui` bypass in `middleware/csrf.go`), and a startup
  gate that refuses *CORS-enabled-without-auth*. Token auth already exists
  (`middleware/auth.go`, `nano-brain auth token`) but is a flat list with no
  revocation.
- **Drop OpenAPI from the critical path.** Port the existing, battle-tested
  `web/src/api/types.ts` as the contract. OpenAPI/codegen is optional later.
- **Charts: React Flow recommended for a junior;** G6 kept as the stated target
  behind a thin renderer interface so it can be swapped. G6 v5 is imperative/canvas
  with sparse docs — high risk for a junior.
- **Effort re-baselined for a junior:** ~30–45 days on the corrected (local-served,
  no-OpenAPI) path; the original hosted+G6+OpenAPI scope is not junior-appropriate.
- **`/ui` removal inventory corrected** (see §13 checklist in the guide): includes
  the committed `internal/server/webui/dist/`, the `vite.config.ts` `outDir`
  coupling, `scripts/smoke-ui.sh` + its harness ladder entry, and the now-orphaned
  `middleware/SecurityHeaders()`. **CI has no web-build step.**

### Sources (browser security mechanisms)

- [Private Network Access: introducing preflights — Chrome for Developers](https://developer.chrome.com/blog/private-network-access-preflight)
- [PNA permission prompt (mixed-content relaxation, Chrome 124+) — Chrome for Developers](https://developer.chrome.com/blog/pna-permission-prompt-ot-end)
- [Private Network Access update / deprecation trial — Chrome for Developers](https://developer.chrome.com/blog/private-network-access-update)
- [Feedback wanted: CORS for private networks (RFC1918) — Chrome for Developers](https://developer.chrome.com/blog/cors-rfc1918-feedback)
