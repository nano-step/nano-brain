# Tasks

- [x] Write red unit tests in `internal/symbol/extractor_gaps_704_test.go`
      covering same-name top-level vs nested/assignment dedupe, `satisfies`/`as`/
      parenthesized arrows, multi-level HOF wrappers (`memo(forwardRef(...))`,
      `var` + HOF) without collection-callback false positives, and class-field
      arrows across `.ts`, `.tsx`, `.js`, `.jsx`, and `.vue`.
- [x] Implement shared candidate ranking, initializer unwrapping / HOF filtering,
      and class-field arrow extraction across `typescript_extractor.go`,
      `javascript_extractor.go`, and `vue_extractor.go`.
- [x] Add `CHANGELOG.md` entry under `[Unreleased] -> ### Fixed` documenting #704
      and `POST /api/v1/update` recovery for existing workspaces.
- [x] Run full validation ladder (`CGO_ENABLED=0 go build ./...`, `go vet ./...`,
      `go test -race -short ./...`, integration tests on `nanobrain_test`,
      `openspec validate`, `gofmt`, `git diff --check`) and commit locally.
