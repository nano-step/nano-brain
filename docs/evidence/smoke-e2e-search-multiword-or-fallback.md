# smoke:e2e — Issue #573 (#542 F9: memory_search multi-word OR-fallback)

Change-type: bug-fix. Verified over the real MCP-over-HTTP transport on an
isolated **:3199 / nanobrain_test** server (dev DB / :3100 never touched).

## Setup (isolated)

Seeded into nanobrain_test: a workspace (64-hex hash) + a document/chunk
(`chunk_type=symbol`, content "deposit account balance controller",
`search_vector` populated). Started a standalone server on :3199
(`NANO_BRAIN_ALLOW_DUPLICATE_SERVER=1`, `NANO_BRAIN_DATABASE_URL=…/nanobrain_test`).

## MCP streamable-HTTP handshake + tool calls

```
HTTP/1.1 200 OK   initialize            (Mcp-Session-Id issued)
HTTP/1.1 200 OK   tools/call memory_search  query="deposit"                       chunk_type="symbol"
HTTP/1.1 200 OK   tools/call memory_search  query="deposit zzzmissing balance"    chunk_type="symbol"
```

Decoded result titles:

```
single-word "deposit"                       → ["depositController"]
multi-word  "deposit zzzmissing balance"    → ["depositController"]  total=1
```

The multi-word query contains `zzzmissing`, which the chunk does NOT contain, so
the `websearch_to_tsquery` AND (`deposit & zzzmissing & balance`) matches nothing.
The OR-fallback (`deposit | zzzmissing | balance`) rescues it → the symbol chunk
is returned. **Before this fix** the multi-word + `chunk_type=symbol` search
returned `results:[] total:0` (confirmed by the red-green integration test).

## Isolation / cleanup

Server on :3199 / nanobrain_test only; killed by captured PID (no broad kill).
Seeded workspace/doc/chunk deleted, temp binary removed.
