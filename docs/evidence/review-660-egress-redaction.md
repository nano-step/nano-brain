# Independent Review — #660 Egress-Only Credential Redaction

Reviewer: independent-agent (R88)
Date: 2026-11-22
Verdict: **MERGE_WITH_NITS** (round 1 SEND_BACK → all blocking findings verified fixed in round 2; see addendum)

The wiring is correct and complete — every provider-boundary call site scrubs, and no storage path touches `redact`. However, the `secret-assignment` pattern has two *systematic* coverage holes that let clearly credential-shaped text reach providers in the most common secret file formats (JSON configs and `*_TOKEN`/`*_SECRET` env vars). For a security control whose sole purpose is masking credential-shaped text, these are fix-now defects, not residual risk; both are ~5-line pattern changes plus table tests.

---

## Findings

### Blocking

**B1 — `secret-assignment` does not match quoted keys, so JSON-style `"key": "value"` secrets leak.**

`internal/redact/redact.go:62`

The optional `["']?` sits *after* the `[:=]` separator but there is nothing to absorb a closing quote *after* the key name. For `{"api_key": "abcd1234efgh"}` the capture group ends at `api_key`, then `[:=]` must match `"` — it fails, and no backtracking alternative exists. Result: `{"api_key": "..."}`, `"password": "hunter2xx"`, `"secretKey": "..."` (Go map literals, `appsettings.json`, `package.json`, config examples — among the most common secret-carrying formats in harvested code/docs) pass through unredacted. YAML `api_key: v` and unquoted JS `apiKey: "v"` *do* match, making the gap non-obvious.

Fix applied (round 2): `(\w*(names)\w*)["']?\s*[:=]\s*(?:"[^"]{8,}"|'[^']{8,}'|[^\s"',;$}\[]{8,})` + table tests for `{"api_key": "…"}`, `"password": "…"`, `apiKey: "…"` — verified PASS.

Prevented by: a positive test case with a JSON `"key": "value"` pair — existed and not followed (round 1), now covered.

**B2 — Key-name alternation misses the `*_TOKEN` and `*_SECRET` families and common provider token shapes.**

`internal/redact/redact.go` alternation and shape list

The alternation covered `auth_token`, `access_token`, `refresh_token` but **not** bare `token`/`api_token`/`github_token`/`gitlab_token`/`private_token`/`bearer_token` — `GITHUB_TOKEN=<unshaped>` leaked. Symmetrically, `secret` only appeared inside `secret_key`/`api_secret`/`client_secret`, so `JWT_SECRET`, `SESSION_SECRET`, `AUTH_SECRET` (NextAuth), `APP_SECRET`, `WEBHOOK_SECRET` all leaked — `JWT_SECRET=…` is arguably the single most common secret assignment in modern web codebases. The shape list also lacked `glpat-` (GitLab), `hf_` (HuggingFace), `xapp-`/`xoxe` (Slack app-level), `rk_live_`/`rk_test_`/`whsec_`/`sk_live_`/`sk_test_` (Stripe restricted/webhook/secret).

Fix applied (round 2): `token`, `secret`, `api[_-]?token` added to the alternation; shapes extended with `glpat-`, `hf_`, `xapp-`, `(sk|rk|whsec)_(live|test)_`. Table tests for `JWT_SECRET=…`, `SESSION_SECRET=…`, `GITHUB_TOKEN=…`, `GITLAB_TOKEN=glpat-…`, `sk_live_…`, `hf_…`, `xapp-…` — verified PASS.

Prevented by: a negative sweep listing real-world `.env` key names against the alternation — existed and not followed (round 1), now covered.

### Nits / residual risks (non-blocking)

- **N1 — False-positive masking of non-secret names** (e.g. `password_reset_url=https://…`, `authToken: localStorage.getItem("x")`). Stored content is unaffected; only degrades provider input. Accepted for a shape-based scrubber — author added a comment in `redact.go` documenting this (round 2).
- **N2 — Quoted values containing spaces leaked** (`password="my secret pass"`): fixed in round 2 via quoted-value branches.
- **N3 — Double scan per pattern** (`MatchString` then `ReplaceAllString`): fixed in round 2.
- **N4 — Boundary-test asymmetry**: wire-level proof existed only for Ollama + summarize.Client; round 2 added `TestVoyageAIEmbedder_Embed_RedactsSecrets`. Remaining sites (reranker, hyde, preprocess, codesummarize, bench) verified by inspection of one-line `Scrub` calls — a shared boundary-test helper is reasonable follow-up hardening, not blocking.
- **N5 — Doc-count drift** in self-review evidence: corrected in round 2.
- **N6 — "covers intelligence" overstated reach** (`NewConsolidator`/`NewCategorizer` have no production callers; interface would be satisfied by scrubbed client if wired): wording corrected in round 2.
- **Residual (accepted, matches author's self-review):** shapeless high-entropy secrets; MCP/REST responses returning stored content to *local* clients; partial `sk-` fragments split by the 3,000-char embed truncation (harmless — an incomplete key isn't a credential).

