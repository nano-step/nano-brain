## 1. SQL queries

- [x] 1.1 Add `ListTopGraphNodesByDegree :many` to `internal/storage/queries/graph.sql`
- [x] 1.2 Add `CountDistinctGraphNodes :one` + `ListEdgesTouchingNodes :many` (replaces ListEdgesBetweenNodes — too restrictive, dropped 100% of edges in real workspaces)
- [x] 1.3 Run sqlc generate
- [x] 1.4 Verify regenerated bindings compile

## 2. Backend handler

- [x] 2.1 Create `internal/server/handlers/graph_overview.go` with `GraphOverview(q OverviewQuerier, logger) echo.HandlerFunc`
- [x] 2.2 Define request type matching spec
- [x] 2.3 Resolve mode → edge_types defaults (code: calls/imports/contains, knowledge: references)
- [x] 2.4 Clamp limit to [1, 200]
- [x] 2.5 Call ListTopGraphNodesByDegree → ListEdgesTouchingNodes (cap 400 edges)
- [x] 2.6 Collect implicit endpoint nodes from edges (so graph is connected, not just top-N hubs)
- [x] 2.7 Map to existing GraphNeighborhoodResponse shape
- [x] 2.8 Set truncated=true if distinct node count > limit

## 3. Routes

- [x] 3.1 Register `data.POST("/graph/overview", handlers.GraphOverview(s.queries, s.logger))` in routes.go

## 4. Frontend

- [x] 4.1 Add `useGraphOverview` mutation hook (sibling to useGraphNeighborhood)
- [x] 4.2 Update GraphPanel.tsx: useEffect on (mode, edgeTypes, workspace) with empty focus → fetchOverview
- [x] 4.3 Keep existing focus → fetchNeighborhood path
- [x] 4.4 Update empty state copy: only show "Enter symbol/doc" when overview returns 0 nodes

## 5. Tests

- [x] 5.1 TestGraphOverview_ResponseShape — wrapped fields per spec
- [x] 5.2 TestGraphOverview_CodeModeDefaults — edge_types from mode
- [x] 5.3 TestGraphOverview_KnowledgeModeDefaults
- [x] 5.4 TestGraphOverview_EmptyWorkspace — null-safe arrays
- [x] 5.5 TestGraphOverview_LimitClamping — 0/negative/>200
- [x] 5.6 TestGraphOverview_TruncatedFlag
- [x] 5.7 TestGraphOverview_IncludesImplicitEndpointNodes — verifies edges include implicit endpoint nodes

## 6. Verification

- [x] 6.1 go build ./... exit 0
- [x] 6.2 go vet clean
- [x] 6.3 go test -race -short ./... PASS
- [x] 6.4 Rebuild web (npm run build)
- [x] 6.5 Rebuild dev binary
- [x] 6.6 curl /api/v1/graph/overview returns nodes + edges (231 nodes + 400 edges, code mode next-app)
- [x] 6.7 Browser devtools: /ui/graph auto-loads graph (no input) — verified 166-node code graph rendered
- [x] 6.8 Switch Code/Knowledge tabs re-fetches — verified empty-state copy on knowledge mode
- [x] 6.9 Type focus → switches to neighborhood (no behavior change to focused path)

## 7. smoke:ui evidence (per #285 harness gate — this PR touches web/ + handlers/)

- [x] 7.1 Run scripts/smoke-ui.sh > docs/evidence/add-graph-overview-endpoint/smoke-ui-output.log
- [x] 7.2 Verify "=== smoke:ui PASS ===" in log

## 8. PR + Review

- [ ] 8.1 Commit + push
- [ ] 8.2 PR with E2E evidence
- [ ] 8.3 Gemini triage
- [ ] 8.4 Merge + close issue

## 9. Archive + Release

- [ ] 9.1 openspec archive add-graph-overview-endpoint
- [ ] 9.2 Tag v2026.6.9
