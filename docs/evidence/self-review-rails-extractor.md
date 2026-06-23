# Self-Review: Rails Route Extractor + Configurable Timeout

**Date:** 2026-06-15
**Story:** rails-extractor (#439)
**Reviewer:** Sisyphus (self)

## Files Changed
- `internal/graph/rails_extractor.go` (747 lines) — Rails route extraction via tree-sitter Ruby
- `internal/graph/rails_extractor_test.go` (158 lines) — 7 unit tests
- `internal/graph/testdata/rails/routes.rb` (31 lines) — fixture with real rails-app patterns
- `internal/graph/detector.go` — detectRails via Gemfile check
- `internal/graph/detector_test.go` — 3 Rails detection tests
- `cmd/nano-brain/main.go` — register RailsExtractor
- `internal/config/config.go` — add RequestTimeout field
- `internal/config/defaults.go` — default 600s
- `internal/codesummarize/provider.go` — use config timeout

## Review Checklist

### Correctness
- [x] All 7 Rails tests pass
- [x] Full graph test suite passes (all extractors)
- [x] Build succeeds (`go build ./...`)
- [x] Detector correctly identifies Rails via `gem 'rails'` in Gemfile
- [x] Monorepo support (1-level subdirectory search)
- [x] `go vet ./...` clean

### Pattern Compliance
- [x] Follows express_extractor.go pattern (FrameworkAwareExtractor interface)
- [x] Follows nuxtjs_extractor.go pattern for filesystem-based detection
- [x] Uses existing tree-sitter/grammars (RubyLanguage)
- [x] EdgeHTTP kind with metadata map
- [x] SourceFile normalized to slash
- [x] Line numbers via lineForByte

### Rails Patterns Covered
- [x] `resources :foo` — 7 RESTful routes
- [x] `resources :foo, only: [...]` — filtered
- [x] `namespace :api do` — path prefix
- [x] `scope "path" do` — path prefix
- [x] `get/post/put/patch/delete` — direct routes
- [x] `resources + collection do` — collection routes
- [x] `resources + member do` — member routes with :id
- [x] `mount Engine => "/path"` — mount points
- [x] `root to: "ctrl#action"` — root route
- [x] `devise_for :users` — auth routes
- [x] `redirect` routes — correctly skipped
- [x] Nested namespaces (`api/v1`)
- [x] Symbol actions (`post :upload`)

### Config Changes
- [x] `code_summarization.request_timeout` defaults to 600s
- [x] Falls back to 600s if not set (backward compatible)
- [x] Config documented in config.yml

## Verdict: PASS
