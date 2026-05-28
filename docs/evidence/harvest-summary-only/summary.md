# Section 8: User-Flow Validation — Summary

**Change**: harvest-summary-only
**Branch**: feat/harvest-summary-only
**Issue**: #189
**Date**: 2026-05-28
**Validation Environment**: Container agent + host PostgreSQL + host Ollama + ai-proxy LLM

## Infrastructure

- **Binary**: Built from feature branch at `/tmp/nano-brain-section8/nano-brain` (51MB, CGO_ENABLED=0)
- **Test Database**: `nanobrain_harvest_validation` on `host.docker.internal:5432` (isolated, disposable)
- **Test Config**: `/tmp/nano-brain-section8/config.yml` (never touched user's `~/.nano-brain/config.yml`)
- **Synthetic SQLite**: `/tmp/nano-brain-section8/test-opencode.db` with 3 test sessions
- **Server Port**: 3199 (non-default, avoids conflicts)
- **Embedding**: Ollama (nomic-embed-text) on host
- **LLM**: ai-proxy (nano-brain model) for summarization

## Task Results

| Task | Description | Verdict | Evidence File |
|------|-------------|---------|---------------|
| 8.1  | Start server, trigger harvest | PASS | 8.1-server-start-harvest.log |
| 8.2  | Collections breakdown (session-summary > 0) | PASS | 8.2-collections-breakdown.txt |
| 8.3  | No new docs in sessions collection | PASS | 8.3-no-stale-sessions.txt |
| 8.4  | Query returns summary docs | PASS | 8.4-query-results.json |
| 8.5  | workspace_hash correct (not empty) | PASS | 8.5-workspace-hash-correct.txt |
| 8.6  | Structured log line (M3 counters) | PASS | 8.6-structured-log.txt |
| 8.7  | LLM failure -> fallback (B1 unified path) | PASS | 8.7-llm-failure-fallback.txt |
| 8.8  | Fallback persistence (B2 skip-check) | PASS | 8.8-fallback-skip.txt |
| 8.9  | LLM recovery (fallback NOT auto-replaced) | PASS | 8.9-recovery-not-replaced.txt |
| 8.10 | Backward-compat config (output_dir in YAML) | PASS | 8.10-backward-compat-output-dir.txt |
| 8.11 | Disabled summarization fallback (B1) | PASS | 8.11-disabled-summarization.txt |
| 8.12 | Evidence capture | PASS | (this file) |

**Overall: 12/12 PASS**

## Key Findings

### Happy Path (8.1-8.6)
- LLM summarization works end-to-end: sessions → LLM → structured summary → `session-summary` collection
- workspace_hash correctly threaded from registration through to summary docs
- Structured log emits exactly one line per harvest cycle with all M3 counters
- Second harvest correctly skips all docs (content_hash idempotency)

### Error/Edge Path (8.7-8.11)
- LLM failure triggers graceful fallback to raw content at unified `summary://opencode/<id>` path
- Fallback docs use `collection="sessions"` and `metadata.fallback=true` (B1 + M2)
- Fallback docs persist across re-harvests (B2 skip-check works)
- LLM recovery does NOT auto-replace fallback docs (B2 known trade-off)
- Disabled summarization produces same unified path with fallback markers
- Backward-compat: `output_dir` in YAML is parsed without error, no files written

### Bug Discovered (Pre-existing, NOT in this change)
When `embedding.provider: ""` (embedding disabled), the embed queue is nil, but passed to
`Persister` as a non-nil interface wrapping a nil `*embed.Queue` pointer. The nil check
`p.enqueuer != nil` at persist.go:116 passes, causing a nil pointer dereference at
queue.go:84. **Workaround**: Used Ollama embedding for all tests. **Recommendation**: File
a separate issue to fix the nil-interface trap in `NewPersister` or `main.go`.

## Cleanup

- Test database `nanobrain_harvest_validation` should be dropped after review
- Test artifacts at `/tmp/nano-brain-section8/` can be deleted
- No production code was modified
- No production database was touched
