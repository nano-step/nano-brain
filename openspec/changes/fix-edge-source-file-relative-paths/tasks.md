## 1. Watcher Edge Path Normalization

- [x] 1.1 Add `filepath.Rel(col.dirPath, filePath)` in `extractAndUpsertEdges` to normalize absolute paths to workspace-relative (matching `extractAndUpsertCFGs` at line 835)
- [x] 1.2 Update `DeleteGraphEdgesByFile` call to use the normalized relative path
- [x] 1.3 Verify edge extractors receive relative paths (no other changes needed in extractors)

## 2. deriveServiceName Defense-in-Depth

- [x] 2.1 Strip leading `/` from `source_file` before splitting on `/`
- [x] 2.2 Handle Windows-style paths (drive letter prefix) if applicable
- [x] 2.3 Add unit tests for `deriveServiceName` with absolute paths, relative paths, and empty paths

## 3. Verification

- [x] 3.1 Run existing tests: `go test -race -short ./...`
- [x] 3.2 Trigger `POST /api/v1/reindex-cfg` with `wipe: true` to re-index edges with relative paths
- [x] 3.3 Test sequence diagram: `POST /api/v1/graph/flow` with `format=sequence` — verify service name appears instead of "Backend"
- [x] 3.4 Run full integration tests: `go test -race -tags=integration ./...`
