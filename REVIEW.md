# nano-brain — Code Review

_Date: 2026-06-16 · Scope: `cmd/`, `internal/`, `migrations/`, harness docs/CI. Excludes `.opencode/worktrees/`, `web/`, vendored `node_modules/`._

> **Verification caveat:** the sandbox proxy blocked the Go toolchain download, so I could **not** run `go build`, `go vet`, `go test -race`, or `golangci-lint`. Every finding below is from static reading of the actual source. Treat "bug" findings as high-confidence-but-unverified until you compile/test them locally.

---

## 1. How it works (architecture)

nano-brain is a single static Go binary (`CGO_ENABLED=0`) that gives AI coding agents persistent memory and code intelligence. It indexes a codebase + past AI sessions into PostgreSQL 17 + pgvector and serves them over a REST API, an MCP server, and a small web UI.

**Entry & lifecycle** — `cmd/nano-brain/main.go` is a custom CLI dispatcher (no cobra). `serve` loads config → creates a pgx pool → runs goose migrations → wires the symbol/graph extractors, file watcher, embedding queue, search service, and MCP server → launches background goroutines (watcher, embed workers, harvest ticker, code-summarization workers, flow materializer) and the HTTP/MCP server (default `localhost:3100`).

**MCP surface** (`internal/mcp/tools.go`, ~1200 lines) — memory tools (`memory_query` hybrid search, `memory_search` BM25, `memory_vsearch` vector, `memory_get`, `memory_write`, `memory_tags`, `memory_status`, `memory_update`, `memory_wake_up`) plus code-intelligence tools (`memory_symbols`, `memory_graph`, `memory_impact`, `memory_trace`, `memory_flow`, `memory_workspaces_resolve`). All are workspace-scoped (SHA-256 hash of the project root; `"all"` for cross-workspace).

**Hybrid search** (`internal/search/`) — BM25 (`websearch_to_tsquery`) and pgvector legs run concurrently via `errgroup` (one leg failing degrades gracefully). Results are fused with **dynamic RRF** (the `k` constant adapts to the overlap between the two result sets), then re-scored by **recency** (exponential half-life decay), optional **entity/PageRank** boosts, and an optional **Cohere reranker**. Optional **HyDE** rewrites the query through an LLM before embedding.

**Ingestion** — the `fsnotify` watcher (`internal/watcher/`) detects file changes, applies glob/exclude/`.nano-brainignore` filtering, dispatches to a chunker (symbol-aware / heading / fixed), upserts `documents`+`chunks` (content-hash dedup), extracts symbols + graph edges, and enqueues chunks for embedding. The harvester (`internal/harvest/`) periodically ingests OpenCode (SQLite, read-only) and Claude Code (JSONL) sessions, with optional LLM summarization and regex-based auto-extraction of `DECISION:`/`LESSON:` markers.

**Storage** (`internal/storage/`) — pgx pool + sqlc-generated queries (hand-written SQL in `queries/*.sql`, codegen into `sqlc/` which is DO-NOT-EDIT). Data model: `workspaces`, `documents`, `chunks`, `embeddings`, `collections`, `graph_edges`, `entities`, `graph_context`, `graph_pagerank`, plus telemetry/budget tables (25 goose migrations).

Design is clean: constructor DI throughout, small consumer-side interfaces (`Embedder`, `Querier`, `Harvester`), zerolog structured logging, `fmt.Errorf("...: %w", err)` wrapping, graceful degradation, and idempotent content-hash upserts. It's a well-organized codebase.

---

## 2. Harness rules & harness checks

The repo runs a formal **engineering harness** (defined in root `AGENTS.md` + `docs/HARNESS.md` / `HARNESS_GATES.md`), driven by `scripts/harness-check.sh <phase>`.

**Lanes & gates** — work is risk-classified into `tiny` / `normal` / `high-risk` lanes; a 6-phase gate lifecycle (PRE-WORK → IN-PROGRESS → PRE-MERGE → ASYNC-PR-REVIEW → POST-MERGE → NEXT-READY → RETRO) must pass in order. OpenSpec-first is mandatory for any multi-file change. A GitHub issue is required before starting work.

**Validation ladder** — `validate:quick` (`go build ./... && go test -race -short ./...`) for every lane; `test:integration` (`-tags=integration`) and `smoke:e2e` for normal/high-risk; staged-files check forbids committing `.opencode/` or `package-lock.json`.

