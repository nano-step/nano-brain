# smoke:e2e — Issue #553 (impact-in calls requalify, Phase 2 PR-A)

Real HTTP smoke of the fixed `POST /api/v1/graph/impact` endpoint against a live
server on **:3199 / nanobrain_test** (never dev DB / :3100). The endpoint's
`collectImpact` had the same bare-target bug as the MCP tool; this exercises the
REST fix end-to-end.

## Setup

```
# build + start test server (nanobrain_test, port 3199, embeddings off)
CGO_ENABLED=0 go build -o ./bin/nano-brain ./cmd/nano-brain/
NANO_BRAIN_DATABASE_URL=postgres://nanobrain:nanobrain@host.docker.internal:5432/nanobrain_test?sslmode=disable \
NANO_BRAIN_SERVER_PORT=3199 NANO_BRAIN_ALLOW_DUPLICATE_SERVER=1 NANO_BRAIN_EMBEDDING_PROVIDER="" \
  ./bin/nano-brain --unsafe-no-auth &

# seed one bare-target calls edge (as the extractors write it): caller.go::doThing --calls--> "B"
psql "$NANO_BRAIN_DATABASE_URL" -c \
  "INSERT INTO workspaces(hash,path,name,created_at,updated_at) VALUES('smoke553','/tmp/smoke553','smoke553',now(),now()) ON CONFLICT DO NOTHING;
   INSERT INTO graph_edges(workspace_hash,source_node,target_node,edge_type,source_file,metadata)
   VALUES('smoke553','caller.go::doThing','B','calls','caller.go','{}');"
```

## Request / response

Health:

```
HTTP/1.1 200 OK
{"status":"ok","ready":true,"version":"dev","uptime_s":23,"workspace_count":1673}
```

Impact (reverse / "in" is the endpoint's only mode — `GetImpactorsByTargets`):

```
curl -sS -i -X POST http://localhost:3199/api/v1/graph/impact \
  -H 'Content-Type: application/json' \
  -d '{"workspace":"smoke553","node":"b.go::B","edge_type":"calls","max_depth":1}'
```

```
HTTP/1.1 200 OK
Content-Type: application/json; charset=UTF-8

{"node":"b.go::B","impacted":[{"node":"caller.go::doThing","depth":1,"edge_type":"calls"}]}
```

**Result:** the caller `caller.go::doThing` is returned for the bare-stored calls
target `B` — before the fix `collectImpact` returned `impacted:[]` (the qualified
`b.go::B` frontier never matched the bare row). Confirms the REST surface now
resolves calls-edge callers, matching the MCP tool.

## Note on execution

`curl` is redirected by this environment's context-mode hook, so the request was
issued via the sandbox's `fetch` with the identical method/path/JSON body shown
above; the HTTP status and response body are verbatim. Server teardown killed
only the captured PID (no broad-kill); the `smoke553` seed rows were deleted
afterward.
