# Migration Plan — Unify graph visualization on AntV G6

Status: Proposal · Owner: TBD · Target lib: [`@antv/g6`](https://github.com/antvis/g6) v5 (pin latest 5.x at install)

## 1. Goal & scope

Replace the two independent graph-rendering stacks in the web UI with a single
AntV **G6 v5** layer, behind one shared `GraphCanvas` component. This removes the
Mermaid text round-trip (the source of the reserved-word / sanitization / parse
failures we've been fixing) and the Sigma+graphology stack, and gives one
interactive, themeable, large-graph-capable renderer for every graph view.

**In scope (graph visualizations):**

- **FlowPanel** — request/call flow, currently Mermaid `graph TD` / `sequenceDiagram`.
- **GraphPanel** — knowledge / dependency graph overview + neighborhood, currently Sigma.js + graphology + forceatlas2.

**Out of scope (not graphs — G6 is the wrong tool):**

- **DashboardPanel** sparkline `<svg>` + CSS progress bars. Leave as-is, or, if
  you want AntV consistency later, use `@antv/g2` separately. Not part of this plan.

**Explicitly retained:** the backend **Mermaid/text serializer** (`internal/flow`)
stays for the MCP `memory_flow` tool and markdown/PR embedding. Agents and docs
consume text; only the *human* UI moves to G6. Same `Flow` source of truth feeds
both a text serializer and the JSON graph payload.

## 2. Current state inventory

| Area | Today | Feeds from |
|---|---|---|
| Flow render | `mermaid@11` in `web/src/panels/FlowPanel.tsx` (`mermaid.render`) | `POST /api/v1/graph/flow` → returns `mermaid` string (+ `chain[]`, `externals[]`, **no edges**) |
| Graph render | `sigma@3` + `graphology@0.25` + `graphology-layout-forceatlas2` in `web/src/panels/graph/SigmaGraph.tsx` | `POST /api/v1/graph/overview`, `POST /api/v1/graph/neighborhood` → structured `GraphNode[]`/`GraphEdge[]` |
| Position cache | `web/src/panels/graph/usePositionCache.ts` | localStorage-ish in-memory |
| Legend / colors | `web/src/panels/graph/GraphLegend.tsx`, color maps in `SigmaGraph.tsx` | edge_type / collection |
| Dashboard | inline `<svg>` sparkline + progress bars | `useStats` (out of scope) |

Backend serializers live in `internal/flow/` (`mermaid.go`, `sequence.go`,
`builder.go`). Graph overview/neighborhood handlers already return structured
JSON — G6-ready with no backend change.

## 3. Target architecture

```
                         ┌─ text: internal/flow Render() → Mermaid string ── MCP memory_flow / markdown
Flow (Flow struct) ──────┤
                         └─ json: nodes[] + edges[] (roles, kind, conditional) ─┐
                                                                                ├─→  GraphCanvas (G6 v5)
Graph overview/neighborhood ── json: GraphNode[] / GraphEdge[] ─────────────────┘     (web/src/components/graph/)
```

- **One shared component** `GraphCanvas` wraps a G6 `Graph` instance and accepts a
  normalized `{ nodes, edges }` model plus a `layout` and `styleProfile`.
- **Adapters** convert each source into that model:
  `flowToGraphModel(flowResponse)` and `apiGraphToModel(GraphNode[], GraphEdge[])`.
- **Layouts:** flow → `antv-dagre` (directed, ranked); knowledge/dependency graph
  → `force`/`d3-force` (replaces forceatlas2). Both are built into G6 v5.
- **Behaviors:** `zoom-canvas`, `drag-canvas`, `drag-element`, `hover-activate`,
  `collapse-expand`, plus a click handler bridging to the existing `DocDrawer`.
- **Styling:** node/edge style mappers keyed by role (entry/handler/service/repo/
  external/integration) and edge kind (calls/imports/contains/http/middleware),
  reusing the current color constants and reading theme tokens from `styles/tokens.css`.

## 4. Why G6 (brief, balanced)

- Kills the entire Mermaid text-grammar failure class (reserved IDs, label
  escaping, `<var:…>`/`<br/>`, duplicate-edge text) — G6 consumes JSON.
- One engine for both panels → drop `mermaid`, `sigma`, `graphology`,
  `graphology-layout-forceatlas2` (4 deps for 1).
- Interactivity that directly addresses the "too much / redundant" problem:
  collapse/expand by depth, filter by role/edge-kind, search, focus-neighborhood.
- Trade-offs: G6 v5 has a learning curve and a different API from v4; canvas
  rendering means no cheap text-snapshot tests (mitigated by keeping the text
  serializer + model-level unit tests). **React Flow** is a viable alternative
  (tighter React ergonomics, weaker built-in graph algorithms) — decide in Phase 0.

## 5. Backend changes (small, additive, non-breaking)

1. **Flow JSON payload.** Add `nodes[]` and `edges[]` to `flowResponse`
   (`internal/server/handlers/flow.go`) and the MCP `memory_flow` result — the
   `Flow` struct already holds `Nodes`/`Edges` with `Role`, `Kind`, `Conditional`,
   `Line`. Keep `mermaid`, `chain`, `externals` for backward compatibility.
2. **No change** to `/graph/overview` and `/graph/neighborhood` — already structured.
3. Optional: a shared edge/node DTO so flow and graph payloads share a shape the
   single adapter can consume.

## 6. Frontend changes

- **Add:** `@antv/g6` (v5). **Remove (end state):** `mermaid`, `sigma`,
  `graphology`, `graphology-layout-forceatlas2`.
- **New:** `web/src/components/graph/GraphCanvas.tsx` (G6 wrapper), `layouts.ts`,
  `styles.ts` (role/kind → visual), `adapters.ts` (flow + api → model).
- **Migrate `GraphPanel`** to `GraphCanvas` first (it already has structured data
  and is the higher-value, more-used view). Port `GraphLegend`, color maps, and
  `usePositionCache` (G6 supports fixed node positions / layout pinning).
- **Migrate `FlowPanel`** to `GraphCanvas` using the new flow JSON payload; drop
  the `mermaid.render` path. Keep a "copy Mermaid" affordance (from the retained
  text field) for users who want to paste into docs.
- **Tests:** replace Mermaid/Sigma DOM assertions with (a) adapter unit tests
  (pure model transforms — high value, easy) and (b) light G6 mount smoke tests
  (jsdom + canvas stub or skip canvas, assert data wiring).

## 7. Phased rollout

| Phase | Deliverable | Exit criteria |
|---|---|---|
| **0 — Spike & decision** | 1–2 day spike: G6 v5 vs React Flow on a real overview graph; confirm layout quality, bundle size, theming, jsdom testability | Library chosen; `GraphCanvas` API sketched; OpenSpec proposal opened |
| **1 — Backend payload** | Flow API + MCP return `nodes[]`/`edges[]` (additive); DTO finalized | Existing clients unaffected; new fields covered by Go tests |
| **2 — Shared component** | `GraphCanvas` + adapters + layouts + styles + legend, behind a feature flag | Renders both sample datasets; unit tests on adapters green |
| **3 — GraphPanel cutover** | GraphPanel uses `GraphCanvas`; Sigma path removed | Visual parity + position cache + neighborhood expand work; GraphPanel tests updated |
| **4 — FlowPanel cutover** | FlowPanel uses `GraphCanvas`; Mermaid render removed from UI | `GET /balance` & friends render interactively; no parse-failure fallbacks |
| **5 — Cleanup** | Remove `mermaid`/`sigma`/`graphology`(+layout) deps; delete dead code; docs | `npm ls` clean; bundle size measured; CHANGELOG + docs updated |

Feature-flag phases 2–4 (`?renderer=g6`) so old and new render side-by-side until parity is signed off.

## 8. Testing & verification

- **Go:** `go test -race -short ./internal/flow/... ./internal/server/handlers/...`
  for the new payload; keep the text-serializer golden tests.
- **Web:** `npm run typecheck && npm run lint && npm run test`; new adapter unit
  tests; mount smoke tests for `GraphCanvas`.
- **Manual / e2e:** the existing tunnel + browser harness — load each panel,
  confirm parity vs the flagged-off renderer, exercise zoom/expand/filter.
- **Perf:** measure render time + bundle size before/after on the largest
  workspace (zengamingx) at depth 4.

## 9. Risks & mitigations

- **G6 v5 API churn / learning curve** → Phase 0 spike; pin exact version.
- **Layout quality on large/cyclic graphs** → evaluate dagre vs force in spike;
  cap nodes (reuse the depth/fanout/reconciliation limits already in the builder).
- **Loss of text snapshots** → keep the backend Mermaid serializer + add
  model-level adapter tests.
- **Bundle size** → tree-shake G6 v5 (modular), lazy-load the graph panels.
- **MCP regression** → the text path is untouched; assert `memory_flow` still
  returns `mermaid`.
- **Canvas + jsdom testability** → test adapters (pure) heavily; smoke-test mount only.

## 10. Effort (rough)

- Phase 0: ~1–2 days. Phase 1: ~0.5 day. Phase 2: ~2–3 days. Phase 3: ~2 days.
  Phase 4: ~1–2 days. Phase 5: ~0.5 day. **Total ≈ 7–10 dev-days**, parallelizable
  after Phase 2.

## 11. Harness / process alignment

- This is multi-file and touches an API contract → **OpenSpec-first** (`/opsx-propose`),
  likely **normal/high-risk** lane (public-api-contract gate for the flow payload).
  Create the GitHub issue first, then the proposal, per `docs/HARNESS.md`.
- Validation ladder per phase: `validate:quick` always; `test:integration` +
  `smoke:e2e` for the cutover phases.
- **Rollback:** the feature flag (`renderer=g6`) and the retained Mermaid/Sigma
  code until Phase 5 make rollback a flag flip; Phase 5 is the only irreversible step.

## 12. Open decisions (resolve in Phase 0)

1. **G6 v5 vs React Flow** — recommend G6 for built-in layouts/algorithms; React
   Flow if React ergonomics + custom node React components matter more.
2. **Keep backend Mermaid for MCP?** — recommend **yes** (agents/docs need text).
3. **Dashboard sparkline** — leave out of scope (not a graph).
4. **graphology** — drop entirely (G6 has its own model) vs keep as a data layer
   feeding G6 — recommend drop to avoid two graph models.
