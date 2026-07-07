# smoke:e2e — Issue #558 (memory_query mode:debugging source labels, Phase 3 PR-D)

Real **MCP-over-HTTP** smoke of the exact changed surface (debugging mode is an
MCP-tool feature — there is no REST endpoint that runs `DebugSearch`). Live server
on **:3199 / nanobrain_test** with the real **ollama** embedder, workspace with
~412 embeddings.

## Handshake + tool call (MCP streamable HTTP, `/mcp`)

```
# 1. initialize
curl -sS -i -X POST http://localhost:3199/mcp \
  -H 'Content-Type: application/json' -H 'Accept: application/json, text/event-stream' \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"smoke","version":"1"}}}'
# → HTTP/1.1 200 OK, Mcp-Session-Id: <sid>

# 2. tools/call memory_query with mode:debugging (Mcp-Session-Id header)
curl -sS -X POST http://localhost:3199/mcp \
  -H 'Content-Type: application/json' -H 'Accept: application/json, text/event-stream' -H 'Mcp-Session-Id: <sid>' \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":"memory_query","arguments":{"workspace":"<ws-hash>","query":"deposit fails error handling","mode":"debugging","max_results":5}}}'
```

```
HTTP/1.1 200 OK
{"jsonrpc":"2.0","id":2,"result":{"content":[{"type":"text","text":"{\"results\":[ ... 5 items ... ]}"}]}}
```

Decoded `results[].source` labels: **`["code","code","code","code","session"]`**.

**Result:** `mode:"debugging"` returns results each carrying a `source` leg label
(`code`/`session`/`config`) end-to-end over the real MCP HTTP endpoint with a live
embedder. **Before PR-D** `DebugSearch` RRF-merged the legs into a flat list with
no `source` field — the advertised source labeling was absent.

## Note on execution / privacy

`curl` is redirected by this environment's context-mode hook; the requests were
issued via the sandbox `fetch` performing the identical MCP JSON-RPC handshake
(initialize → session id → tools/call). Status/body verbatim; workspace hash
redacted (`<ws-hash>`). Non-debug queries omit `source` (`omitempty`), covered by
the integration test. Server teardown killed only the captured PID; :3199 clean.
