## 1. SQL aggregates

- [x] 1.1 Add `CountChunksByWorkspace :one` to `internal/storage/queries/chunks.sql`
- [x] 1.2 Add `CountEmbeddingsByWorkspace :one` to `internal/storage/queries/embeddings.sql`
- [x] 1.3 Run `sqlc generate` to regenerate Go bindings
- [x] 1.4 Verify regenerated queries have correct signature

## 2. Stats handler refactor

- [x] 2.1 Convert `Stats` function into `StatsHandler` struct with injected context (queries, version, startTime, embedding cfg, getCfg func, migrationVersion, harvestStatus getter)
- [x] 2.2 Define new response struct `statsResponse` matching all 14 frontend fields
- [x] 2.3 Define nested structs: `embeddingInfo`, `chunksByEmbedStatus`, `graphEdgesByType`, `tagCount` (with `count` field), `harvestInfo`, `watcherInfo`, `collectionItem`, `recentDoc`
- [x] 2.4 Implement `Handle(c echo.Context) error` method
- [x] 2.5 Transform array results from CountChunksByEmbedStatus → object keyed by status
- [x] 2.6 Transform array results from CountGraphEdgesByType → object keyed by edge_type
- [x] 2.7 Populate server context fields (version, uptime, embedding, migration_version)
- [x] 2.8 Populate aggregate totals (docs_total, chunks_total, embeddings_total)
- [x] 2.9 Populate harvest + watcher info from server context (placeholder zeros where data not yet tracked)

## 3. Route wiring

- [x] 3.1 Update `server.New` to construct `StatsHandler` with full context
- [x] 3.2 Update `routes.go` registration: `data.GET("/stats", s.statsHandler.Handle)`
- [x] 3.3 Verify build passes

## 4. Tests

- [x] 4.1 Update `stats_test.go` mock interface to match new StatsHandler dependencies
- [x] 4.2 Add `TestStats_ResponseShape` asserting all 14 top-level keys + nested object shapes
- [x] 4.3 Add `TestStats_EmptyWorkspace` asserting zeros for empty workspace
- [x] 4.4 Add `TestStats_ChunksByEmbedStatusIsObject` asserting object shape with pending/embedded/embed_failed keys
- [x] 4.5 Update any failing existing tests to match new shape

## 5. Verification

- [x] 5.1 `go build ./...` exit 0
- [x] 5.2 `go vet ./...` clean
- [x] 5.3 `go test -race -short ./internal/server/handlers/... ./internal/storage/...` all PASS
- [x] 5.4 Build dev binary, start on port 3199
- [x] 5.5 `curl /api/v1/stats?workspace=<hash>` returns shape matching spec
- [x] 5.6 Browser devtools: navigate /ui/, select workspace, dashboard renders without errors
- [x] 5.7 Screenshot evidence of working dashboard

## 6. PR + Review

- [x] 6.1 Commit: `fix(api): align /api/v1/stats response with frontend StatsResponse (#279)`
- [x] 6.2 Push branch `fix/279-stats-api-contract`
- [x] 6.3 Open PR with full E2E evidence
- [x] 6.4 Gemini triage in `docs/evidence/fix-stats-api-contract/gemini-triage.md`
- [x] 6.5 Address findings
- [x] 6.6 Squash merge with `--delete-branch`
- [x] 6.7 Close issue #279

## 7. Archive + Release

- [x] 7.1 Pull merged b-main
- [x] 7.2 `openspec archive fix-stats-api-contract --yes`
- [x] 7.3 Tag next v2026.6.X, push, verify release
