# smoke:e2e — Issue #545 (memory_vsearch over-fetch, Phase 3 PR-C)

Live server on **:3199 / nanobrain_test** with the real **ollama** embedder
(`nomic-embed-text`), against a workspace with ~399 real embeddings. Confirms the
vector-search HTTP surface is healthy end-to-end (query embedded live, vector
search, ranked JSON response).

## Request / response

```
curl -sS -i -X POST http://localhost:3199/api/v1/vsearch \
  -H 'Content-Type: application/json' \
  -d '{"workspace":"<ws-hash>","query":"database connection pool setup","max_results":5}'
```

```
HTTP/1.1 200 OK
Content-Type: application/json; charset=UTF-8

{"results":[{"score":0.5098,"source_path":".../internal/mcp/vsearch_overfetch_545_integration_test.go",...},
            {"score":0.5014,"source_path":".../internal/mcp/tools.go?symbol=ub&kind=var",...}, ... 5 total]}
```

Query embedded via ollama (`http://host.docker.internal:11434`, nomic-embed-text),
vector search over real embeddings, 5 ranked results returned — the vsearch HTTP
path works end-to-end with a real embedder.

## Why the fix itself is covered by integration tests, not this HTTP smoke

The #545 fix (over-fetch chunks before `group_by=document` dedup, + the R88
cap-floor) lives in the **MCP tool** `registerMemoryVSearch` (`tools.go`), on the
`group_by=document` path. The REST `/api/v1/vsearch` handler
(`internal/server/handlers/search.go`) is **chunk-level — it does no
document-dedup** — so it does not exercise the fixed path (and correctly needs no
fix). Reproducing the fix's behavior (top chunks clustering into few documents)
requires *controlled* embeddings in a known vector space — impossible to arrange
deterministically with a live embedder over HTTP. That is done deterministically
by 6 integration tests driving the real MCP tool handler against `nanobrain_test`:

- `TestMemoryVSearch_OverFetch_FewDocumentsCollapse` — #545 repro (2 hot docs ×5
  chunks vs 10 cold docs ×1); asserts 7 distinct docs, not 2.
- `TestMemoryVSearch_OverFetchCapFloor_DeepPagination` — R88 cap-floor at deep offset.
- `TestMemoryVSearch_ManyDocumentsAlreadyDiverse`, `TestMemoryVSearch_GroupByNone_FetchLimitUnaffected` — regression/scope controls.
- (+ existing pagination/time-filter tests.) All PASS.

## Note on execution / privacy

`curl` is redirected by this environment's context-mode hook; the request was
issued via the sandbox `fetch` with identical method/path/body — status and body
verbatim. The workspace hash is redacted (`<ws-hash>`) per the privacy rule.
Server teardown killed only the captured PID (no broad-kill); :3199 confirmed clean.
