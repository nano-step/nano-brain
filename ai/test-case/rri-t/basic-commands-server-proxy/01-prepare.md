# RRI-T Phase 1: PREPARE — basic-commands-server-proxy

**Feature:** basic-commands-server-proxy
**Date:** 2026-05-14
**Branch:** fix/basic-commands-server-proxy
**GitHub Issue:** nano-step/nano-brain#8

---

## Feature Summary

Three CLI commands currently bypass the HTTP server and open SQLite directly,
violating the container topology where the CLI must always be a thin HTTP client.

| Command | Current | Target |
|---------|---------|--------|
| `tags` | `createStore()` direct SQLite | `GET /api/tags` → proxy |
| `update` | `createStore()` direct SQLite | `POST /api/update` → proxy |
| `status` | Mixed (partial proxy + local DB) | `GET /api/status` only |

New endpoints to add: `GET /api/tags`, `POST /api/update`
Reference pattern: `src/cli/commands/write.ts` (proxyPost), `src/cli/commands/search.ts` (proxyGet)

---

## Container Topology

```
HOST (macOS)
├── nano-brain-server :3100   ← owns SQLite + Qdrant
└── nano-brain-qdrant :6333

AGENT CONTAINER (OpenCode)
└── npx nano-brain <cmd>
      └── isInsideContainer() == true
            └── HTTP → host.docker.internal:3100
```

When NOT in container: existing local SQLite fallback behaviour is preserved.

---

## Relevant Source Files

| File | Role |
|------|------|
| `src/cli/commands/tags.ts` | To be converted |
| `src/cli/commands/update.ts` | To be converted |
| `src/cli/commands/status.ts` | To be completed |
| `src/cli/utils.ts` | `proxyGet`, `proxyPost`, `isServerRunning` helpers |
| `src/host.ts` | `isInsideContainer()`, `resolveHostUrl()` |
| `src/http/server.ts` | HTTP route registration |
| `src/http/routes.ts` | Route handlers |
| `test/rest-api.test.ts` | Test pattern reference |

---

## Dimensions In Scope

| Dimension | Relevance |
|-----------|-----------|
| D2: API | Primary — new endpoints must exist and return correct shape |
| D5: Data Integrity | Proxy must return identical data to direct SQLite |
| D6: Infrastructure | Container routing, server-down fallback |
| D7: Edge Cases | Malformed response, timeout, empty data |
| D4: Security | No cross-workspace tag leakage |
| D3: Performance | HTTP hop overhead acceptable |
| D1: UI/UX | N/A — CLI only |

---

## Out of Scope

- Code intelligence commands (`context`, `symbols`, `code-impact`, `focus`, `detect-changes`) — separate issue
- `isInsideContainer()` logic changes — already correct
- Qdrant / embedding pipeline — unchanged