## Persistence Sweep

**PASS — egress-only invariant holds.** `internal/redact` is imported by exactly 8 files, all provider-boundary: `embed/ollama.go`, `embed/voyageai.go`, `summarize/client.go`, `codesummarize/provider.go`, `search/reranking/reranker.go`, `search/hyde/generator.go`, `search/preprocess/preprocessor.go`, `bench/generate.go`. No import in `internal/storage`, `internal/harvest`, `internal/chunker`, `internal/watcher`, or any `internal/server/handlers` write path. `internal/storage/redact.go` is a **pre-existing** unrelated DSN-error scrubber (#307) — does not touch stored content. Reranker returns original unscrubbed `docs` mapped by index; preprocessor keeps `OriginalQuery` raw. Stored `documents`/`chunks`/sessions remain byte-identical.

## State-lifetime Sweep

**PASS.** Scrub placement is inside the egress call on the final outbound text: `Embed()` scrubs before marshal; `ChatCompletion` scrubs once before `bodyBytes` — retries reuse the already-scrubbed body. `callAPI` scrubs per-doc before marshal; `generateLLMQuery` scrubs *before* the 3,000-char truncation — correct order. Pattern ordering sound: `secret-assignment` runs last and its value class excludes `[`, preventing re-consumption of `[REDACTED:x]` markers. `${1}=[REDACTED]` preserves full key incl. `\w*` affixes. No ReDoS: Go's `regexp` is RE2 — linear on multi-MB input.

## Laundering Sweep

**PASS for call-graph laundering.** Every `net/http` outbound site in non-test code under `internal/` enumerated: the 8 scrubbed provider calls plus `internal/health/doctor/doctor.go` (GETs + POST a *fixed* JSON-RPC `initialize` body — no user/stored content). All `internal/server/handlers/**` and `internal/mcp/**` `net/http` usage is *inbound*. No handler proxies stored content outbound. `cmd/nano-brain/client*.go`, `collection.go`, benchmark runners POST only to the local nano-brain server. Telemetry writes to DB, not network. All LLM callers route through the scrubbed client: `summarize.Pipeline`, `flow.NewLLMFlowSummarizer` (takes `*summarize.Client` concretely), `codesummarize` retries → `provider.SummarizeBatch`. The remaining laundering channel was **pattern-level** — closed by B1/B2 fixes in round 2.

## Real-path Checks

**PASS.** `embed.NewFromConfig` (`main.go:496`) is the only production embedder construction; `NewOllamaEmbedder`/`NewVoyageAIEmbedder` appear only in `factory.go` and tests. The single `embedder` instance is shared by the queue, `SearchService` query-time embedding, `/api/v1/search` embed handler, embed handler, and MCP tools — all funnel through `Embedder.Embed`, which scrubs internally. `serve_only` mode still scrubs query-time embeds. `QueryCache` keyed on raw text only skips the round-trip on hit — no unscrubbed egress. Docs accurately describe coverage.

## Honest Gaps

- Reviewer profile had no exec tool — regex verdicts derived by manual RE2 trace; round-2 table tests now cover every finding mechanically (author-executed: all PASS).
- `internal/intelligence` has no production callers — coverage verified at interface level only (N6).
- Nothing mechanically stops a future provider client from skipping `Scrub` (N4) — accepted as follow-up hardening, consistent with the repo's AGENTS.md-note approach.

---

## Addendum — Round 2 author resolution

All blocking findings (B1, B2) and nits N2, N3, N4(partial), N5, N6 were fixed
in the follow-up commit on this branch:

- Assignment pattern: `["']?` after key capture + quoted-value branches.
- Alternation: `token`, `secret`, `api[_-]?token` added.
- Shapes: `glpat-`, `hf_`, `xapp-`, `(sk|rk|whsec)_(live|test)_`.
- 13 new table cases (JSON pairs, camelCase, `JWT_SECRET`, `GITHUB_TOKEN`,
  `GITLAB_TOKEN=glpat-`, `sk_live_`, `hf_`, `xapp-`, quoted multi-word) —
  `go test ./internal/redact` all PASS.
- `TestVoyageAIEmbedder_Embed_RedactsSecrets` added.
- `MatchString` pre-check removed; evidence docs corrected.

**Reviewer re-check (round 2): MERGE_WITH_NITS** — B1/B2 verified fixed by manual trace + author-executed table tests; N2–N6 verified. Remaining nits: quoted env-var references (`"${{ secrets.X }}"`) now over-mask (bounded, no leak); bare `token`/`secret` can over-mask identifiers like `tokenizer=`/`secrets=` (documented in pattern comment); off-by-one in self-review counts corrected.
