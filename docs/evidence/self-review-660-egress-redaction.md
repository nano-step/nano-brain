# Self-Review — #660 Egress-Only Credential Redaction

**Change type:** security-hardening · **Lane:** standard · **Author pass:** complete

## Scope reviewed

| File | Change |
|------|--------|
| `internal/redact/redact.go` | NEW — `Scrub(text)` applying 9 ordered credential patterns → `[REDACTED:<kind>]` |
| `internal/redact/redact_test.go` | NEW — 28 positive shapes, 15 false-positive guards, multi-secret case |
| `internal/embed/ollama.go`, `voyageai.go` | `text = redact.Scrub(text)` at top of `Embed()` |
| `internal/summarize/client.go` | Scrub `systemPrompt`/`userPrompt` in `ChatCompletion` — every LLM routed through this client is covered (session summarize pipeline, `flow` summarizer today; `intelligence` consolidator/categorizer has no production callers yet but satisfies the same interface) |
| `internal/codesummarize/provider.go` | Scrub batch prompt in `SummarizeBatch` |
| `internal/search/reranking/reranker.go` | Scrub `query` + every `documents[i]` in `callAPI` (Cohere egress) |
| `internal/search/hyde/generator.go`, `preprocess/preprocessor.go` | Scrub outbound query |
| `internal/bench/generate.go` | Scrub `content` in `generateLLMQuery` |
| `internal/embed/embed_test.go`, `internal/summarize/client_test.go` | Wire-level boundary tests (Ollama + Voyage + ChatCompletion): httptest server asserts secret absent + marker present in actual request body |
| `docs/CONFIGURATION.md`, `internal/embed/AGENTS.md` | Document the egress-only policy |

## Correctness findings (self)

1. **Pattern order vs marker re-match:** `secret-assignment` runs last but its value charset initially matched `[REDACTED:x]` markers and overwrote them — fixed by excluding `[` from the value class (`[^\s"',;$}\[]`).
2. **Env-style prefixed keys:** `\b(password|api_key)` missed `DB_PASSWORD` / `OPENAI_API_KEY` (`_` is a word char) — fixed with `(\w*(names)\w*)` capture, preserving the key name (`KEY=[REDACTED]`).
3. **PEM blocks:** `[^-]*-END` could never reach `-----END` — fixed with `[\s\S]*?-----END`.
4. **Egress coverage verified by call-graph:** all stored-content → provider paths covered — embedders (2), `summarize.Client` (covers `summarize/pipeline`, `flow`, `intelligence`), `codesummarize.LLMProvider`, Cohere reranker, bench generator. Query-side egress (HyDE, preprocessor) also scrubbed — a credential pasted into a query leaves as `[REDACTED]` too.
5. **Egress-only semantics preserved:** storage writes (`documents`, `chunks`, sessions) untouched; `memory_get`/BM25/vector retrieval operate on original bytes. Verified: no `redact` import in `internal/storage`, harvesters, or write paths — `grep -rn "redact" internal/` shows imports only in provider-boundary files.

## False-positive risk assessment

- Assignment pattern requires ≥8-char non-space value — `EDITOR=vim`, `memory_limit=512mb`, `password=short` unaffected (test-covered).
- Embedding-side impact: chunks containing credentials embed the masked form (rare; desired trade-off — a remote provider never receives the raw secret).
- Known gap (accepted): random high-entropy secrets with no recognizable shape (e.g. bare 40-hex strings) are not redacted — shapeless secrets are unredactable without unacceptable false positives on hashes/UUIDs. Documented as residual risk.

## Independent-review findings resolved (review-660, round 1 → SEND_BACK → fixed)

- **B1 (blocking)** — JSON `"key": "value"` pairs leaked: added optional closing
  `["']?` after the key capture and quoted-value branches
  (`"..."`/`'...'` ≥8 chars incl. spaces) to the assignment pattern.
  Table tests: `json api_key pair`, `json password pair`, `js camelCase key`,
  `quoted json token value`.
- **B2 (blocking)** — `*_TOKEN`/`*_SECRET` families missed (`JWT_SECRET`,
  `GITHUB_TOKEN`, `GITLAB_TOKEN`, `SESSION_SECRET`): added bare `token`,
  `secret`, `api_token` to the alternation; extended shapes with `glpat-`,
  `hf_`, `xapp-`, `sk/rk/whsec_(live|test)_`.
- **N2** — quoted multi-word secrets (`password="my secret pass"`) now matched
  via quoted-value branches.
- **N3** — dropped redundant `MatchString` pre-check.
- **N4** — added `TestVoyageAIEmbedder_Embed_RedactsSecrets` boundary test.
- **N5/N6** — evidence counts and the intelligence coverage claim corrected
  above.

## Checks run

- `go build ./...` PASS
- `go test ./internal/redact ./internal/embed ./internal/summarize ./internal/codesummarize ./internal/search/... ./internal/bench ./internal/flow ./internal/intelligence` — all PASS
- `gofmt -l` clean on touched files

No unresolved critical/major findings.
