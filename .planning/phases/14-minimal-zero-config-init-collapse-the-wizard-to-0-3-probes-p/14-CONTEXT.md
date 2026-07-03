# Phase: Minimal zero-config init - Context

**Gathered:** 2026-07-03
**Status:** Ready for implementation (lean phase)
**Note:** numbered 14 on this master-based branch; the curl|bash installer PR #533 also claimed "Phase 14" — renumber to 15 at ship time if #533 is still open.

<domain>
## Phase Boundary

Radically simplify `nano-brain init`. Today's Phase 13 wizard asks ~a dozen consequential questions (DB, port, embedding provider/url/model/concurrency, harvester dirs, session poll, summarization provider/key/model/tokens/concurrency/rps, rrf_k/recency/limit, debounce/reindex, log level/file, …) and writes secrets to the config file. Research across 11 memory/RAG/vector tools (mem0, Zep, Cognee, Letta, Chroma, Qdrant, Ollama, txtai, LlamaIndex, Weaviate, Supermemory) is unanimous: **no interactive wizard, zero-config + sensible defaults, secrets via env vars, advanced tuning in a config file edited later, domain context set later — never prompted.** Letta explicitly REMOVED its `letta configure` wizard (the exact pattern we have). Mirror that.

</domain>

<decisions>
## Locked Decisions (from research + Tâm)

- **D-01 — Target 0 questions, cap 3.** On a machine with reachable Postgres + Ollama, `nano-brain init` asks ZERO questions, writes the full config, prints what it chose (like `ollama run`).
- **D-02 — The only things init may PROBE/ASK (in order, ask only when unresolved):**
  1. **Postgres** — probe default DSN; reachable → 0 questions. Unreachable + Docker present → offer auto-provision (reuse Phase 13 `stepDatabase`/`provisionPostgres`). Neither → prompt DSN or point at `NANO_BRAIN_DATABASE_URL`.
  2. **Embeddings** — probe Ollama at localhost (`detectOllama`). Up → silent default `ollama`/`nomic-embed-text`. Down → one prompt (or point at env), else BM25-only (`provider: ""`).
  3. **Harvester dirs** — auto-detect (`detectOpenCodeStorageDir`/`detectClaudeCodeStorageDir`); write silently if found; never prompt.
- **D-03 — Full self-documenting config (Tâm's core requirement).** init writes a config file containing **every** section at default values **with explanatory comments**; advanced sections present but `enabled: false`. The probed DB URL / embedding block are substituted in; everything else is the commented default. User/LLM edits the file later to enable HyDE/reranking/etc.
- **D-04 — Implementation of the full config = a hand-authored commented template**, NOT `yaml.Marshal(getDefaults())`. Reason (verified): the Config structs carry only `koanf:`/`json:` tags, no `yaml:` tags, so yaml.v3 lowercases field names (`code_summarization`→`codesummarization`, `recency_half_life_days`→`recencyhalflifedays`) and the output does NOT round-trip through `config.Load` (koanf). The template must use correct snake_case keys matching the koanf tags. A round-trip test (write template → `config.Load` → assert equals `getDefaults()` for the defaulted fields) is mandatory to prevent key drift.
- **D-05 — Secrets via env vars, never written to the config file.** Stop prompting for and persisting `VOYAGE_API_KEY`, `NANO_BRAIN_SUMMARIZE_API_KEY`, `NANO_BRAIN_CODE_SUMMARIZE_API_KEY` (all already supported by the `NANO_BRAIN_` env scheme). The commented template documents which env var enables each key-dependent feature. Key-dependent features default OFF, so no key is needed at init.
- **D-06 — `context_hints` NOT prompted.** It is nano-brain's analog of mem0 `custom_instructions` / Ollama `SYSTEM` — set per-workspace later, by hand or by an LLM agent. It lives under `search.hyde.context_hints` (+ query_preprocessing/reranking). The template shows it commented with a one-line "written later per workspace" note.
- **D-07 — Advanced sections = commented defaults only (no prompt):** summarization, code_summarization, search.hyde, search.reranking, search.query_preprocessing, search.entity_boost, search.pagerank, flow, bench, intelligence, storage, telemetry, watcher.exclude_patterns, bm25_language.
- **D-08 — Add `--yes` / non-interactive path** that writes pure defaults (probed DB via env/default, embeddings auto or off) with ZERO prompts — mirrors `ollama run` / `docker run`. Complements the existing `--json`/non-TTY contract.
- **D-09 — Keep the chain-to-running behavior from Phase 13** (doctor → serve → register → MCP client picker) unchanged; only the CONFIG-BUILDING portion is collapsed. The "Advanced settings? [y/N]" gated detailed prompts (Phase 13 `stepAdvanced`) are REMOVED — the commented template replaces the need to prompt them.
- **D-10 — Re-runnable / keep-existing gate stays** (Phase 13 D-03): existing config → keep (default) skips straight to service steps.

</decisions>

<code_context>
## Files (on master, post-#532)

- `cmd/nano-brain/init.go` — `runInteractiveInit`: remove the DB/port/embedding-detail/harvester/summarization/search/watcher/logging prompt sequence + `stepAdvanced`; replace config-build with: probe (stepDatabase from Phase 13 + Ollama detect) → render the commented template with substitutions → write. Keep keep/overwrite gate, doctor, serve, register, MCP steps.
- NEW `internal/config/template.go` (or `config.template.yml` via `//go:embed`) — the hand-authored commented full-config template with `{{.DatabaseURL}}` / `{{.EmbeddingBlock}}` substitution points; a `RenderConfig(opts)` func.
- `internal/config/defaults.go` — `getDefaults()` is the source of truth for default VALUES; the template's defaults must match it (round-trip test enforces).
- `cmd/nano-brain/init_db.go`, `init_embedding.go` — reuse `stepDatabase`, `detectOllama`; `stepEmbedding` simplified (probe → default or one prompt; no provider/concurrency prompts).
- Tests: round-trip (template → Load == defaults), `--yes` writes valid config with 0 prompts, probe-hits-default asks nothing (promptReader/isTTYFn injection), env-var-secret (no key ever written to file).

## Evidence required

- `go test -race -short ./cmd/nano-brain/... ./internal/config/...`
- Live: `nano-brain init` on this machine (Postgres+Ollama up) asks 0 config questions and writes a full commented config that `config.Load` accepts and `doctor` passes.
- Round-trip test proves no koanf/yaml key drift.

</code_context>

<deferred>
## Deferred

- OpenAI-compatible embedding provider (REQ-INFRA-01)
- Auto-writing context_hints from repo analysis (an LLM agent could, later — not init's job)
- Windows daemon serve (unchanged; still prints manual instruction)

</deferred>
