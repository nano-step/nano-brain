# Self-Review Evidence — Story 9.7: Graph Panel Dual-Mode

Date: 2026-05-30
Story: 9.7 — Graph panel dual-mode (Code / Knowledge) with Sigma.js + Graphology lazy-load
Issue: #250
Branch: feat/9.7-graph-panel

---

## Validation Ladder

| Layer | Command | Result |
|---|---|---|
| `validate:quick` (build) | `go build ./...` | ✅ exit 0 |
| `validate:quick` (test) | `go test -race -short ./...` | ✅ all packages pass |
| `self-review:staged-files` | `git status` — no `.opencode/`, no unintended files | ✅ clean |
| `test:integration` | `go test -race -tags=integration ./...` | ✅ all pass (1 pre-existing failure in `TestEventsIntegration_ReindexPublishesSequence` — confirmed pre-existing on base branch) |
| `golangci-lint` | `golangci-lint run ./...` | ✅ 0 new warnings (3 pre-existing — confirmed pre-existing on base) |
| vitest | `npm run test` (in `web/`) | ✅ 37/37 passed |
| `smoke:e2e` (SPA serving) | `go test ./internal/server/webui/...` + dist asset inspection | ✅ sigma chunk NOT in index.html (lazy), SPA fallback handles `/ui/graph` |

---

## Pre-existing Issues (Not Introduced by This Story)

| Issue | Location | Evidence |
|---|---|---|
| `TestEventsIntegration_ReindexPublishesSequence` fails | `internal/server/handlers/events_test.go:96` | Reproduced on base branch (git stash + run) |
| `golangci-lint` S1011 in `graph_neighborhood.go:171` | `internal/server/handlers/graph_neighborhood.go` | Reproduced on base branch (git stash + run) |
| `golangci-lint` unused funcs in `events_test.go` | `internal/server/handlers/events_test.go` | Reproduced on base branch (git stash + run) |

---

## Bundle Budget Verification

| Metric | Value | Budget | Pass? |
|---|---|---|---|
| Sigma chunk (gzip) | 39.59 kB | ≤ 200 kB | ✅ |
| Initial bundle total (gzip) | ~94 kB | ≤ 600 kB | ✅ |
| Sigma chunk in index.html? | No (lazy via React.lazy) | Must be absent | ✅ |

---

## Acceptance Criteria Checklist

| AC | Pass? |
|---|---|
| Dual-mode toggle renders (Code / Knowledge) | ✅ |
| Per-mode state preserved on toggle-back | ✅ |
| Focus input clears on mode switch | ✅ |
| Depth chips [1-5] render and update state | ✅ |
| Direction chips [in/out/both] render and update state | ✅ |
| Edge-type chips render per mode | ✅ |
| Truncated badge renders (truncated: false / true + frontier count) | ✅ |
| SigmaGraph lazy-loaded (not in initial bundle) | ✅ |
| ForceAtlas2 layout positions persisted to localStorage | ✅ (unit-tested) |
| Positions restored from cache on repeat visit | ✅ (unit-tested) |
| Hover tooltip on node | ✅ (Sigma `enterNode` event) |
| Double-click Code → navigate to /ui/symbols | ✅ |
| Double-click Knowledge → DocDrawer stub (9.6 parallel worktree) | ✅ stub |
| GraphLegend renders per mode | ✅ |
| No new Go code touched | ✅ (pure frontend change) |
| No sigma/graphology in initial JS bundle | ✅ verified via index.html inspection |

---

## Files Changed

```
web/src/panels/GraphPanel.tsx              NEW — main panel
web/src/panels/graph/SigmaGraph.tsx        NEW — heavy chunk (lazy-loaded)
web/src/panels/graph/GraphLegend.tsx       NEW — mode-aware legend
web/src/panels/graph/useGraphNeighborhood.ts  NEW — TanStack mutation hook
web/src/panels/graph/usePositionCache.ts   NEW — localStorage cache
web/src/app/router.tsx                     MOD — /graph → GraphPanel
web/src/api/types.ts                       MOD — graph type definitions
web/vite.config.ts                         MOD — sigma manual chunk
web/src/__tests__/GraphPanel.test.tsx      NEW — 12 tests
web/src/__tests__/usePositionCache.test.ts NEW — 5 tests
internal/server/webui/dist/               REGEN — built output (vite build)
web/package.json / package-lock.json       MOD — sigma, graphology, fa2 deps
```
