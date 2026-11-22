# Tasks

- [x] Write red regression tests on `gotreesitter` v0.19.1 across
      `internal/symbol`, `internal/graph`, and `internal/chunker` (`#703`).
- [x] Bump `github.com/odvcencio/gotreesitter` to `v0.57.0` and run
      `go mod tidy`; verify compatibility across `internal/tsparse`,
      `InjectionParser`, and all language extractors.
- [x] Add `tsparse.NewInjectionParser()` bounded by `DefaultTimeout` and
      switch `symbol.VueExtractor` and `graph.VueSFCExtractor` to per-call
      thread-safe `InjectionParser` instances with concurrent `-race` tests.
- [x] Verify all `#703` regression tests turn green (including
      `ExtractEdgesWithImportContext` coverage and exact 500-symbol count on
      5000-line `.js`).
- [x] Document the fix and safe `POST /api/v1/update` backfill path (without
      destructive `reset-workspace` guidance) in `CHANGELOG.md`,
      `proposal.md`, and `design.md`.
- [x] Run full validation ladder (`go build`, `go vet`, `-race -short`,
      `-race -tags=integration` on `nanobrain_test`, `openspec validate`,
      `gofmt`, `git diff --check`, `govulncheck`) and commit locally.
