## 0. Age Gate — Active Session Detection

- [ ] 0.1 Add `UpdatedAt time.Time` field to `SqSession` struct in `opencode_sqlite.go`
- [ ] 0.2 Update `listSessions()` SQL query to include `COALESCE(s.time_updated, s.time_created, 0)` column; scan into `UpdatedAt`
- [ ] 0.3 Add `isActiveSession(sess SqSession) bool` helper: returns `time.Since(sess.UpdatedAt) < 10*time.Minute`
- [ ] 0.4 In `HarvestAll()` loop, before skip-check: if `isActiveSession(sess)` → `skipped++; continue`
- [ ] 0.5 Add test: session updated 5min ago → skipped; session updated 15min ago → processed

## 1. Thread WorkspaceHash into SummaryMeta

- [ ] 1.1 Add `WorkspaceHash string` field to `SummaryMeta` in `internal/harvest/harvest.go`
- [ ] 1.2 Add `WorkspaceHash string` field to `SessionMetadata` in `internal/summarize/pipeline.go`
- [ ] 1.3 Update `HarvestSummarizer.SummarizeAndPersist` in `internal/summarize/harvest_adapter.go` — copy `meta.WorkspaceHash` into `sessionMeta.WorkspaceHash`
- [ ] 1.4 In `Persister.Save`, replace ALL 3 occurrences of `p.workspace` with `meta.WorkspaceHash`:
  - Line 62: idempotency lookup (`GetDocumentBySourcePath`)
  - Line 94: document upsert (`UpsertDocument`)
  - Line 112: chunk upsert (workspace param)
- [ ] 1.5 Remove `p.workspace` field from `Persister` struct — grep for any other callers first (`grep -rn "p\.workspace" internal/summarize/`)
- [ ] 1.6 Update `NewPersister` call in `main.go` — remove workspace arg (or pass empty, ignore) after field removal

## 2. Remove file write from Persister

- [ ] 2.1 Remove `writeFile` call from `Persister.Save` in `internal/summarize/persist.go`
- [ ] 2.2 Delete `writeFile` method
- [ ] 2.3 `NewPersister` no longer needs to `os.MkdirAll` — remove that call
- [ ] 2.4 Update `NewPersister` signature: remove `outputDir` param. Update all callers (main.go + tests).
- [ ] 2.5 Remove `OutputDir` field from `SummarizationConfig` struct in `internal/config/config.go` (M4: keep YAML key parseable, just no longer mapped)
- [ ] 2.6 Remove the "summary output directory" prompt from `nano-brain init` interactive flow (`cmd/nano-brain/init.go`)
- [ ] 2.7 Update tests in `persist_test.go` to not assert file writes

## 3. Rewire opencode_sqlite.go — summary-first harvest (UNIFIED source_path)

- [ ] 3.1 Change skip-check in `HarvestAll`: ALWAYS check `summary://opencode/<id>` regardless of summarizer state (B1: unified path)
- [ ] 3.2 After `listMessages` + `renderSQLiteMarkdown` (in-memory), branch on summarizer state:
  - **summarizer != nil**: call `summarizer.SummarizeAndPersist(ctx, md, SummaryMeta{..., WorkspaceHash: wsHash})`
    - On success: `summary_success++`, continue to next session (no raw UpsertDocument)
    - On summarizer error: log WARN, attempt raw fallback (see below), `summary_fallback++`
    - On DB upsert error inside `Persister.Save`: log WARN, attempt raw fallback (see below), `summary_fallback++` (M6)
  - **summarizer == nil**: write raw fallback directly (see below), `summary_fallback++`
  - **Raw fallback writer**: `UpsertDocument(raw, sourcePath="summary://opencode/<id>", collection="sessions", metadata={"fallback": true})` (B1 unified + M2 fallback marker)
  - **If raw fallback ALSO errors**: log ERR, `errors++`, skip session (no infinite retry)
- [ ] 3.3 Remove the post-commit `SummarizeAndPersist` call (currently at bottom of `HarvestAll` loop)
- [ ] 3.4 Increment harvest-cycle counters per-session: `summary_success`, `summary_fallback`, `skipped`, `active`, `errors`
- [ ] 3.5 At end of `HarvestAll`, emit one structured INFO log: `harvest cycle complete source=opencode summary_success=N summary_fallback=N skipped=N active=N errors=N` (M3)
- [ ] 3.6 Add test: LLM fails → fallback doc path is `summary://opencode/<id>` with `metadata.fallback=true`; next harvest cycle skips it (matches existing doc by content_hash)

## 4. Rewire claudecode.go — summary-first harvest

- [ ] 4.1 Inspect `internal/harvest/claudecode.go` — identify where raw UpsertDocument and summarizer call happen
- [ ] 4.2 Apply the SAME pattern as opencode (task 3.2): unified `summary://claudecode/<filename>` path, summarize-first when summarizer != nil, raw fallback with `metadata.fallback=true` and `collection="sessions"` when summarizer is nil OR errors OR DB upsert fails
- [ ] 4.3 Pass `WorkspaceHash` in `SummaryMeta` (claudecode harvester must have wsHash available — derived from `cfg.Harvester.ClaudeCode.SessionDir`)
- [ ] 4.4 Increment same per-cycle counters as opencode; emit `harvest cycle complete source=claudecode ...` INFO log at end (M3)

## 5. Fix init order + graceful degradation in main.go

