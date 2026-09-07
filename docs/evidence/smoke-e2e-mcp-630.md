# smoke:e2e — #630 MCP SDK v0.8.0 → v1.7.0

Change type: `bug-fix` · Branch: `fix/mcp-sdk-v1-upgrade` · Date: 2026-08-26

Server: binary built from source with `CGO_ENABLED=0`, run on **port 3199** against **`nanobrain_test`**. Never `nanobrain_dev`, never `:3100`. Stopped afterwards by exact PID; `nanobrain-pg` untouched.

```bash
CGO_ENABLED=0 go build -o "$SCRATCH/nb-smoke" ./cmd/nano-brain
NANO_BRAIN_ALLOW_DUPLICATE_SERVER=1 NANO_BRAIN_SERVER_PORT=3199 \
  DATABASE_URL="postgres://nanobrain:nanobrain@localhost:5432/nanobrain_test" \
  "$SCRATCH/nb-smoke" serve &
```

Database binding confirmed from the server's own log before probing:

```
{"level":"info","url":"postgres://***:***@localhost:5432/nanobrain_test","message":"database pool connected"}
```

## 1. REST surface reachable

```bash
curl -s http://localhost:3199/api/status
```

```json
{"pg_status":"healthy","migration_version":29,"active_provider":"ollama","queue_status":"busy",...}
```

## 2. `server/discover` — the 2026-07-28 entry point v0.8.0 could not serve

```bash
curl -s -X POST http://localhost:3199/mcp \
  -H 'Content-Type: application/json' \
  -H 'Accept: application/json, text/event-stream' \
  -H 'Mcp-Protocol-Version: 2026-07-28' \
  -H 'Mcp-Method: server/discover' \
  -d '{"jsonrpc":"2.0","id":1,"method":"server/discover","params":{"_meta":{
       "io.modelcontextprotocol/protocolVersion":"2026-07-28",
       "io.modelcontextprotocol/clientCapabilities":{}}}}'
```

```json
{"jsonrpc":"2.0","id":1,"result":{
  "resultType":"complete",
  "supportedVersions":["2026-07-28","2025-11-25","2025-06-18","2025-03-26","2024-11-05"],
  "capabilities":{"logging":{},"tools":{"listChanged":true}},
  "ttlMs":0,"cacheScope":"public",
  "_meta":{"io.modelcontextprotocol/serverInfo":{"name":"nano-brain","version":"dev"}}}}
```

All five protocol revisions advertised. This is the goal of the upgrade, met.

## 3. `tools/list` over the 2026-07-28 path

All **19** tools returned: `memory_query`, `memory_search`, `memory_vsearch`, `memory_get`, `memory_delete`, `memory_write`, `memory_update`, `memory_tags`, `memory_status`, `memory_wake_up`, `memory_symbols`, `memory_graph`, `memory_impact`, `memory_trace`, `memory_flow`, `memory_flowchart`, `memory_ticket`, `memory_workspaces_list`, `memory_workspaces_resolve`.

## 4. Host-header matrix — the regression this PR fixes

`/mcp` and `/sse`, curl with an explicit `Host:` header:

| Host | Before fix | After fix |
|---|---|---|
| `localhost:3199` | 200 | 200 |
| `127.0.0.1:3199` | 200 | 200 |
| `host.docker.internal:3199` | **403** | **200** |
| `evil.example.com` | 403 | 403 |
| `192.168.1.50:3199` | 403 | 403 |

`GET /sse`: `host.docker.internal` → 200, `evil.example.com` → 403.

Raw-socket probes, verifying the guard cannot be bypassed by a protocol version that makes `Host` optional:

```bash
printf 'POST /mcp HTTP/1.1\r\nContent-Length: 0\r\nConnection: close\r\n\r\n' | nc localhost 3199
# HTTP/1.1 400 Bad Request: missing required Host header   (net/http, before the handler)

printf 'POST /mcp HTTP/1.0\r\nContent-Length: 0\r\n\r\n' | nc localhost 3199
# HTTP/1.0 403 Forbidden                                   (HostGuard; passed through before 008b317)

printf 'POST /mcp HTTP/1.0\r\nHost: host.docker.internal\r\nContent-Length: 0\r\n\r\n' | nc localhost 3199
# HTTP/1.0 415 Unsupported Media Type                      (passes the guard, hits the Content-Type check)
```

## 5. Cross-compile, all release targets

```
OK    linux/amd64
OK    linux/arm64
OK    darwin/amd64
OK    darwin/arm64
```

Confirms the new `segmentio/asm` dependency does not break the `CGO_ENABLED=0` static-binary release matrix.

## Independent reproduction

The reviewer reproduced the original regression from a **real Docker container** against a host-native daemon (`container POST /mcp` → 403 on unpatched v1.7.0, 200 on v0.8.0) and confirmed the fix there (→ 200), plus probed the Host normalization path with trailing-dot FQDNs, userinfo forms, absolute-form URIs, IDN homographs, IPv6 zone-ids and duplicate `Host` headers — all fail closed.

## Note on the legacy handshake

Calling the deprecated `initialize` with `protocolVersion: 2026-07-28` negotiates **down** to `2025-11-25`. Expected, not a defect: `negotiatedVersion` (`go-sdk/mcp/shared.go:68-79`) caps there deliberately because `initialize` is removed in 2026-07-28. Modern clients use `server/discover`.
