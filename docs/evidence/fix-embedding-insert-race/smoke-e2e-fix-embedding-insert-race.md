# Smoke E2E — fix-embedding-insert-race

## Environment

- Built the server from this change and ran it with `config.test.yml` on port `3199`.
- Used only the isolated `nanobrain_test` database with duplicate-server protection enabled.
- Used the configured local embedding provider; the production dev server and database were not contacted.

## API smoke

```text
GET  /health       -> 200
POST /api/v1/init  -> 200
POST /api/v1/write -> 201 {"collection":"memory","chunk_count":1}
POST /api/v1/embed -> 200 {"embedded":1,"remaining":0}
```

The workspace identifier and generated document identifier were deliberately redacted from this evidence. The ordinary endpoint path wrote one pending chunk and persisted its embedding successfully. The deterministic stale-result behavior is covered by the isolated PostgreSQL and handler regressions.

## UI route check

The service binary does not register a static web UI route: both `/` and `/ui/` returned `404`. `scripts/smoke-ui.sh` is not present, so a browser/UI smoke is not applicable to this handler-only server change.
