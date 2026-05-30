# deep-design report — Web UI

Captured during planning of the `web-ui` OpenSpec change. Run consisted of:
1. Five Wave-1 parallel discovery agents (explore × 3, librarian × 2).
2. Two Wave-2 architecture reviewers (Metis scope/risk, Oracle architecture) — see status below.
3. One huashu-design hi-fi mockup (status below).

## Oracle review — APPROVED WITH REVISIONS

**Verdict:** APPROVED WITH REVISIONS. Must-fix items 1–5 applied to design.md + tasks.md before any implementation. Items 6–8 strongly recommended (also applied).

### Critical (applied)

1. **Event bus location → import cycle.** Original design placed `Bus` in `internal/server/webui/`. Producers (`embed`, `watcher`, `harvest`) cannot import back into `server/webui` since `server` already imports them. **Fix applied:** move to `internal/eventbus/` (zero-dep package), wire via `eventbus.Publisher` interface at construction.
2. **Drop-oldest channel semantics are racy.** Go channels can't atomically drain-and-send. **Fix applied:** drop-newest (non-blocking send) + periodic `lag` event ticker. Fan-out goroutine pattern removes per-publisher mutex.
3. **XSS sanitization missing for memory note content.** Notes are user-generated. **Fix applied:** add `react-markdown` + `rehype-sanitize`, all rendering goes through `<SafeMarkdown>` component.

### Medium (applied)

4. **CSP / security headers missing.** **Fix applied:** `/ui` route group gets `Content-Security-Policy`, `X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy`. Not applied to `/api/v1/*` to avoid affecting CLI/MCP.
5. **`go install` users get empty UI.** **Fix applied:** runtime check for `dist/index.html`; if absent, serve instructional fallback page.
6. **Shiki blows bundle budget.** ~500 KB WASM. **Fix applied:** dropped from v1; Prism-only.
7. **YAML comment preservation feasibility.** Works with `yaml.v3` `yaml.Node`, but koanf doesn't expose Nodes. **Documented in design.md:** patch flow does file-read → yaml.v3 Decoder → Node walk → Encoder writeback → then trigger koanf reload. "Best-effort" caveat: newly added keys go to end of section.
8. **CSRF edge cases.** Original 2-rule middleware missed `Origin: null`, `Referer`-only requests, and same-origin browser direct API access. **Fix applied:** 7-step decision order documented in `web-ui-server/spec.md` and `tasks.md`.

### Validated (don't change)

- `//go:embed` SPA in single binary — correct (matches Grafana/Prometheus/Gitea).
- SSE over WebSocket — correct (server→client only, auto-reconnect).
- Vite over Next.js — correct (no SSR needed).
- CSS Modules over Tailwind — correct (matches "not generic admin" goal).
- Sigma.js over react-flow — correct (WebGL scales to 5k nodes).
- 500-node hard cap on graph neighborhood — correct (prevents hairball).
- BFS in app code vs recursive CTE — correct (cap enforceable mid-traversal).
- Workspace filtering at SSE read site (not publish) — correct (Bus stays topic-agnostic).
- Manual TS/Go type sync for v1 — correct (auto-gen premature for <10 endpoints).
- Bind-safety check for non-loopback — correct.

### Alternative approaches considered + rejected

| Alternative | Why rejected |
|---|---|
| HTMX server-rendered | Insufficient for graph viz + cmd palette + SSE multiplex. Would need JS anyway for Sigma. |
| Companion process | Violates single-binary constraint. |
| Tauri/Wails desktop | Massive build-chain expansion. Cross-platform burden. |
| Recursive CTE | Harder to enforce 500-node cap mid-query. |
| WebSocket | Adds upgrade complexity. No client→server stream needed. |
| Watermill/NATS | Over-engineered for single-process, <50 subscribers. |

## Metis review — status

Metis run cancelled after deadlocking in self-polling at 5m+ (suspected agent loop). Scope risks were addressed directly during planning:

- **Scope creep**: Embedding-viz panel and authentication explicitly deferred to v2 (proposal.md "Out of Scope").
- **Hidden requirements**: "Control" interpreted as CLI-equivalents + live config (NOT live state mutation beyond what CLI permits). MCP debugging, V1 migration UI, benchmark results explicitly out of scope for v1 — flagged as v2 candidates.
- **Destructive operations**: `reset-workspace`, `remove-workspace`, `reset-embeddings` exposed via UI but gated behind double-confirm modal in spec (web-ui-app needs explicit scenario — see Followup).
- **0.0.0.0 + `--unsafe-no-auth` posture**: documented in proposal Constraints and design Risk Register. A clearer warning banner in UI when bound non-loopback is a Followup.
- **Lane classification**: `high-risk` confirmed (touches `public-api-contract`, `authorization` hard gates).

