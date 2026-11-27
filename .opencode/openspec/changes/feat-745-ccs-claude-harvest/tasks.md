# Tasks (#745)

- [x] Add failing focused tests for CCS root derivation (`CCS_DIR`, `$CCS_HOME/.ccs`, default root, union ordering, dedupe, missing/relative/custom roots) and stable pattern detection.
- [x] Implement CCS-aware detection/pattern helpers in `cmd/nano-brain/detect.go`; move auto-detection before server config is copied and populate stable patterns.
- [x] Add failing tests for dynamic first-session, new-profile, and newly-registered-workspace discovery, plus transcript-`cwd` mismatch protection.
- [x] Implement dynamic harvest-cycle Claude discovery and bounded cwd validation while preserving legacy parsing/dedup/summarizer/raw-fallback behavior.
- [x] Add failing config-render/init tests for conditional `claudecode.enabled`, empty directory fields, commented `session_dirs` examples, visible notice, and no existing-config rewrite.
- [x] Implement template/render/init changes and update round-trip coverage.
- [x] Update `docs/CONFIGURATION.md`, `docs/SETUP_AGENT.md`, `docs/ROADMAP.md`, and generated config comments consistently.
- [x] Run focused tests, `go test -race -short ./...`, build/lint validation, and relevant isolated `nanobrain_test` integration tests.
- [x] Run fixture-based smoke evidence without touching the dev DB or port 3100; write evidence under `docs/evidence/`.
- [x] Self-review the diff, obtain independent review distinct from implementation, fix blockers, and document any deferrals.
- [ ] Commit with repo attribution rules, push the existing feature branch, open/refresh the PR, and verify CI.
