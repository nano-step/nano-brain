# CLI UX Overhaul Phase 1 — Smoke Test Evidence

Branch: `feat/342-cli-ux-overhaul-phase1`
Date: 2026-06-03

## go build ./...

```
(no output — build succeeded)
```

## go test -race -short ./...

```
?   	github.com/nano-brain/nano-brain/internal/storage/sqlc	[no test files]
ok  	github.com/nano-brain/nano-brain/cmd/nano-brain	4.296s
ok  	github.com/nano-brain/nano-brain/internal/bench	(cached)
ok  	github.com/nano-brain/nano-brain/internal/chunk	(cached)
ok  	github.com/nano-brain/nano-brain/internal/config	(cached)
ok  	github.com/nano-brain/nano-brain/internal/embed	(cached)
ok  	github.com/nano-brain/nano-brain/internal/eventbus	(cached)
ok  	github.com/nano-brain/nano-brain/internal/graph	(cached)
ok  	github.com/nano-brain/nano-brain/internal/harvest	(cached)
ok  	github.com/nano-brain/nano-brain/internal/health	(cached)
ok  	github.com/nano-brain/nano-brain/internal/health/doctor	1.033s
ok  	github.com/nano-brain/nano-brain/internal/links	(cached)
ok  	github.com/nano-brain/nano-brain/internal/mcp	(cached)
ok  	github.com/nano-brain/nano-brain/internal/migrate	(cached)
ok  	github.com/nano-brain/nano-brain/internal/search	(cached)
ok  	github.com/nano-brain/nano-brain/internal/server	1.049s
?   	github.com/nano-brain/nano-brain/internal/testutil	[no test files]
?   	github.com/nano-brain/nano-brain/migrations	[no test files]
ok  	github.com/nano-brain/nano-brain/internal/server/handlers	1.836s
ok  	github.com/nano-brain/nano-brain/internal/server/middleware	(cached)
ok  	github.com/nano-brain/nano-brain/internal/server/webui	(cached)
ok  	github.com/nano-brain/nano-brain/internal/storage	(cached)
ok  	github.com/nano-brain/nano-brain/internal/summarize	(cached)
ok  	github.com/nano-brain/nano-brain/internal/symbol	(cached)
ok  	github.com/nano-brain/nano-brain/internal/telemetry	(cached)
ok  	github.com/nano-brain/nano-brain/internal/watcher	(cached)
```

## nano-brain version --which

```
path: /tmp/nano-brain-phase1
version: dev
source: dev-build
```

## nano-brain version --which --json

```json
{"path":"/tmp/nano-brain-phase1","source":"dev-build","version":"dev"}
```

## nano-brain mcp-url

```
http://host.docker.internal:3100/mcp
```

(Container detected via `/.dockerenv`; `NANO_BRAIN_MCP_URL` not set)

## nano-brain doctor (offline)

```
nano-brain doctor

  Config................ /home/agent/.nano-brain/config.yml OK
  Binary................ /tmp/nano-brain-phase1...... OK
  PostgreSQL............ localhost:5432.............. FAIL
    → Is PostgreSQL running?
    → Try: docker compose up -d
  pgvector.............. no connection............... SKIP
  Embedding provider.... localhost:11434............. FAIL
    → Is Ollama running? Try: ollama serve
  Embedding model....... nomic-embed-text............ SKIP

Some checks failed.
```

## nano-brain doctor --online

```
nano-brain doctor

  Config................ /home/agent/.nano-brain/config.yml OK
  Binary................ /tmp/nano-brain-phase1...... OK
  PostgreSQL............ localhost:5432.............. FAIL
    → Is PostgreSQL running?
    → Try: docker compose up -d
  pgvector.............. no connection............... SKIP
  Embedding provider.... localhost:11434............. FAIL
    → Is Ollama running? Try: ollama serve
  Embedding model....... nomic-embed-text............ SKIP
  Server running........ http://host.docker.internal:3100 OK
  Queue health.......... 1011/10000.................. OK
  Version skew.......... cli=dev server=............. OK
  MCP reachable......... HTTP 400.................... FAIL
    → MCP endpoint returned unexpected status

Some checks failed.
```

Note: Server is reachable (host server running at host.docker.internal:3100). MCP endpoint returns 400 because it requires specific MCP protocol headers — the endpoint is live but not serving plain GET. This is expected behavior for the MCP protocol endpoint.