### Followups (status)

- [x] Add destructive-operation double-confirm scenario to `web-ui-app/spec.md` — applied (lines covering delete doc, reset workspace, remove workspace).
- [x] Add a "non-loopback bound" banner spec to `web-ui-app/spec.md` — applied (red warning when server bound non-loopback).
- [ ] Re-run Metis in a fresh session (optional; Oracle review covered the critical gaps).

## huashu-design mockup — status: PRODUCED

Agent task ran for ~9 minutes apparently stuck on `todowrite`. Cancelled. Mockup produced directly by the orchestrator using the huashu-design methodology (dev-tool aesthetic, anti-AI-slop, single polished variant, speaker notes appendix) and the locked design tokens below.

**Artifact**: `openspec/changes/web-ui/mockup/index.html` — 65 KB, 1119 lines, React 18 + Babel (CDN for prototype convenience only), 22 components, all 6 panels interactive with mock data reflecting the real data model (memory docs + supersedes chains, symbols with kind/language/file:line, graph edges contains/imports/calls, opencode/claudecode harvest sessions, embed queue counters with simulated SSE tick), dark/light toggle, Cmd+K command palette, two-keystroke navigation (g+d/m/g/s/h/,), non-loopback bind banner toggleable, destructive-op double-confirm pattern wired into Delete buttons.

### Mockup design tokens (locked, applied)

| Token | Value |
|---|---|
| `--bg-primary` | `#0D0D0D` |
| `--bg-elevated` | `#141414` |
| `--text-primary` | `rgba(255,255,255,0.95)` |
| `--text-secondary` | `rgba(255,255,255,0.65)` |
| `--text-tertiary` | `rgba(255,255,255,0.45)` |
| `--accent-primary` | `#0070F3` |
| `--status-ok` | `#10B981` |
| `--status-warn` | `#F59E0B` |
| `--status-err` | `#EF4444` |
| `--border` | `rgba(255,255,255,0.08)` |
| `--font-body` | `Inter, system-ui, -apple-system, sans-serif` |
| `--font-mono` | `'JetBrains Mono', 'Fira Code', ui-monospace, monospace` |
| `--radius` | `0px` (sharp) — exceptions: `2px` on chips, `4px` on modals |
| `--space-1..8` | `4 8 12 16 24 32 48 96 px` (8px base grid + 4px micro) |

### Mockup acceptance criteria — verified

- ✅ Single self-contained HTML (React 18 + Babel CDN). 65 KB.
- ✅ 6 panels reachable via sidebar + Cmd+K.
- ✅ Mock data reflects real data model.
- ✅ Dark default, light toggle.
- ✅ Speaker Notes appendix (4 open questions for user).
- ✅ No Tailwind, no UI kit. Hand-rolled CSS using the tokens above.
- ✅ Syntax check passed (`node` brace/paren balance = 0).

## Momus final plan review — OKAY

Ran against `.sisyphus/plans/web-ui-plan.md`. Duration 1m 27s. Verdict: **OKAY**. All 8 referenced artifacts exist and contain exactly what's claimed. CSRF 7-step decision order in tasks.md matches the spec verbatim. Each task has enough context (file paths, handler signatures, endpoint contracts, spec scenarios as QA criteria) for a developer to start working. Validation ladder covers QA verification per harness requirements. No blocking issues found.

## Oracle verification pass — PASS (after 2 fixes)

After delivery, a second Oracle skeptical-verification pass was run. Two blocking issues flagged + fixed:

1. **SSE streaming spec inconsistency** (FIXED): `web-ui-streaming/spec.md` "Lag event is emitted under backpressure" scenario said "drop the oldest events" — contradicted design.md and tasks.md which adopted drop-newest after the first Oracle review. Scenario rewritten to "drop the newest event via non-blocking select-send" with the race-free justification inline.
2. **Evidence file stale** (FIXED): this file previously said the mockup was "deferred to next session" — contradicted by the 65 KB artifact on disk. Updated above.

