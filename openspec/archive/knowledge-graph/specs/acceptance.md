# Knowledge Graph — Acceptance Criteria

## AC-1: Migration + Schema

- `go build ./...` passes with `CGO_ENABLED=0` after migration
- `goose up` creates `graph_edges` table with all indexes and unique constraint
- `goose down` drops the table cleanly

## AC-2: Go Extractor — Contains

Given `internal/watcher/watcher.go` processed by watcher:

```
memory_graph(node="internal/watcher/watcher.go", edge_type="contains", direction="out")
→ edges include "internal/watcher/watcher.go::processFile" and others
→ edge count ≥ 10
```

## AC-3: Go Extractor — Imports

Given `internal/watcher/watcher.go` processed by watcher:

```
memory_graph(node="internal/watcher/watcher.go", edge_type="imports", direction="out")
→ edges include target containing "internal/symbol"
→ edge count ≥ 5
```

Reverse lookup:
```
memory_graph(node="internal/symbol", edge_type="imports", direction="in")
→ edges include source "internal/watcher/watcher.go"
```

## AC-4: Go Extractor — Calls (best-effort)

Given `internal/watcher/watcher.go` processed:

```
memory_graph(node="internal/watcher/watcher.go::processFile", edge_type="calls", direction="out")
→ returns results (may be empty if processFile has no extractable calls, but no error)
→ no panic, no crash
```

## AC-5: Incremental Update

1. Add a dummy import line to a test fixture file
2. Trigger watcher rescan
3. Query imports for that file → new import appears
4. Remove the dummy import line
5. Trigger watcher rescan
6. Query imports → stale import is gone

## AC-6: Unit Tests Pass

```
CGO_ENABLED=0 go test -short ./internal/graph/... → PASS
```

Tests cover:
- `GoGraphExtractor.ExtractEdges`: contains, imports, calls on fixture Go source
- Known-edge-count assertions using test fixtures in `internal/graph/testdata/`
- Edge case: file with no imports → empty slice, no error
- Edge case: file with circular-looking names → no infinite loop

## AC-7: No Regression

```
CGO_ENABLED=0 go test -short ./... → PASS (all existing tests still pass)
```

## AC-8: Build Clean

```
CGO_ENABLED=0 go build ./... → exit 0
CGO_ENABLED=0 go vet ./... → exit 0
```