**Key forbidden practices** (from AGENTS.md): no `_ = err` on startup constructor calls, no claiming tests pass without output, no self-review, no work without an issue, no `nanobrain_dev` for tests (use `nanobrain_test` / port 3199), no direct commits to `master`.

### Compliance check (what I could verify statically)

| Rule | Status | Notes |
|---|---|---|
| No `_ = err` swallowed in `cmd/`/`internal/` startup | ✅ Pass | grep found none |
| Test coverage present | ✅ | 164 `_test.go` files, 25 integration-tagged |
| sqlc files untouched / generated | ✅ (assumed) | no manual edits evident |
| Minimal `panic()` in prod code | ⚠️ | only 1 (`internal/search/cursor.go:153`) — acceptable but see Bug L-1 |
| Low TODO debt | ✅ | only 3 TODOs in real source |

### Gaps in the harness itself

- **CI is weaker than the harness mandates.** `.github/workflows/ci.yml` runs only `go build` + `go test -race -short`. It does **not** run `golangci-lint` (despite `make lint` existing) nor the integration suite (`-tags=integration`) that the validation ladder requires for normal/high-risk lanes. The harness gates are enforced by an agent script, not by CI — so a human PR that skips the script gets much lighter checking than the docs imply.
- **CI uses `nanobrain_dev`** as the Postgres DB name (`ci.yml`), which contradicts the repeated "never use `nanobrain_dev` for testing" rule. Harmless (it's an ephemeral CI service container) but inconsistent with the stated convention; rename to `nanobrain_test` to match.
- **harness-check.sh is bash-only and unverified here** — it shells out to `gh`, `openspec`, and a running server; worth a periodic smoke test of the script itself.

---

## 3. Bugs & correctness issues

Grouped by severity. File:line references are to the real source tree.

### High

**B-H1 · Multiplicative score boosts are never re-sorted when recency is disabled — wrong ranking.**
`internal/search/service.go:~450`, `ranking.go:45,69`, `recency.go:13`.
Pipeline order is `DynamicRRFMerge` (sorted) → `DeduplicateResults` (returns *insertion* order) → `ApplyCodeAwareBoost` (mutates score, no sort) → `ApplyExtensionBoost` (mutates score, no sort) → `ApplyRecencyBoost`. The recency stage is the first thing that re-sorts, but it **early-returns without sorting** when `recencyWeight <= 0`. Entity/PageRank boosts only run when enabled and `workspace != "all"`. So with recency + entity + pagerank all off, a result that earned a 1.3× code-aware boost stays in its original RRF slot — the boost has no effect on order. Subtle because it only manifests under a specific config combination. **Fix:** re-sort after the multiplicative boosts (or make `ApplyRecencyBoost` always sort).

### Medium

**B-M1 · `sess.ID[:8]` can panic on short/empty session IDs.**
`internal/harvest/opencode_sqlite.go:222`. A malformed/empty `id` in the OpenCode SQLite DB panics the harvest goroutine. **Fix:** guard `if len(sess.ID) >= 8`.

**B-M2 · Snippet extraction mixes byte offsets with rune counts.**
`internal/search/snippet.go:48,60-90`. `bestPos`/`start` are byte offsets (`strings.Index`, `maxLen/2`) but the extraction loop treats them as rune counts. On multi-byte UTF-8 the snippet window lands in the wrong place / wrong length. ASCII unaffected. **Fix:** stay in bytes (back up to a rune boundary) or convert to rune indices consistently.

**B-M3 · Data race on `Runner.harvesters`.**
`internal/harvest/runner.go:46-53,62-68` vs `92-103`. `RunOnce` iterates the slice under `r.mu`; `AddHarvester`/`WithSummarizer` mutate it **without** the lock. Runtime harvester registration (e.g. multi-DB discovery) during a tick is a data race. **Fix:** take `r.mu` in the mutators. (Would be caught by `-race`, which I couldn't run.)

**B-M4 · Deleted files leave orphaned documents/chunks/embeddings.**
`internal/watcher/watcher.go:374-382` (TODO `story-2.x`). On file removal nothing is deleted from the DB, so stale docs accumulate and pollute search indefinitely. Acknowledged in-code but a real correctness gap.

### Low

**B-L1 · `panic(err)` in cursor encode.** `internal/search/cursor.go:153` — JSON-marshal of a fixed struct realistically can't fail, but a panic in a search path is undesirable; return an error instead.

**B-L2 · `DeduplicateResults` contract violation.** `internal/search/dedup.go:12-15` — documented to "return a new slice" but returns the input for `len ≤ 1`; latent aliasing footgun since downstream stages mutate in place.

**B-L3 · Non-atomic config write.** `internal/config/patch.go:85` — `os.WriteFile` can be observed truncated by a concurrent reader / on crash. Write-temp + `os.Rename`.

**B-L4 · Unchecked `c.Get("workspace").(string)` assertions.** `handlers/multi_get.go:30`, `graph.go:37`, `impact.go:36`, peers — panic if middleware ordering ever changes. Use the comma-ok form.

**B-L5 · Embedders don't validate vector length against `Dimension()`.** `embed/ollama.go:74`, `voyageai.go:88` — wrong-width vectors surface as opaque pgvector errors instead of a clear dimension mismatch.

**B-L6 · `multiGet` is N+1.** `handlers/multi_get.go:45-80` — one query per id/path; batch it if request sizes grow.

> Areas checked and found **correct**: RRF math + overlap `k`, cursor bounds/overflow guard, MCP pagination slicing, embed-queue inflight/pending accounting + no-OFFSET-loop guard, eventbus double-close protection, errgroup degrade-on-error, summarize map-reduce concurrency, graph impact BFS cycle guard, SQLite `rows.Err()`/`Close` handling.

---

## 4. Gaps & what's missing

- **Doc drift (notable):** `CHANGELOG.md` states *"Replaced sqlite-vec with Qdrant as sole vector store"* and mentions a Qdrant `project_hash` payload filter, but the actual code has **zero Qdrant references** and uses **pgvector** everywhere (`internal/embed/queue.go`, `handlers/search.go`, `storage/sqlc/embeddings.sql.go`), matching `CLAUDE.md`/`AGENTS.md`. The CHANGELOG describes a vector backend the code doesn't use — reconcile it.
- **No `LICENSE` file** despite a public npm package and GitHub releases. Add one (or mark proprietary) before further distribution.
- **CI doesn't enforce lint or integration tests** (see §2) — the biggest process gap.
- **No HTTP request-body size limit** — every POST body is buffered fully before per-handler size checks (`server/middleware.go` `io.ReadAll`); no `echo` `BodyLimit` registered. DoS/memory exhaustion vector (also S-M1).
- **Stale-document cleanup unimplemented** (B-M4) — there's no reconciliation pass to remove docs for deleted files.
- **Shipped `docker-compose.yml` is a footgun** — binds `0.0.0.0`, no auth, default `nanobrain:nanobrain` creds, publishes Postgres `5432` (also S-M2/M3).

---

## 5. Security

**Threat model:** local-first, self-hosted, single-user, `localhost`-only by intent. A startup **bind-safety gate** (`cmd/nano-brain/bindsafety.go`) refuses to bind non-loopback without auth unless `--unsafe-no-auth` is passed — good. The realistic adversaries are therefore (a) malicious local processes / **web pages in the user's browser** hitting `localhost:3100`, and (b) misconfiguration that exposes the port. The code shows real security awareness: parameterized SQL throughout, constant-time auth compares, secret redaction in logs/config API, CSP on the UI.

**No SQL injection, no command injection, no secret logging found.** All DB access is sqlc/parameterized; full-text search uses `websearch_to_tsquery($1)`; the one dynamic SQL fragment (`harvest/opencode_sqlite.go:295`) only assembles `?,?,…` placeholders. `harvest/git.go` uses `exec.Command("git", args...)` with no shell and non-user args. Provider keys are sent only as `Authorization: Bearer` to configured endpoints and masked in logs.

### High

**S-H1 · CSRF protection covers only a subset of state-changing endpoints.**
`internal/server/routes.go:31-138`, `middleware/csrf.go`. A solid origin-checking CSRF middleware exists but is attached **only to the `write` subgroup**. Unprotected mutating routes include `POST /api/v1/init`, `POST /api/v1/config`, `DELETE /api/v1/workspaces/:hash`, `POST /api/v1/reset-workspace`, collection CRUD, `DELETE /api/v1/documents/:id`, `POST /api/harvest`, `POST /api/reload-config`, and the entire **`/mcp` + `/sse`** tool surface. With auth off by default, any web page the user visits can drive these cross-origin. **Fix:** apply the origin/CSRF check (or a `Sec-Fetch-Site` gate) to all non-GET routes, and validate `Origin` on `/mcp` (the MCP spec recommends this against DNS-rebinding).

**S-H2 · `POST /api/v1/init` indexes an attacker-chosen host directory.**
`handlers/workspace.go:110-174`. `root_path` from the request body is registered with glob `**/*` and a recursive watcher attached — no allowlist, no confinement, no CSRF. A caller can point nano-brain at `~/.ssh`, `/etc`, `/`; contents get ingested into Postgres and become readable via the search/get API. Combined with no-auth-default + S-H1 → **arbitrary host-file exfiltration via the index**. **Fix:** treat `init` as privileged (auth/CSRF), and ideally require out-of-band/CLI confirmation for new roots.

**S-H3 · Config-patch + reload enables SSRF and provider-key exfiltration.**
`config/patch.go:14-38`, `handlers/config.go`, `handlers/reload.go`. `embedding.url` and `summarization.provider_url` are patchable with no scheme/host validation. An attacker reaching `POST /api/v1/config` (no auth/CSRF) repoints these to a server they control; subsequent embedding/summarization/HyDE/rerank calls then ship indexed content **and the `Authorization: Bearer <api_key>`** to the attacker. **Fix:** gate config behind auth/CSRF; validate provider URLs (scheme allowlist, block loopback/link-local/metadata ranges); don't attach the key to unexpected hosts.

### Medium

**S-M1 · No request-body size limit (DoS)** — `server/middleware.go` `io.ReadAll` with no cap; no `echo` `BodyLimit`. Register a global body limit / `http.MaxBytesReader`.

**S-M2 · Shipped `docker-compose.yml` binds `0.0.0.0`, no auth, default PG creds, publishes 5432.** Either it won't boot (bind-safety gate) or it nudges users toward `--unsafe-no-auth`. Bind `127.0.0.1`, require auth, don't publish Postgres, use strong creds.

**S-M3 · Docker images run as root** — root `Dockerfile` (alpine, no `USER`) and `docker/Dockerfile` (distroless default user is root). Add a non-root `USER`.

### Low / informational

- **S-L1** Auth disabled by default — reasonable for localhost (and the bind gate backstops it), but it's the linchpin that makes S-H1–H3 reachable by local browser/process adversaries. Document and recommend auth-on for any non-trivial deployment.
- **S-L2** No per-client cap on SSE/`/sse` connections — mild local DoS, bounded by fd limits.
- **S-L3** Bearer-token loop can leak the *number* of configured tokens via timing (each compare is constant-time, the loop isn't). Negligible.
- **S-L4** Security headers/CSP applied to the `/ui` group only, not the JSON API. Cheap to add globally.
- **S-L5** No CORS middleware — **correct** for this design (same-origin UI + CSRF). Do not add permissive CORS.

---

## 6. Top recommendations (priority order)

1. **S-H1** — apply CSRF/origin checking to *all* state-changing routes, especially `/mcp` and `/api/v1/config`. This single change closes the local-browser path that makes S-H2 and S-H3 exploitable.
2. **B-H1** — re-sort after the multiplicative search boosts so ranking is correct when recency is off.
3. **S-H2 / S-H3** — make `init` and `config` privileged + validate provider URLs.
4. **Strengthen CI** — add `golangci-lint` and the integration suite to `ci.yml`; rename CI DB to `nanobrain_test`.
5. **S-M1** — add an HTTP body-size limit.
6. **B-M1/M2/M3** — guard `sess.ID[:8]`, fix snippet byte/rune mixing, lock `Runner.harvesters`.
7. **Housekeeping** — reconcile the Qdrant/pgvector CHANGELOG drift, add a `LICENSE`, implement deleted-file cleanup (B-M4), harden the docker-compose/Dockerfiles.

_Build/test verification (`go build`, `go vet`, `go test -race`, `golangci-lint`) could not be run in this environment — please run the validation ladder locally to confirm the bug findings._
