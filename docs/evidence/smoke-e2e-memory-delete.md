# smoke:e2e — Issue #544 N3 (memory_delete)

Change-type: user-feature. Verified over the real MCP-over-HTTP transport on an
isolated **:3199 / nanobrain_test** server (dev DB / :3100 never touched).

## Setup (isolated)

Seeded into nanobrain_test: a workspace (64-hex hash) and a document
(`collection=memory`, a synthetic "throwaway smoke note").

## MCP streamable-HTTP handshake + tool calls

```
HTTP/1.1 200 OK   initialize                    (Mcp-Session-Id issued)
202                notifications/initialized
tools/call memory_get     (BEFORE delete)  → full content returned
tools/call memory_delete                    → {"deleted":true,"id":"<uuid>","title":"throwaway smoke note"}
tools/call memory_get     (AFTER delete)   → isError:true, "document not found: no document or chunk found for id <uuid> in workspace <hash>"
```

**Result:** the full write→recall→delete→confirm-gone lifecycle works over
the real transport. `memory_delete` returns a clean confirmation; a
subsequent `memory_get` on the same id errors cleanly (no leaked SQL, no
stale content) — proving the document (and its chunks, via `ON DELETE
CASCADE`, exercised separately in the integration test) is actually gone,
not just hidden.

## Isolation / cleanup

Server on :3199 / nanobrain_test only; killed by captured PID (no broad kill).
Seeded workspace/document deleted (document already gone via memory_delete
itself), temp binary removed.
