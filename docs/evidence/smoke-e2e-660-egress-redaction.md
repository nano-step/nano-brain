# Smoke E2E — #660 Egress-Only Credential Redaction

**Date:** 2026-11-22 · **Branch:** `feat/660-egress-redaction` · **Change type:** security-hardening

## What was verified (wire level)

Two boundary tests run real HTTP through `httptest` servers and inspect the
**actual request body the provider would receive**:

- `internal/embed`: `TestOllamaEmbedder_Embed_RedactsSecrets` —
  `sk-ant-api03-...` + `ghp_...` sent to a fake Ollama `/api/embed`;
  `TestVoyageAIEmbedder_Embed_RedactsSecrets` — `AKIA...` sent to a fake
  Voyage endpoint; each handler asserts the secret is absent and
  `[REDACTED:` is present.
- `internal/summarize`: `TestChatCompletion_RedactsSecrets` — user prompt
  containing an `sk-ant-` key is sent through the shared OpenAI-compatible
  client (covers session summarize, `flow`, `intelligence` callers); body
  asserted secret-free with marker.

## Results

```
$ go test ./internal/redact/ -v        # 28 secret shapes + 15 false-positive guards — all PASS
$ go test ./internal/embed/ -run Redact -v
--- PASS: TestOllamaEmbedder_Embed_RedactsSecrets (0.00s)
--- PASS: TestVoyageAIEmbedder_Embed_RedactsSecrets (0.00s)
$ go test ./internal/summarize/ -run Redact -v
--- PASS: TestChatCompletion_RedactsSecrets (0.00s)

$ go test ./internal/embed ./internal/summarize ./internal/codesummarize \
    ./internal/search/... ./internal/bench ./internal/redact \
    ./internal/flow ./internal/intelligence
ok  internal/embed            ok  internal/summarize       ok  internal/codesummarize
ok  internal/search           ok  internal/search/hyde     ok  internal/search/preprocess
ok  internal/search/reranking ok  internal/bench           ok  internal/redact
ok  internal/flow             ok  internal/intelligence
```

## Egress-only invariant check

```
$ grep -rn '"github.com/nano-brain/nano-brain/internal/redact"' --include='*.go' internal/ cmd/ | grep -v _test
internal/bench/generate.go            internal/embed/ollama.go
internal/embed/voyageai.go            internal/codesummarize/provider.go
internal/summarize/client.go          internal/search/reranking/reranker.go
internal/search/hyde/generator.go     internal/search/preprocess/preprocessor.go
```

No `redact` usage in `internal/storage`, harvesters, or document/chunk write
paths → stored content is byte-identical; only provider-bound payloads are
masked.
