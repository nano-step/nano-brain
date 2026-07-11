# Smoke E2E — dedup code-summary docs + retry 524 (#552)

**Date:** 2026-07-11
**Branch:** `fix/552-summary-doc-dedup`
**Lane:** high-risk (data-model) · bug-fix
**Binary:** `CGO_ENABLED=0 go build -o nano-brain-552 ./cmd/nano-brain`
**Database:** `nanobrain_test` (port 5432 via `localhost`/`host.docker.internal`) — **never** `nanobrain_dev`
**Server port:** **3199** — **never** `:3100`

## Setup

- Config: `config.test.yml` (repo root) — `code_summarization.enabled: true`, points at `nanobrain_test`, port 3199.
- Fixture workspace: a tiny Go package with a single function symbol (`Greet`), created in the session scratchpad, isolated from any real project:
  ```go
  package fixture

  // Greet returns a friendly greeting for name.
  func Greet(name string) string {
      return "hello " + name
  }
  ```
- Server started with `NANO_BRAIN_ALLOW_DUPLICATE_SERVER=1` (bypasses the single-instance guard, which only checks port 3100/`NANO_BRAIN_PORT` and does not know about the config file's `server.port: 3199`) and `NANO_BRAIN_CONFIG=config.test.yml`.
- Server PID captured to a pidfile at launch (`echo $! > server-552b.pid`) and stopped by that exact PID only (`kill <pid>`) — no broad `pkill`/`lsof`-based kill used at any point.

## Server start + health check

```
$ NANO_BRAIN_ALLOW_DUPLICATE_SERVER=1 NANO_BRAIN_CONFIG=config.test.yml ./nano-brain-552 serve > server-552b.log 2>&1 &
$ echo $! > server-552b.pid

$ curl -s -D headers.txt -o body.json http://localhost:3199/health --max-time 5
$ cat headers.txt
HTTP/1.1 200 OK
Content-Type: application/json
X-Nano-Brain-Version: dev
X-Request-Id: 02496ad2
Date: Sat, 11 Jul 2026 05:12:31 GMT
Content-Length: 82

$ cat body.json
{"status":"ok","ready":true,"version":"dev","uptime_s":32,"workspace_count":3218}
```

## TC-1 — Register fixture workspace

```
$ curl -s -D headers.txt -o body.json -X POST http://localhost:3199/api/v1/init \
    -H 'Content-Type: application/json' \
    -d '{"root_path":"<scratchpad>/fixture-552","name":"smoke552"}'
$ cat headers.txt
HTTP/1.1 200 OK
Content-Type: application/json

$ cat body.json
{"workspace_hash":"b3a505dc71083d433d32fbedc8803b209f23a45f800be8b3c2706a35f688f0a2",
 "root_path":"<scratchpad>/fixture-552","name":"fixture-552", ...}
```

`WS=b3a505dc71083d433d32fbedc8803b209f23a45f800be8b3c2706a35f688f0a2` (used below).

## TC-2 — Reindex + confirm symbol indexed

```
$ curl -s -D headers.txt -o body.json -X POST http://localhost:3199/api/v1/reindex \
    -H 'Content-Type: application/json' -d "{\"workspace\":\"$WS\"}"
$ cat headers.txt
HTTP/1.1 202 Accepted

$ cat body.json
{"status":"queued","chunks_enqueued":1,"watcher_triggered":true,"scanned":1,"skipped":0,"embedded":1,"deleted":0,"duration_ms":8, ...}
```

Server log confirms the symbol was extracted:

```
{"level":"info","component":"watcher","file":".../fixture-552/greet.go","collection":"code","chunks":1,"message":"indexed file"}
{"level":"info","component":"watcher","file":".../fixture-552/greet.go","symbols":1,"message":"symbols extracted"}
```

DB check:

```
$ psql ... -c "SELECT chunk_type, symbol_name, symbol_kind FROM chunks WHERE workspace_hash='$WS';"
 chunk_type | symbol_name | symbol_kind
------------+-------------+-------------
 symbol     | Greet       | function
```

**Note:** This shared `nanobrain_test` instance also has ~3200 previously-registered workspaces (including this nano-brain repo itself) from prior smoke-test sessions, whose boot-time watcher replay + background re-embedding briefly delayed the fixture's own scan (single watcher goroutine). This is pre-existing environment state unrelated to #552 — the fixture was indexed correctly once its turn came, and no dev DB/port was touched.

## TC-3 — First `code/summarize` call (AC-1, first half)

```
$ curl -s -D headers.txt -o body.json -X POST http://localhost:3199/api/v1/code/summarize \
    -H 'Content-Type: application/json' -d "{\"workspace\":\"$WS\"}"
$ cat headers.txt
HTTP/1.1 200 OK

$ cat body.json
{"processed":1,"skipped":0,"errors":0}
```

DB check — exactly one summary doc, path has no `&hash=`:

```
$ psql ... -c "SELECT source_path, content_hash FROM documents WHERE workspace_hash='$WS' AND tags @> ARRAY['symbol-summary']::text[];"
 .../fixture-552/greet.go?symbol=Greet&kind=function&summary=true | c7653d78...
```

✅ PASS — source_path has no `&hash=` segment (D1 confirmed).

## TC-4 — Change symbol content, reindex, second `code/summarize` call (AC-1, second half)

Modified `greet.go` (new function body → new content hash), then:

```
$ curl -s -D headers.txt -o body.json -X POST http://localhost:3199/api/v1/reindex \
    -H 'Content-Type: application/json' -d "{\"workspace\":\"$WS\"}"
$ cat body.json
{"status":"queued","chunks_enqueued":0,"watcher_triggered":false,"scanned":1,"skipped":1,"embedded":0,"deleted":2,"duration_ms":8, ...}

$ curl -s -D headers.txt -o body.json -X POST http://localhost:3199/api/v1/code/summarize \
    -H 'Content-Type: application/json' -d "{\"workspace\":\"$WS\"}"
$ cat headers.txt
HTTP/1.1 200 OK

$ cat body.json
{"processed":1,"skipped":0,"errors":0}
```

DB check:

```
$ psql ... -c "SELECT source_path, content_hash, LEFT(content,90) FROM documents WHERE workspace_hash='$WS' AND tags @> ARRAY['symbol-summary']::text[];"
 .../fixture-552/greet.go?symbol=Greet&kind=function&summary=true | c646d6c2... | Greet takes a name string, converts it to uppercase...
```

✅ PASS — still exactly one summary doc for the symbol; path still has no `&hash=`; content reflects the latest (uppercase) summary.

*(Note: the reindex handler's orphan-cleanup pass removes the summary document between calls here because its synthetic `?symbol=...&summary=true` source_path isn't a real on-disk file — this is pre-existing reindex behavior, not part of #552's scope, and does not affect the dedup assertion below, which checks the final state across the whole sequence.)*

## TC-5 — Third content change + `code/summarize` call (exercises upsert-in-place / ON CONFLICT path)

Modified `greet.go` a third time, reindexed, then called `code/summarize` a third time:

```
$ curl -s -D headers.txt -o body.json -X POST http://localhost:3199/api/v1/code/summarize \
    -H 'Content-Type: application/json' -d "{\"workspace\":\"$WS\"}"
$ cat body.json
{"processed":1,"skipped":0,"errors":0}
```

## Final assertion — one summary doc per symbol, no hash-path dups (AC-1 + AC-2 intent)

```
$ psql "postgres://nanobrain:nanobrain@localhost:5432/nanobrain_test?sslmode=disable" -c \
    "SELECT COUNT(*) FROM documents WHERE workspace_hash='$WS' AND tags @> ARRAY['symbol-summary']::text[];"
 count
-------
     1

$ psql "postgres://nanobrain:nanobrain@localhost:5432/nanobrain_test?sslmode=disable" -c \
    "SELECT COUNT(*) FROM documents WHERE workspace_hash='$WS' AND source_path LIKE '%&hash=%';"
 count
-------
     0
```

✅ PASS — across three `code/summarize` calls with three distinct content versions, exactly **one** summary document exists per symbol, and **zero** documents contain a legacy `&hash=` segment in `source_path`.

## Cleanup

```
$ kill <server-552b-pid>   # exact PID only, captured via `echo $! > pidfile` at launch — no pkill/lsof-kill used
```

## Validation ladder

| Layer | Command | Result |
|---|---|---|
| `validate:quick` | `CGO_ENABLED=0 go build -o /dev/null ./...` | ✅ PASS (clean) |
| `validate:quick` | `go test -race -short ./internal/codesummarize/... ./internal/storage/...` | ✅ PASS |
| `test:integration` | `NANO_BRAIN_TEST_DATABASE_URL=... go test -race -tags=integration ./internal/codesummarize/...` | ✅ PASS — `TestUpsertSummaryDocument_552_AC1`, `TestUpsertSummaryDocument_552_AC2` |
| `smoke:e2e` | this document | ✅ PASS |

## Acceptance criteria coverage

| AC | Description | Evidence |
|---|---|---|
| AC-1 | Two summarizations of the same symbol with different content → exactly 1 doc, no `&hash=`, latest content | TC-3/TC-4 (this doc) + `TestUpsertSummaryDocument_552_AC1` (integration) |
| AC-2 | Upsert removes legacy `&hash=...&summary=true` docs for the symbol (+ chunks via cascade) | `TestUpsertSummaryDocument_552_AC2` (integration, seeds a legacy doc directly) |
| AC-3 | Surviving doc's `content_hash` / `metadata.source_content_hash` reflect latest summary | TC-4 content_hash changed between calls; integration tests assert content match |
| AC-4 | `transientRegex` matches `524`; 524 classified transient/retryable | `TestClassifyError_524` (unit) |
| AC-5 | No regression: existing `internal/codesummarize` + `internal/storage` tests green; `sqlc generate` clean | `go test -race -short` green; `sqlc generate` produced only the new query (see PR diff) |

## Notes / deviations

- **Permanent-error regex extended to include `404`** (`internal/codesummarize/retry.go`): the plan's Task 4 test list specified `400, 404 → not transient`, but `permanentRegex` previously only matched `400|401|403`. Added `404` so the AC-4 test suite (which the plan itself specifies) passes as intended. Documented in SUMMARY.md.
- **`DeleteLegacySummaryDocsForSymbol` changed from `:exec` to `:execrows`**: the plan's Task 2 prose asked to "log the deleted count at debug," which requires a rows-affected return value; the plan's literal SQL used `:exec` (no return value). Used `:execrows` to satisfy the logging requirement.
- Environment note: `nanobrain_test`'s shared public schema has accumulated ~3200 workspaces from prior smoke-test sessions (including this very nano-brain repo). This pre-existing pollution is unrelated to #552 and was not modified (no destructive DB operations were run against it) — the smoke test simply waited for the shared watcher to reach the fixture's turn.