### Non-blocking Oracle observations (not yet addressed)

- AC10 ("no regression: existing CLI/REST/MCP/tests unchanged") is implicitly covered by the validation ladder in `tasks.md` (`validate:quick`, `test:integration`) but has no explicit scenario in any spec delta. Future hardening: add an explicit "no regression" scenario to `web-ui-server/spec.md` listing each preserved endpoint surface as MODIFIED-(no behavior change).
- Metis (cancelled mid-run) and full Momus deep-design pipeline did not execute in their original form; Oracle's review caught the critical issues, and Momus's final review on the plan file is OKAY. Metis can be re-fired pre-implementation if the user wants a fresh scope/risk pass.

## What was delivered

- `openspec/changes/web-ui/proposal.md`
- `openspec/changes/web-ui/design.md` (revised with Oracle must-fixes)
- `openspec/changes/web-ui/tasks.md` (revised with Oracle must-fixes)
- `openspec/changes/web-ui/specs/web-ui-server/spec.md` (revised with Oracle must-fixes)
- `openspec/changes/web-ui/specs/web-ui-streaming/spec.md`
- `openspec/changes/web-ui/specs/web-ui-app/spec.md`
- `docs/evidence/deep-design-web-ui.md` (this file)
- `openspec validate web-ui --strict --no-interactive` → ✅ valid

## Scope extension (post-initial-delivery)

User asked: "Can summaries display Obsidian-style with links to other summaries?" After clarifying (4 options offered), user chose **Full graph view for memory + summaries**. Scope extended:

- **proposal.md**: added AC13 (Graph panel two-mode) + AC14 (inline wikilinks + backlinks). Updated lane to call out `data-model` gate (migration extends `graph_edges.edge_type` CHECK). Updated outcome + why.
- **design.md**: added Schema-migration section (Up/Down explicit), Link-extraction-pipeline section (`internal/links/` package, parse + extractor + resolver with LRU cache, where extraction runs, idempotency, escape syntax), Graph-panel two-modes section. Risk register grew by 6 entries covering rollback, wikilink injection, XSS via title, title cache size, stale backlinks after delete, old clients ignoring new edge type.
- **specs/web-ui-server/spec.md**: extended Graph neighborhood with `node_kind=symbol|doc` (5 scenarios incl. mode-specific cap, 422 on invalid kind, empty focus, edge_type filtering). Added Backlinks endpoint (3 scenarios). Added Wikilink resolution endpoint (3 scenarios).
- **specs/web-ui-app/spec.md**: Graph panel rewritten for dual-mode (6 scenarios incl. mode-switch focus persistence, per-mode position cache, double-click-opens-drawer). Added Inline-wikilinks requirement (6 scenarios incl. ID/title resolution, ambiguous, broken, escaped, XSS sanitization compose). Added Backlinks-in-drawer requirement (3 scenarios incl. empty state, snippet, live update).
- **tasks.md**: added Schema-migration section, Link-extractor package, Wire-extractor section, dual-mode graph handler, Backlinks/resolve endpoints, frontend `WikilinkRewriter` + `BacklinksList`, harvest panel routed through `DocDrawer`, npm dep `remark-wiki-link` (or hand-rolled), backend unit + integration tests for parser/resolver/backlinks, migration Up/Down/idempotency test.
- **mockup/index.html**: 65 KB → 77 KB. Added wikilink+backlinks indices, `renderWithWikilinks` helper, knowledge-mode graph data synthesized from RECENT_DOCS, dual-mode GraphPanel with toggle, Backlinks section in DocDrawer, Harvest rows now open the same drawer with a synthetic session-doc. Speaker notes updated. CSS classes `.wikilink` + `.wikilink-broken` added.
- Re-validated: `openspec validate web-ui --strict` → valid. Mockup brace/paren/bracket balance = 0.

## What's deferred to the implementation kickoff

- GitHub issue creation at intake gate (per `docs/FEATURE_INTAKE.md` Step 0) — deferred per user request "chưa cần implement".
- Re-fire Metis for a fresh scope/risk pass (optional; Oracle review covered the critical gaps; scope-extension Oracle pass pending).
- Add explicit "no regression" scenario to `web-ui-server/spec.md` (non-blocking Oracle observation).
- Implementation itself — when the user is ready, run `/opsx-apply` or work through `openspec/changes/web-ui/tasks.md` top-down.
