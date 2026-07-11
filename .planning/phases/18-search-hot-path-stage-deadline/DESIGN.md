# Phase 18 — Design: agent-supplied query expansion (provider-free HyDE) + fix HyDE latency footgun

Tracking: #588 (split from #543 D2). Lane: **high-risk** (search-quality hard gate). Change-type: **user-feature + bug-fix**.

## Journey (why this design, not the first one)

1. Original triage: "search has no timeout" → **wrong**. HyDE/rerank *are* bounded by `httpClient.Timeout = max_latency_ms`; default 500ms is sane (`config/defaults.go:73`).
2. The live 56s (`query_ms: 56032`) came from dev config `hyde.max_latency_ms: 120000` (a footgun), not missing code.
3. First fix idea (tight 2.5s per-stage cap) — **rejected by user**: HyDE latency is provider-dependent; a hard 3s cap just guarantees HyDE times out → wasted LLM call + degraded result. Keep it config-driven.
4. User's better idea: **use the calling agent's own model** (OpenCode/Claude/Codex) instead of a separately-configured server-side LLM.
5. MCP **sampling** (server→client `createMessage`) is the textbook mechanism and the go-sdk supports it server-side (`ServerSession.CreateMessage`, server.go:957) — but **blocked today**: nano-brain runs `StreamableHTTPOptions{Stateless: true}`, and the SDK rejects all server→client requests in stateless mode (streamable.go:58); plus client support (Claude Code et al.) is absent/unverified. Deferred to roadmap.
6. **This phase = the viable realization of the idea:** let the agent *push* an expansion in, rather than the server *pull* via sampling.

## Decisions (locked)

- **D1 — add optional `hypothetical` param to `memory_query`** (MCP tool, `internal/mcp/tools.go:329`). When the agent supplies it, nano-brain embeds THAT text for the vector leg; BM25 still uses the raw `query` (keyword matching wants real terms, not a prose hypothetical). This is server-side HyDE without a server-side LLM — the agent used its own model.
- **D2 — additive, not a replacement.** Existing server-side HyDE stays exactly as-is (config-driven, unchanged) for the no-param case. `hypothetical` simply *overrides* it when present: if `hypothetical != ""`, skip `s.hydeGenerator.Generate` entirely and use the supplied text. No behavior change for existing callers.
- **D3 — plumb via one new param on `HybridSearch`.** Add `hypothetical string` (last positional param). Production callers: `mcp/tools.go:430`, `server/handlers/query.go:82`, `bench/run.go:25`. Test callers pass `""`. Inside the vector leg (service.go:361-372): `if hypothetical != "" { embedQuery = hypothetical } else if hydeEnabled { …Generate… }`.
- **D4 — tool description teaches agents when to use it.** Add to the `memory_query` description: agents MAY pass `hypothetical` = a short ideal-answer paragraph they generate themselves to improve vector recall on conceptual queries; omit for exact/keyword lookups. Keep it opt-in and cheap-by-default.
- **D5 — fix the committed footgun.** `config.test.yml` HyDE `max_latency_ms` 120000 → 3000. Note in the PR that operators should lower any high value in their own (uncommitted) `~/.nano-brain/config.yml`.
- **D6 — no tight per-stage cap.** Explicitly NOT adding `stage_timeout_ms`. HyDE/rerank budgets stay operator-owned via `max_latency_ms` (reverses the first design).

## Non-goals

- MCP sampling / dropping stateless transport → **roadmap** (needs transport rearchitecture + client support). Add a roadmap note referencing SDK TODO #148.
- Query-embedding cache — already shipped (Phase 17 F4).
- Caching agent-supplied hypotheticals — unnecessary; the agent controls repetition.
- Other #543 friction points (trace #576 done, debugging buckets, ticket-recall pollution).

## Acceptance criteria

- **AC-1:** `memory_query` accepts an optional `hypothetical` string; when present, the vector leg embeds it (not the raw query, not server-side HyDE). Unit test asserts `embedQueryCached` is called with the supplied text and `hydeGenerator.Generate` is NOT called.
- **AC-2:** BM25 leg still uses the raw `query` when `hypothetical` is set (unit test).
- **AC-3:** Omitting `hypothetical` = byte-identical behavior to today (server-side HyDE path unchanged; existing search tests green).
- **AC-4:** `HybridSearch` signature change compiles; all callers updated; `go build ./...` clean.
- **AC-5:** `config.test.yml` HyDE `max_latency_ms` ≤ 3000.
- **AC-6:** Tool description documents `hypothetical`.

## Test plan

- **Unit** (`internal/search/service_test.go`): fake embedder recording the text it was asked to embed + fake HyDE generator with a call counter. Case A: `hypothetical` set → embedder sees it, HyDE counter == 0, BM25 sees raw query. Case B: `hypothetical` empty + hyde enabled → HyDE counter == 1 (no regression).
- **Integration** (`nanobrain_test`/:3199, PID-scoped kill, never dev DB/:3100): seed a few docs; call `memory_query` over MCP/HTTP with a `hypothetical` and assert non-empty results and that no HyDE provider is contacted (point hyde.provider_url at an unreachable host to prove it's never called when `hypothetical` is supplied).
- **smoke:e2e** (gate 3.12, user-feature+bug-fix): `curl -i` a `/api/v1/query` with the expansion field over :3199; capture `curl` + `HTTP/` lines + result into `docs/evidence/smoke-e2e-search-hot-path-stage-deadline.md`.
- **Ladder:** `go build ./... && go test -race -short ./...` → `-tags=integration` → smoke:e2e → independent reviewer (R88) → ship. Note: REST `/api/v1/query` (`server/handlers/query.go`) should get the same optional field for parity (self-review:response-shape).
