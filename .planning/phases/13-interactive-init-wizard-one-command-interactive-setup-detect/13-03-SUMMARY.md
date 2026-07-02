---
phase: 13-interactive-init-wizard-one-command-interactive-setup-detect
plan: 03
subsystem: cli
tags: [wizard, postgres, pgx, docker, prompt]

# Dependency graph
requires:
  - "13-01: dockerStatus(ctx) classification + provisionPostgres(ctx) (consumed via provisionPostgresFn seam)"
provides:
  - "stepDatabase(scanner, defaultURL) (dbURL string, ok bool) — the D-05 database wizard step: ping default URL → zero prompts on success; else dockerStatus branches to Docker-provision prompt or remote-URL prompt"
  - "waitForPostgresReady(ctx, dbURL, timeout, interval) — pgx connect+ping poll loop (D-08), mirrors waitForServerHealthy's deadline/clamp structure"
  - "promptRemoteURL(scanner) — D-09 live-validated URL entry with deliberate-blank save-anyway escape hatch; EOF is a decline (CR-01); postgres://-scheme gate (T-13-05 V5)"
  - "connectPostgresFn / provisionPostgresFn injectable seams so tests run without a real DB or Docker"
affects: [13-07 (orchestrator wires stepDatabase into runInteractiveInit and writes database.url)]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Function-typed package vars as test seams (connectPostgresFn, provisionPostgresFn), swapped in tests with save/restore — same idiom as runDocker in 13-01"
    - "Credential hygiene: only hostOnly(dbURL) (url.Parse .Host) is ever printed; pgx's own ParseConfigError/ConnectError redaction covers %v on connect errors (verified against pgx v5 source in review)"

key-files:
  created:
    - cmd/nano-brain/init_db.go
    - cmd/nano-brain/init_db_test.go
  modified: []

key-decisions:
  - "Readiness poll starts only after docker run returns (image pull time excluded from the 30s/500ms budget) — per RESEARCH.md resolved Q2"
  - "dockerStatusUnknownError groups with the not-available branch (fail closed to remote-URL prompt)"
  - "Independent pre-commit review (R88) found and fixed before commit: (1) MAJOR — EOF after a failed URL entry was treated as the save-anyway escape hatch, violating CR-01 (EOF-means-decline); now !promptOK always returns ok=false. (2) MEDIUM — threat-model mitigation V5 (postgres:///postgresql:// scheme gate before pgx.Connect) was missing; now enforced with re-prompt. Both covered by new tests TestStepDatabase_EOFAfterFailedEntry_DeclinesNotSaves and TestStepDatabase_NonPostgresScheme_RejectedThenValidAccepted."
  - "Committed as a single feat commit (test+impl) per repo pre-commit whole-suite gate, same precedent as 13-01 / Phase 999.1-01"

## Self-Check

- go test -race -short -count=1 ./cmd/nano-brain/ → ok (5.7s), including 2 new review-driven tests
- go vet clean (run during independent review)
- No DSN/password appears in any printed output (hostOnly only)
- Commit 8d02ba0 on worktree-agent-ae6a2a3d84f8fe391

Self-Check: PASSED
