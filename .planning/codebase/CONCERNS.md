# Codebase Concerns — nano-brain

> Generated: 2026-06-28 | Scope: 5,250 Go files, 211K LOC, 370 internal packages

## Table of Contents

1. [Critical (P0)](#critical-p0)
2. [High (P1)](#high-p1)
3. [Medium (P2)](#medium-p2)
4. [Low (P3)](#low-p3)
5. [Summary Table](#summary-table)

---

## Critical (P0)

### C1. CVE-2026-34742 — DNS Rebinding in MCP Go SDK

**Package:** `github.com/modelcontextprotocol/go-sdk v0.8.0`
**Published:** 2026-06-25
**Impact:** DNS Rebinding Protection Disabled by Default — affects 13 files in `internal/mcp/` and `internal/server/server.go:13`

The MCP Go SDK used by nano-brain has a known CVE that allows DNS rebinding attacks against servers running on localhost. Since nano-brain's primary interface is the MCP endpoint at `POST /mcp`, this directly affects the attack surface.

**Action:** Bump to a fixed version as soon as upstream releases one. Track via `govulncheck ./...`.

---

### C2. Data Race — `Server.SetHarvestStatus` (server.go:214-227)

**File:** `internal/server/server.go:214-227`

`SetHarvestStatus` mutates `s.harvestStatus` **without any lock**. Concurrent reads happen from:
- `routes.go:24,112` (startup, single-threaded — safe)
- Health/stats handler goroutines via `s.healthHandler.SetHarvestStatus` / `s.statsHandler.SetHarvestStatus`

Five other fields on `Server` use dedicated `sync.RWMutex` (`harvestMu`, `summarizeMu`, `codeSummarizeMu`, `flowMaterializeMu`, `configMu`). This field is the obvious outlier.

**Action:** Reuse `harvestMu` (already protects `harvestRunner` on L48) or switch to `atomic.Pointer[handlers.HarvestStatusSnapshot]`.

---

## High (P1)

### H1. Swallowed `json.Marshal` Errors — 13 Production Sites

`_, _ := json.Marshal(...)` or `meta, _ := json.Marshal(...)` discards the error. If marshalling fails, metadata is stored as `nil`, silently degrading stored context. No log, no error propagation.

| File | Line | Context |
|------|------|---------|
| `internal/links/extract.go` | 140 | Wikilink metadata for `KindID` |
| `internal/links/extract.go` | 163 | Wikilink metadata for `KindTitle` |
| `internal/links/extract.go` | 199 | `links_changed` event payload |
| `internal/summarize/persist.go` | 97 | Summary document metadata |
| `internal/flow/materializer.go` | 210 | Flow document metadata |
| `internal/harvest/automemory.go` | 83 | Auto-memory document metadata |
| `internal/harvest/claudecode.go` | 176 | Session metadata |
| `internal/harvest/runner.go` | 133 | Harvest event payload |
| `internal/watcher/watcher.go` | 843 | File event metadata |
| `internal/watcher/watcher.go` | 1127 | Reindex event payload |
| `internal/server/handlers/reindex.go` | 358 | Reindex event payload |
| `internal/server/handlers/events.go` | 74 | SSE hello payload |
| `internal/mcp/tools.go` | 56 | MCP tool schema |

---

### H2. Partial Writes Without Compensation

**`internal/links/extract.go:179-195`** — Old edges are deleted first (L179), then new edges are upserted one by one (L187). If any `UpsertReferenceEdge` fails mid-loop, the document loses its reference edges for the failed targets. No compensation or rollback.

**`internal/summarize/persist.go:126-146`** — Document is upserted (L112), then chunks are upserted in a loop. Failed chunk upserts are skipped via `continue`. Document exists with fewer chunks than expected — embeddings will be generated for fewer chunks.

**Action:** Wrap in a transaction, or add rollback on partial failure.

---

### H3. Config Hot-Reload Is Broken By Design

**File:** `internal/config/reload.go:14-66`

`Reload()` classifies which fields are "reloaded" vs "require restart" but **does nothing else** — no RWMutex, no atomic pointer, no signal to in-flight goroutines. The HTTP handler that calls it has undefined semantics:
- If it assigns `cfg = newCfg`, concurrent reads see torn reads
- If it does nothing, the "Reloaded" fields are never actually reloaded
- 24 fields are marked `reloadable: false` but change behavior at runtime (e.g., `embedding.concurrency`, `watcher.debounce_ms`)

Additionally, `ApplyPatch` (patch.go:57-89) reads + rewrites the YAML file with no locking against concurrent `Reload` or `Load`.

**Action:** Either wire `Reload` into running subsystems via RWMutex-protected config pointer, or remove the misleading "Reloaded" classification.

---

### H4. Watcher Mutex Held During I/O

**File:** `internal/watcher/watcher.go:84`

`w.mu` is a plain `sync.Mutex` (not `RWMutex`). Every fsnotify `Add`/`Remove` call happens under it (L227, L308, L460, L478), and the recursive subdirectory walk (`watchDir`) acquires it for every directory. Read-heavy paths like `processDirty` (L412) and `CollectionsWatched` (L118) serialize behind these.

**Action:** Promote to `sync.RWMutex` and narrow critical sections — snapshot collections, then call fsnotify outside the lock.

---

### H5. Three Fire-and-Forget Goroutines Use `context.Background()`

| File:Line | Context | Risk |
|-----------|---------|------|
| `internal/server/handlers/reindex.go:372` | `TriggerUpdate` HTTP handler | Runs forever after server shutdown; multiple concurrent triggers each spawn unbounded goroutines |
| `internal/graph/pagerank_service.go:48` | `IncrementEdgeCount` threshold trigger | No cancellation tied to server lifecycle |
| `internal/telemetry/telemetry.go:24` | Per-request `Record` | Leaks per call on shutdown; bounded to request rate |

**Action:** Plumb the server's shutdown context (or accept it as a parameter). For telemetry, add a `sync.WaitGroup` so `Shutdown` waits for in-flight inserts.

---

### H6. No HTTP Rate Limiting

The server has no request-level rate limiting on any endpoint. The only rate limiting in the codebase is:
- `internal/watcher/watcher.go:1116-1119` — file re-index rate limiter per workspace (10/sec)
- `internal/summarize/client.go:55` — LLM API call rate limiter

POST endpoints like `/api/v1/write`, `/api/v1/reindex`, `/api/v1/summarize` are unbounded. A malicious or misconfigured agent could overwhelm the server.

**Action:** Add Echo middleware rate limiter or per-IP token bucket on write endpoints.

---

### H7. Auth Bypass via `BypassPaths` Config

**File:** `internal/server/middleware/auth.go:36-41`

The auth middleware checks `cfg.BypassPaths` for exact path matches. The default config includes `/health` (defaults.go:22). If an operator adds `"/"` or `"/api"` to `BypassPaths`, all endpoints are unprotected. There's no warning log when bypass paths are active.

**Action:** Log a warning at startup when `auth.enabled=true` AND `bypass_paths` is non-default. Consider rejecting `"/"` as a bypass path.

---

### H8. `summarize/pipeline.go:156-181` — Unnecessary Mutex During LLM Calls

A single `mu sync.Mutex` is held for the entire `wg.Wait()`. Each goroutine acquires `mu` before writing to `results[idx]`, but since `idx` is unique per goroutine, the mutex is unnecessary — direct indexed writes to a pre-sized slice are race-free. This serializes all concurrent LLM completion handlers through one lock for the duration of the slowest call.

**Action:** Remove the mutex; pre-allocate `results` slice and write directly to `results[idx]`.

---

## Medium (P2)

### M1. Large Files Needing Decomposition

| File | LOC | Concern |
|------|----:|---------|
| `internal/mcp/tools.go` | 2,361 | 16 tool handlers in one file. Split per-tool (one file per MCP tool) like handler files already are |
| `internal/watcher/watcher.go` | 1,434 | Mixes fsnotify loop, DB upserts, gitignore stack, symbol/graph extraction, notification callbacks. `processFile` (L686-830) does 5 different concerns |
| `internal/search/service.go` | 991 | `HybridSearch` body (L131-609) is 478 lines. Extract RRF/recency/rerank dispatch to existing `rrf.go`/`recency.go` |
| `internal/graph/js_cflow.go` | 876 | Complex tree-sitter CFG builder — acceptable for now but worth monitoring |

---

### M2. `watcher.flowNotify` Synchronous Callback

**File:** `internal/watcher/watcher.go:826-828`

`w.flowNotify(col.workspaceHash)` fires synchronously inside `processFile`, on the same goroutine as the file walk. If the notify handler triggers heavy flow materialization, it stalls every subsequent file processing for the collection.

**Action:** Defer to a goroutine or bounded queue.

---

### M3. `fmt.Errorf` with `%v` Instead of `%w` — Error Chain Lost

| File | Line | Code |
|------|------|------|
| `internal/mcp/flowchart.go` | 135 | `fmt.Errorf("invalid start line: %v", err)` |
| `internal/mcp/flowchart.go` | 139 | `fmt.Errorf("invalid end line: %v", err)` |
| `internal/search/service.go` | 480 | `fmt.Errorf("all search legs failed: bm25: %v, vector: %v", ...)` |
| `internal/search/service.go` | 927 | `fmt.Errorf("all search legs failed: bm25: %v, vector: %v", ...)` |

The `service.go` cases aggregate two errors (intentional), but `flowchart.go` cases are simple wrapping misses that break `errors.Is()`/`errors.As()`.

---

### M4. Error Handling — Log-and-Continue Data Loss Risks

| File | Lines | Risk |
|------|-------|------|
| `internal/server/handlers/embed.go` | 90-122 | Embed loop stops on first error but HTTP response reports success with partial count; `CountPendingChunks` error shows `remaining: 0` |
| `internal/summarize/persist.go` | 141-143 | Chunk upsert errors logged, `continue` — partial chunks silently lost |
| `internal/flow/materializer.go` | 237, 261, 283, 289 | Flow doc/chunk upsert errors logged, no return — mermaid diagram may be missing nodes |
| `internal/graph/pagerank_service.go` | 60 | `logger.Error()` then no return — PageRank scores not stored, downstream boost uses stale data |
| `internal/server/handlers/stats.go` | 141-166 | Six queries logged on error, all return partial stats with `0` values — caller can't distinguish "zero" from "failed" |

---

### M5. Embed Queue — Fragile Hard-Failure Detection

**File:** `internal/embed/queue.go:464-475`

Hard-failure detection uses string-matching on error messages (`"<provider>: unexpected status <N>:"`). If `ollama.go:64` or `voyageai.go:76` changes the error format, hard-failure detection silently breaks — chunks that should be marked `embed_failed` will retry forever.

**Action:** Use typed errors with `errors.Is()` sentinels instead of string matching.

---

### M6. Test Coverage Gaps

**Overall:** 41,723 test LOC / 41,375 source LOC = 1.01x ratio. Good baseline.

**Packages with low test/source ratio:**

| Package | Source LOC | Test LOC | Ratio | Concern |
|---------|----------:|---------:|------:|---------|
| `internal/codesummarize` | 1,318 | 309 | 0.23x | 620-line `service.go` has zero direct tests |
| `internal/chunker` | 379 | 118 | 0.31x | Low-level chunking logic undertested |
| `internal/symbol` | 580 | 382 | 0.66x | Symbol registry undertested |
| `internal/intelligence` | 615 | 482 | 0.78x | Categorization/consolidation undertested |
| `internal/watcher` | 1,865 | 1,484 | 0.80x | `extractAndUpsertSymbols`, `extractAndUpsertEdges`, `Run()`, `Watch()` untested |

**Large files with limited unit coverage:**
- `internal/mcp/tools.go` (2,361 lines) — 23 unit tests focus on schema/registration; the 13 `registerMemory*` handler bodies are mostly untested at unit level
- `internal/watcher/watcher.go` (1,434 lines) — 24 tests cover filtering/debounce; symbol/edge extraction pipeline untested
- `internal/search/service.go` (991 lines) — `HybridSearch` 478-line body has limited direct unit coverage

---

### M7. Missing Config Validation

**File:** `internal/config/config.go:409-521`

- **No URL format validation** for `Database.URL`, `Embedding.URL`, `Summarization.ProviderURL`, `HyDE.ProviderURL`, `Reranking.ProviderURL` — typos pass silently, fail deep in goroutines
- **No positive value validation** for `Embedding.MaxChars`, `Watcher.ChunkOverlap`, `Intelligence.Concurrency`, and ~15 other numeric fields — zero/negative accepted
- **No embedding provider validation at Load time** — `Embedding.Provider` is free-form; error only surfaces at queue construction

---

### M8. Docker Security Concerns

**File:** `Dockerfile`

- **No `USER` directive** — runs as root inside the container. A nano-brain RCE would have root.
- **No `HEALTHCHECK` directive** — Docker doesn't auto-restart on hang
- **Build context is entire repo** (`COPY . .`) — wastes bandwidth, includes `.git`, `.opencode/`, `node_modules/`
- **`alpine:3.21`** is not the latest (3.22 available since 2026-05)
- **No `mem_limit` / `cpus` / `pids_limit`** in `docker-compose.yml` — runaway embed queue could exhaust resources

---

### M9. Missing `SecretFieldPaths` Entries

**File:** `internal/config/secrets.go`

`code_summarization.api_key` and `intelligence.api_key` are **NOT** in `SecretFieldPaths`. They will be exposed in JSON config dumps via `GET /api/v1/config`.

**Action:** Add both to `SecretFieldPaths`.

---

### M10. No HTTP Server Timeouts

**File:** `internal/server/server.go:178-182`

The server starts with `s.echo.Start(addr)` — no `ReadTimeout`, `WriteTimeout`, `IdleTimeout`, `ReadHeaderTimeout`, or `MaxHeaderBytes` configured. A slow client can hold connections indefinitely, exhausting server resources. Combined with no HTTP rate limiting (H6), this amplifies the DoS surface.

**Action:** Set `ReadTimeout`, `WriteTimeout`, `IdleTimeout` on the Echo server. Consider `MaxHeaderBytes` to bound header parsing.

---

### M11. No TLS Enforcement

**File:** `internal/server/server.go:178-182`

Plaintext HTTP only. All API traffic including `Authorization: Basic` and `Bearer` tokens is sent in cleartext. This is a design choice (assumes loopback or reverse proxy), but the README and config docs do not explicitly mandate TLS termination in production.

**Action:** Document the TLS requirement. Consider adding a startup warning when `server.host` is not `localhost`/`127.0.0.1` and TLS is not configured.

---

### M12. Config Patch Type Safety Gap

**File:** `internal/config/patch.go:57-89`

`PatchConfig` uses `PatchableFieldPaths` allowlist (patch.go:14-38) but does not validate type safety of values. The `setNodeValue` function converts via `json.Marshal(value)` then YAML unmarshal — type-confusion is possible if an attacker sends a `map[string]interface{}` for an int field (e.g., `server.port`). YAML's type coercion could silently produce zero-value results.

---

### M13. Defaults Drift Between Config and Queue

**Files:** `internal/config/defaults.go:33` vs `internal/embed/queue.go:42`

`MaxChars = 3000` is hardcoded in both places. If one changes, the other silently disagrees. The queue constant is the actual fallback when config is unset.

Additionally, `RerankingConfig` and `BenchConfig` are absent from `getDefaults()` — the defaults struct drifts from the actual `Config` struct.

---

## Low (P3)

### L1. `context.Background()` in Internal Code (Acceptable with Caveats)

14 instances in `internal/` production code. Most are acceptable:
- `internal/health/doctor/doctor.go` — health checks with `WithTimeout` (correct)
- `internal/graph/pagerank_service.go:48` — fire-and-forget (see H5)
- `internal/watcher/watcher.go:620` — `cleanupDeletedDocument` (acceptable, short-lived)

The pattern is widespread but not harmful except where noted in H5.

---

### L2. `nolint:errcheck` on Transaction Rollback (Acceptable)

6 instances across `internal/watcher/watcher.go` and `internal/harvest/automemory.go`:

```go
defer tx.Rollback() //nolint:errcheck
```

This is the standard Go pattern — `Rollback` after `Commit` is a no-op, and the error is intentionally ignored. Acceptable.

---

### L3. `panic()` in Production Code

**File:** `internal/search/cursor.go:153`

```go
panic(err)  // JSON marshaling of simple struct should never fail
```

This is in `EncodeCursor` for a `json.Marshal` of a simple `{Offset, QueryHash}` struct. The comment is accurate — this should never fail. Low risk but worth noting.

---

### L4. Bare `return err` Without Context Wrapping

Several files return errors from sub-calls without adding contextual wrapping:

| File | Lines |
|------|-------|
| `internal/links/extract.go` | 135, 146, 175, 183, 194 |
| `internal/server/middleware.go` | 106 |
| `internal/config/patch.go` | 108, 112 |

The caller cannot distinguish which sub-call failed. Low severity since the call chains are short.

---

### L5. `expandPaths` Is Dead Code

**File:** `internal/config/config.go:404-406`

```go
func expandPaths(cfg *Config) error {
    return nil  // no-op
}
```

The comment claims it expands tildes, but the function does nothing. Actual tilde expansion happens inline for `Summarization.OutputDir` only (config.go:386-392).

---

### L6. `lib/pq` Dependency (Legacy)

**File:** `go.mod:16`

`github.com/lib/pq v1.12.3` appears as a transitive dependency of `jackc/pgx/v5/pgconn`. It's not directly used but is a legacy driver. No action needed, but worth noting.

---

### L7. No `govulncheck` in CI

**File:** `.github/workflows/ci.yml`

The CI pipeline runs `go build` + `go test -race -short` but does not run `govulncheck`. Given the go-sdk CVE (C1), this would have caught it.

**Action:** Add `govulncheck ./...` to CI.

---

## Summary Table

| ID | Severity | Category | Title | Status |
|----|----------|----------|-------|--------|
| C1 | **P0** | Security | CVE-2026-34742 — DNS Rebinding in MCP Go SDK | **Open** |
| C2 | **P0** | Concurrency | `Server.SetHarvestStatus` data race | **Open** |
| H1 | P1 | Correctness | 13 swallowed `json.Marshal` errors | **Open** |
| H2 | P1 | Correctness | Partial writes without compensation | **Open** |
| H3 | P1 | Config | Hot-reload is broken by design | **Open** |
| H4 | P1 | Concurrency | Watcher mutex held during I/O | **Open** |
| H5 | P1 | Concurrency | 3 fire-and-forget goroutines (no ctx) | **Open** |
| H6 | P1 | Security | No HTTP rate limiting | **Open** |
| H7 | P1 | Security | Auth bypass via BypassPaths | **Open** |
| H8 | P1 | Performance | Unnecessary mutex in summarize/pipeline | **Open** |
| M1 | P2 | Maintainability | 4 files >876 LOC need decomposition | **Open** |
| M2 | P2 | Performance | watcher.flowNotify synchronous callback | **Open** |
| M3 | P2 | Correctness | `fmt.Errorf` uses `%v` not `%w` | **Open** |
| M4 | P2 | Correctness | Log-and-continue data loss risks | **Open** |
| M5 | P2 | Maintainability | Fragile string-match error detection | **Open** |
| M6 | P2 | Testing | Test coverage gaps (5 packages <0.8x) | **Open** |
| M7 | P2 | Config | Missing URL/numeric validation | **Open** |
| M8 | P2 | Security | Docker runs as root, no healthcheck | **Open** |
| M9 | P2 | Security | API keys not in SecretFieldPaths | **Open** |
| M10 | P2 | Security | No HTTP server timeouts | **Open** |
| M11 | P2 | Security | No TLS enforcement | **Open** |
| M12 | P2 | Security | Config patch type safety gap | **Open** |
| M13 | P2 | Config | Defaults drift between config and queue | **Open** |
| L1 | P3 | Code Quality | `context.Background()` in internal (14 sites) | **Open** |
| L2 | P3 | Code Quality | `nolint:errcheck` on tx.Rollback | **Open** |
| L3 | P3 | Code Quality | `panic()` in cursor.go | **Open** |
| L4 | P3 | Code Quality | Bare `return err` without wrapping | **Open** |
| L5 | P3 | Code Quality | `expandPaths` dead code | **Open** |
| L6 | P3 | Dependencies | Legacy `lib/pq` transitive dep | **Open** |
| L7 | P3 | CI/CD | No `govulncheck` in CI | **Open** |
