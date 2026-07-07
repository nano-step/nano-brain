# smoke:e2e — Issue #571 (#542 F7: memory_query chunk_type filter)

Change-type: bug-fix. Verified on an isolated **:3199 / nanobrain_test** server
(dev DB / :3100 never touched).

## Behavioral proof of the vector-leg fix — service-level red-green

The fix is on the VECTOR leg of `HybridSearch`. `chunk_type_vector_571_test.go`
runs the real `HybridSearch` with a capturing querier and asserts all four vector
legs (all/ws × tags/no-tags) receive `ChunkType={symbol,valid}`:

```
RED  (fix stashed): VectorSearch / VectorSearchAll / VectorSearchWithTags /
                    VectorSearchAllWithTags all receive ChunkType={Valid:false}
GREEN (fix applied): all four receive {symbol,valid}  → --- PASS
```

## End-to-end over MCP HTTP

Seeded a workspace with two chunks (`chunk_type=symbol` "depositHandler" and
`chunk_type=raw` "deposit notes", `search_vector` populated) into nanobrain_test;
started a standalone server on :3199 (`NANO_BRAIN_ALLOW_DUPLICATE_SERVER=1`,
`NANO_BRAIN_DATABASE_URL=…/nanobrain_test`).

```
HTTP/1.1 200 OK   initialize            (Mcp-Session-Id issued)
HTTP/1.1 200 OK   tools/call memory_query  query="deposit balance"                    (no filter)
HTTP/1.1 200 OK   tools/call memory_query  query="deposit balance" chunk_type="symbol"
HTTP/1.1 200 OK   tools/call memory_query  query="deposit balance" chunk_type="raw"
```

Decoded result titles:

```
no filter      → ["depositHandler", "deposit notes"]
chunk=symbol   → ["depositHandler"]          (raw chunk excluded)
chunk=raw      → ["deposit notes"]           (symbol chunk excluded)
```

**Result:** `chunk_type` now filters `memory_query` end-to-end over MCP HTTP. The
vector-leg-specific fix (the F7 defect) is proven by the service-level red-green
test above; this HTTP run confirms the user-facing filtering behavior works over
the wire (these chunks carry no embeddings, so the BM25 leg carries the filter
here — both legs now honor `chunk_type`).

## Isolation / cleanup

Server on :3199 / nanobrain_test only; killed by captured PID (no broad kill).
Seeded workspace/docs/chunks deleted, temp binary removed.
