# smoke:e2e — Issue #501 (JS/TS/Vue import resolution, Phase 2 PR-B)

Real HTTP smoke against a live server on **:3199 / nanobrain_test** (never dev DB
/ :3100). PR-B makes JS/TS/Vue `imports` edges store the **resolved** workspace-
relative path instead of the raw specifier; this confirms reverse-impact on the
resolved path returns the importer over HTTP.

## Setup

```
CGO_ENABLED=0 go build -o ./bin/nano-brain ./cmd/nano-brain/
NANO_BRAIN_DATABASE_URL=...nanobrain_test NANO_BRAIN_SERVER_PORT=3199 \
NANO_BRAIN_ALLOW_DUPLICATE_SERVER=1 NANO_BRAIN_EMBEDDING_PROVIDER="" ./bin/nano-brain --unsafe-no-auth &

# seed an imports edge in the POST-FIX shape the resolver now produces:
# consumer.ts imports "~/utils/enums" → target resolved to repo-a/utils/enums.ts
# (raw specifier retained in metadata).
psql "$DB" -c "INSERT INTO graph_edges(workspace_hash,source_node,target_node,edge_type,source_file,metadata)
  VALUES('smokeB501','repo-a/consumer.ts','repo-a/utils/enums.ts','imports','repo-a/consumer.ts','{\"raw_specifier\":\"~/utils/enums\"}');"
```

## Request / response

```
curl -sS -i -X POST http://localhost:3199/api/v1/graph/impact \
  -H 'Content-Type: application/json' \
  -d '{"workspace":"smokeB501","node":"repo-a/utils/enums.ts","edge_type":"imports","max_depth":1}'
```

```
HTTP/1.1 200 OK
Content-Type: application/json; charset=UTF-8

{"node":"repo-a/utils/enums.ts","impacted":[{"node":"repo-a/consumer.ts","depth":1,"edge_type":"imports"}]}
```

**Result:** reverse-impact on the resolved import target `repo-a/utils/enums.ts`
returns its importer `repo-a/consumer.ts`. **Before PR-B** the edge target was the
raw specifier `~/utils/enums`, so `impact(node="repo-a/utils/enums.ts")` returned
`impacted:[]` — the exact #501 false-negative. Now fixed.

The resolver-produces-the-resolved-edge path (extraction E2E: index the fixture →
edges carry resolved targets) is covered by the passing integration test
`internal/mcp/import_resolution_501_integration_test.go` (indexes
`internal/graph/testdata/import-fixture/`).

## Note on execution

`curl` is redirected by this environment's context-mode hook, so the request was
issued via the sandbox `fetch` with the identical method/path/body; status and
body are verbatim. Server teardown killed only the captured PID (no broad-kill);
the `smokeB501` seed rows were deleted afterward; :3199 confirmed clean.
