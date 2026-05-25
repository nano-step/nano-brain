# Knowledge Graph — Tasks

## T1: Migration + sqlc (schema)

- [ ] Write `migrations/00008_knowledge_graph.sql` (up: CREATE TABLE + indexes; down: DROP TABLE)
- [ ] Write `internal/storage/queries/graph.sql` (UpsertGraphEdge, DeleteGraphEdgesByFile, GetOutgoingEdges, GetIncomingEdges, GraphStats)
- [ ] Run `sqlc generate` and verify generated types in `internal/storage/db/`
- [ ] Validate: `goose up && goose down && goose up` against test DB

## T2: internal/graph/ package

- [ ] `internal/graph/edge.go`: Edge struct, EdgeKind constants, Extractor interface
- [ ] `internal/graph/registry.go`: Registry struct, `ExtractEdges()` dispatch
- [ ] `internal/graph/go_extractor.go`: GoGraphExtractor implementing Extractor
  - Contains edges: query existing symbol DB or parse top-level declarations
  - Import edges: tree-sitter query `(import_spec path: (interpreted_string_literal) @path)`
  - Call edges: two-pass (function ranges + call_expression callee), best-effort
- [ ] `internal/graph/testdata/`: Add small multi-file Go fixtures with known edge counts
- [ ] `internal/graph/go_extractor_test.go`: unit tests for all three edge types

## T3: Storage interface extension

- [ ] Add `GraphStore` interface to `internal/storage/` (or extend existing store interface)
- [ ] Implement DB methods: UpsertGraphEdge, DeleteGraphEdgesByFile, GetOutgoingEdges, GetIncomingEdges, GraphStats
- [ ] Wire into existing store in `cmd/nano-brain/main.go`

## T4: Watcher integration

- [ ] Add `WithGraphRegistry(r *graph.Registry, store GraphStore) *Watcher` to `internal/watcher/watcher.go`
- [ ] Implement `extractAndUpsertEdges(ctx, tx, workspaceHash, filePath, content)` — delete-then-insert in transaction
- [ ] Call `extractAndUpsertEdges` after `extractAndUpsertSymbols` in `scanCollection()`
- [ ] Wire `WithGraphRegistry` in `cmd/nano-brain/main.go`
- [ ] Validate: existing watcher tests still pass

## T5: REST handler

- [ ] `internal/server/handlers/graph.go`: NewGraphHandler factory (follows existing handler pattern)
- [ ] Handler: parse request body → call GetOutgoingEdges or GetIncomingEdges → JSON response
- [ ] Register `POST /api/v1/graph/query` in `internal/server/routes.go`

## T6: MCP tool

- [ ] Add `memory_graph` tool in `internal/mcp/tools.go` (follows `registerMemorySymbols` pattern)
- [ ] Input: workspace, node, direction, edge_type (all as MCP tool params)
- [ ] Output: edge list with source, target, type, line

## T7: main.go wiring

- [ ] Instantiate `graph.NewRegistry()` with `GoGraphExtractor`
- [ ] Pass to `watcher.WithGraphRegistry()`
- [ ] Pass graph store to REST handler and MCP tool

## T8: Validate + PR

- [ ] `CGO_ENABLED=0 go build ./...` → exit 0
- [ ] `CGO_ENABLED=0 go vet ./...` → exit 0
- [ ] `CGO_ENABLED=0 go test -short ./...` → all pass
- [ ] Smoke: `./nano-brain status` → healthy
- [ ] Verify AC-2 and AC-3 manually against nano-brain's own source
- [ ] Open PR, request review
