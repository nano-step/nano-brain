# Review #730: MCP trace symbol lookup

Reviewer: two independent read-only code reviewers (separate from implementer)
Review Verdict: PASS

## Independent findings

Two independent read-only reviewers inspected the final implementation,
synthetic test and migration. Both returned MERGE for code correctness.
Neither reviewer had shell access; the orchestrator independently ran the
build, tests, formatting, SQL measurement and privacy scan below.

- Cache is local to each MCP call; both hits and misses are cached, but
  errors are not. Full match lists remain available for nearest-definition
  selection by each caller. Cycles, BFS order and path handling stay intact.
- Migration 00032 builds the partial index concurrently, outside a
  transaction. SQL predicate and lookup keys match `ResolveSymbolByName`.
- The verifier compares indexed/unindexed plans on one isolated test
  schema; a separate same-index MCP baseline/candidate comparison
  isolates the request-local cache effect on a synthetic fixture.
- On that fixture, MCP p50 changed from 75–84ms without the cache to
  31–34ms with the cache (two non-race runs per version). The index's
  measured SQL effect is separate; no combined production speedup is
  claimed.
- Residual bare-node graph scans remain. No claim that the change solves
  every high-depth trace or that the three replay samples define an SLA.

## Orchestrator verification

- `go build ./...`: PASS.
- `go test -race -short ./...`: PASS (exit 0).
- `go test -race -tags=integration ./...` with `nanobrain_test`: PASS.
- `TestMemoryTrace_ResolvesEachBareNameOncePerRequest`: PASS; 4 calls
  before cache vs 2 after per request in the implementer's red/green probe.
- `TestTraceLatency730Evidence`: PASS on isolated `nanobrain_test`.
- `TestTraceLatency730Evidence` also exercised the Streamable HTTP MCP
  handler: HTTP/1.1 200 OK with 42 synthetic trace results.
- `gofmt -l` on changed Go files and `git diff --check`: clean.
- Pending-file privacy scan: no private workspace names, local user paths
  or 64-hex workspace hashes; issue #730 title/body/comments also scanned.

## Process gate

Phase 13 was already marked complete in `.planning/ROADMAP.md` and its
verification report passed all automated criteria. Its stale `verifying`
frontmatter was reconciled before running pre-work (6 PASS, 0 FAIL,
1 SKIP). A new Phase 20 was recorded for #730; in-progress gate:
4 PASS, 0 FAIL, 0 SKIP. The new state/docs must receive independent
review alongside the final performance evidence before landing.
