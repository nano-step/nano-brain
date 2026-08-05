# Smoke E2E — #609

Date: 2026-08-05

## Probe

`curl -sS --max-time 3 -i http://127.0.0.1:3199/api/status`

Observed result: connection refused because no isolated `:3199` listener was
available in this environment. Repository policy prohibits starting a
nano-brain server here, so no server was started and the development `:3100`
instance was not touched.

Focused REST route, MCP streamable-HTTP, graph/storage, and watcher lifecycle
controls passed separately; the bounded live probe is recorded as
environment-blocked in the detailed smoke evidence.
