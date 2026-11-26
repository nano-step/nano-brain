# Tasks — codex-session-harvesting

## 1. Config

- [x] 1.1 `CodexHarvesterConfig{Enabled bool, CodexHome string}` in `internal/config/config.go`; add `Codex CodexHarvesterConfig` to `HarvesterConfig`.
- [x] 1.2 Defaults in `internal/config/defaults.go`: `enabled: false`, `codex_home: ""`.
- [x] 1.3 `codex:` block in `internal/config/template.go` next to `pi:`; `TestRenderConfig_RoundTrip` passes.

## 2. Source plumbing

- [x] 2.1 `SourceCodex` const + `buildSourcePath` case in `internal/summarize/persist.go`.
- [x] 2.2 `pipeline.go` Source switch: `SourceCodex` → `StripClaude`.
- [x] 2.3 `sourceFromTags` (handlers/summarize.go), `extractBackfillSource` (cmd_backfill_summaries.go), `SourceFromPath` (storage/sourcepath.go) gain `"codex"`.

## 3. Harvester

- [x] 3.1 `internal/harvest/codex.go`: `CodexHarvester` + constructor, `HarvestAll`, `WithSummarizer`/`summarizerSettable`, per-source log line `Str("source","codex")`.
- [x] 3.2 Session-meta scan (≤256 lines), workspace attribution (exact → longest-ancestor → hash-check → skip), rollout renderer per design.md Decision 4, presence dedup, `persistRawFallback`, `SummaryMeta{Source: SourceCodex}`.
- [x] 3.3 `codex_test.go` unit tests per design.md Testing.

## 4. Wiring

- [x] 4.1 `cmd/nano-brain/codex_init.go`: `detectCodexHome` + `initCodexHarvesters`; `codex_init_test.go`.
- [x] 4.2 Wire into `main.go` harvester section.
- [x] 4.3 `internal/harvest/AGENTS.md` file table + section; `docs/CONFIGURATION.md` `harvester.codex.*`; `CHANGELOG.md` Unreleased.

## 5. Verify

- [x] 5.1 `gofmt -l` clean; `go build ./...`; `go vet ./internal/harvest ./cmd/nano-brain`.
- [x] 5.2 `go test -race -short ./...`.
- [x] 5.3 `go test -race -tags=integration ./internal/harvest/... ./internal/summarize/... ./internal/config/... ./internal/server/handlers/... ./internal/storage/... ./cmd/nano-brain/...` on `nanobrain_test`.
- [x] 5.4 Privacy grep (no real workspace names/paths) before commit.
