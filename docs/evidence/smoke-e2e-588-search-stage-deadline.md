# smoke:e2e — Issue #588 (agent-supplied query expansion, provider-free HyDE)

Change-type: user-feature + bug-fix. Verified over the real HTTP `/api/v1/query`
endpoint on an isolated **:3199 / nanobrain_test** server. Dev DB / :3100 never
touched.

## Setup (isolated)

- Binary: `CGO_ENABLED=0 go build -o nb588 ./cmd/nano-brain` from this branch.
- Config: scratch `config.yml` pointing at:
  - `database.url`: `nanobrain_test` on `localhost:5432`
  - `server.port`: `3199`
  - `embedding`: ollama `nomic-embed-text` (localhost)
  - `search.hyde.enabled: true`, `search.hyde.provider_url: http://203.0.113.1:65535`
    (RFC 5737 TEST-NET-3 — reserved, non-routable, guaranteed unreachable),
    `search.hyde.max_latency_ms: 3000`
- Server started with `NANO_BRAIN_ALLOW_DUPLICATE_SERVER=1 NANO_BRAIN_CONFIG=./config.yml ./nb588 serve`,
  PID captured to `server.pid` immediately after backgrounding.

## Seed

```
curl -s -X POST http://localhost:3199/api/v1/init -H 'Content-Type: application/json' \
  -d '{"root_path":"<scratch>/ws588","name":"smoke588-hypothetical"}'
{"workspace_hash":"9fb60dc3ffcd97a8a796f12ee21de1984affdede6a8f9c11d6110b7cc7363577", ...}

curl -s -X POST http://localhost:3199/api/v1/write -H 'Content-Type: application/json' \
  -d '{"workspace":"9fb60dc3...","content":"The quokka waddled across the sun-baked marimba plaza carrying a teacup of nebula jam.","title":"quokka-marimba-doc","collection":"memory"}'
{"id":"36be946a-5684-43d0-b46e-d01889c8f5d8", ..., "chunk_count":1}
```

## TC-A — WITH `hypothetical` (must bypass HyDE, must be fast)

```
$ time curl -i -s -X POST "http://localhost:3199/api/v1/query" -H 'Content-Type: application/json' \
    -d '{"workspace":"9fb60dc3ffcd97a8a796f12ee21de1984affdede6a8f9c11d6110b7cc7363577","query":"quokka marimba","hypothetical":"A quokka carrying a teacup of jam through a plaza."}'

HTTP/1.1 200 OK
Content-Type: application/json
X-Nano-Brain-Version: dev
X-Request-Id: 32c173a5
Date: Sat, 11 Jul 2026 00:00:24 GMT
Content-Length: 510

{"results":[{"id":"66cf83a1-a87a-47b3-8929-cde5658cf1a0","title":"quokka-marimba-doc","snippet":"The quokka waddled across the sun-baked marimba plaza carrying a teacup of nebula jam.","score":0.9999991616764263,"collection":"memory","workspace_hash":"9fb60dc3ffcd97a8a796f12ee21de1984affdede6a8f9c11d6110b7cc7363577","source_path":"","document_id":"36be946a-5684-43d0-b46e-d01889c8f5d8","created_at":"2026-07-11T06:59:21.958533+07:00","updated_at":"2026-07-11T06:59:21.958533+07:00"}],"total":1,"query_ms":7}

real  0.027s
```

HTTP 200, **non-empty** (`total:1`), and **fast** (`query_ms: 7`). No HyDE-related log
line appears in the server log for this request.

## TC-B — WITHOUT `hypothetical` (control: HyDE enabled, provider unreachable)

```
$ time curl -i -s -X POST "http://localhost:3199/api/v1/query" -H 'Content-Type: application/json' \
    -d '{"workspace":"9fb60dc3ffcd97a8a796f12ee21de1984affdede6a8f9c11d6110b7cc7363577","query":"quokka marimba"}'

HTTP/1.1 200 OK
Content-Type: application/json
X-Nano-Brain-Version: dev
X-Request-Id: 22342d58
Date: Sat, 11 Jul 2026 00:00:39 GMT
Content-Length: 512

{"results":[{"id":"66cf83a1-a87a-47b3-8929-cde5658cf1a0","title":"quokka-marimba-doc", ... }],"total":1,"query_ms":3184}

real  3.206s
```

Server log for this exact request (only log line matching "hyde" in the whole run):

```
{"level":"warn","error":"hyde: request failed: Post \"http://203.0.113.1:65535/chat/completions\": context deadline exceeded (Client.Timeout exceeded while awaiting headers)","time":"2026-07-11T07:00:39+07:00","message":"hyde: LLM call failed, returning empty"}
```

## Conclusion (proves the design intent)

| | TC-A (`hypothetical` supplied) | TC-B (no `hypothetical`) |
|---|---|---|
| HTTP status | 200 | 200 |
| `total` results | 1 (non-empty) | 1 |
| `query_ms` | **7** | **3184** |
| HyDE provider contacted | **no** (zero log lines) | **yes** (1 warn log line, `context deadline exceeded` at `max_latency_ms: 3000`) |

TC-A is ~450x faster than TC-B because `hypothetical` skips `s.hydeGenerator.Generate`
entirely (`internal/search/service.go` vector leg) — proving the agent-supplied
expansion overrides server-side HyDE and never contacts the configured (here,
unreachable) HyDE provider, exactly as designed (D1/D2/D3).

## Isolation / cleanup

- Server on :3199 / `nanobrain_test` only (`NANO_BRAIN_ALLOW_DUPLICATE_SERVER=1`); dev
  DB / :3100 never touched.
- Seeded workspace + document deleted via `DELETE /api/v1/documents/:id` and
  `DELETE /api/v1/workspaces/:hash` before shutdown.
- Killed by captured PID only (`kill $(cat server.pid)`, graceful shutdown observed
  in logs — no broad `pkill`/`lsof`-based kill).
- Scratch binary, config, and workspace dir lived under the session scratchpad —
  no leftover files in the repo.