- [ ] 5.1 Move `HarvestSummarizer` init block **before** `NewOpenCodeSQLiteHarvester` call (currently at line ~374, must be before ~327)
- [ ] 5.2 Wrap LLM client + pipeline init in error check: if any step fails → log warn, set `harvestSummarizer = nil` (do NOT fatal)
- [ ] 5.3 Pass summarizer into harvester via `WithSummarizer(harvestSummarizer)` at construction time
- [ ] 5.4 Remove `hr.WithSummarizer(harvestSummarizer)` post-init call (keep `srv.SetSummarizer` for HTTP endpoint)
- [ ] 5.5 Same for claudecode harvester if it has `WithSummarizer`

## 6. Tests

- [ ] 6.1 `opencode_sqlite_test.go`: age gate — session `time_updated` < 10min → skipped (`active` counter incremented)
- [ ] 6.2 `opencode_sqlite_test.go`: summarizer set + happy path → exactly 1 doc per session at `summary://opencode/<id>` with `collection="session-summary"`; NO doc at `opencode://session/<id>`
- [ ] 6.3 `opencode_sqlite_test.go`: summarizer returns error → **fallback raw doc at `summary://opencode/<id>` (UNIFIED PATH per B1) with `collection="sessions"` and `metadata.fallback=true`**
- [ ] 6.4 `opencode_sqlite_test.go`: skip-check uses `summary://opencode/<id>` regardless of summarizer state
- [ ] 6.5 `opencode_sqlite_test.go` (M6): mock `Persister.Save` to return DB error → falls back to raw upsert at `summary://opencode/<id>` with `metadata.fallback=true`; harvest continues; `summary_fallback++`
- [ ] 6.6 `opencode_sqlite_test.go` (M3): multi-session run (3 success + 2 fallback + 1 skip) → exactly one INFO log emitted with `summary_success=3 summary_fallback=2 skipped=1 active=0 errors=0`
- [ ] 6.7 `persist_test.go`: verify `meta.WorkspaceHash` used (not empty string); no file write assertions; `~/.nano-brain/summaries/` not created
- [ ] 6.8 `harvest_adapter_test.go` (new or existing): WorkspaceHash threads from `SummaryMeta` → `SessionMetadata`
- [ ] 6.9 (M5) `internal/harvest/opencode_sqlite_integration_test.go` (new, build tag `//go:build integration`): real Postgres + mock LLM → harvest 1 session → assert summary doc exists with correct `workspace_hash`, correct `source_path`, correct `collection`
- [ ] 6.10 (M4) `internal/config/config_test.go`: existing YAML with `summarization.output_dir: /tmp/foo` parses without error and `cfg.Summarization` struct does NOT expose the field
- [ ] 6.11 Run `CGO_ENABLED=0 go build ./... && go test -race -short ./...` — all pass
- [ ] 6.12 Run `go test -race -tags=integration ./...` — integration test from 6.9 passes

## 7. Post-Implementation Cleanup (Follow-up, Out of Scope)

- [ ] 7.0 (FOLLOW-UP) Create GitHub issue: stale raw doc cleanup — delete `opencode://session/<id>` docs from `sessions` collection where matching `summary://opencode/<id>` exists in `session-summary`
- [ ] 7.0b (FOLLOW-UP) Create GitHub issue: increase `max_tokens` from 4800 → 8000 in default config (ai-proxy supports Sonnet 4.6 with 200K output capacity)

## 8. Validation (user-flow tests for HIGH-RISK lane)

### Primary path

- [ ] 8.1 Start server with `summarization.enabled: true`, trigger harvest via `POST /api/harvest`
- [ ] 8.2 Verify: `GET /api/collections` shows `session-summary` with docs > 0
- [ ] 8.3 Verify: no NEW docs in `sessions` collection after harvest (unless fallback fired) — existing pre-change raw docs are grandfathered
- [ ] 8.4 Query: `POST /api/query {"query": "nano-brain harvest fix", "workspace": "<worktree>"}` — returns summary doc
- [ ] 8.5 Verify workspace_hash correct: summary doc workspace matches project worktree hash (NOT empty string)
- [ ] 8.6 Verify structured log line: `harvest cycle complete source=opencode summary_success=N summary_fallback=N skipped=N active=N errors=N` appears exactly once per harvest cycle (M3)

### Edge/error path

- [ ] 8.7 Simulate LLM failure: set invalid `provider_url`, trigger harvest → verify fallback doc at `summary://opencode/<id>` with `collection="sessions"` and `metadata.fallback=true` (B1 unified path); `summary_fallback` counter incremented in log
- [ ] 8.8 **Fallback persistence test (B2)**: With LLM still failing, trigger harvest again → verify the fallback doc from 8.7 is SKIPPED (content_hash unchanged) — `skipped` counter incremented, no LLM call attempted, no duplicate doc created
- [ ] 8.9 **LLM recovery test (B2)**: Re-enable LLM, re-trigger harvest → verify existing fallback docs from 8.7 REMAIN at `collection="sessions"` with `metadata.fallback=true` (NOT auto-replaced); only NEW sessions get proper summaries. Document this in user-facing release notes as a known trade-off; manual recovery procedure: `DELETE FROM documents WHERE source_path='summary://opencode/<id>' AND collection='sessions'` then re-harvest
- [ ] 8.10 **Backward-compat config (M4)**: Start server with existing user config containing `summarization.output_dir: ~/.nano-brain/summaries` → server starts cleanly, no error, no `.md` files written, log shows `summarization config loaded`
- [ ] 8.11 **Disabled summarization fallback (B1)**: Set `summarization.enabled: false`, trigger harvest → all sessions written under `summary://opencode/<id>` with `collection="sessions"` and `metadata.fallback=true` (same unified path); flipping back to `enabled: true` later does NOT cause duplicate docs (skip-check matches)
- [ ] 8.12 Capture evidence artifacts to `docs/evidence/harvest-summary-only/`: screenshots of log output, query results before/after, `psql` output showing collection breakdown
