# Self-Review: Story 10.1 - Basic Auth + Bearer Token

## Files Changed
- `internal/config/config.go` — AuthConfig, UserCred types, env var mappings, validation
- `internal/config/defaults.go` — Auth defaults (disabled, realm=nano-brain, bypass=[/health])
- `internal/config/secrets.go` — Redact users/tokens from API response
- `internal/config/secrets_test.go` — NEW: redaction tests
- `internal/config/config_test.go` — Auth config tests (YAML, env, validation)
- `internal/server/middleware/auth.go` — NEW: Basic Auth + Bearer Token middleware
- `internal/server/middleware/auth_test.go` — NEW: 10 test cases
- `internal/server/middleware.go` — Mount auth middleware, authSnapshotFromConfig
- `cmd/nano-brain/bindsafety.go` — Accept authEnabled param
- `cmd/nano-brain/bindsafety_test.go` — Auth bypass test
- `cmd/nano-brain/auth.go` — NEW: hash + token subcommands
- `cmd/nano-brain/auth_test.go` — NEW: bcrypt + token format tests
- `cmd/nano-brain/main.go` — Wire auth subcommand, pass authEnabled
- `web/src/api/types.ts` — Add auth to Config type
- `web/src/components/NonLoopbackBindBanner.tsx` — Suppress when auth.enabled
- `web/src/__tests__/NonLoopbackBindBanner.test.tsx` — Auth banner test
- `README.md` — Auth config section, CLI commands, env vars

## Validation
- `go build ./...` — PASS
- `go test -race -short ./...` — PASS (all packages)
- `npm run typecheck` — PASS
- `npm run lint` — PASS
- `npm run test -- --run` — PASS (112 tests)
- `npm run build` — PASS (292 KB gzipped)
- Bundle size: 292 KB gzipped (well under 600 KB limit)
