# Tasks

- [x] 1.1 Verify all skill copies and cold-workspace behavior against `internal/server/`, `internal/mcp/`, `internal/watcher/`, and `cmd/nano-brain/`.
- [x] 1.2 Update `.opencode/skills/nano-brain/SKILL.md` and `.opencode/skills/nano-brain/references/{http-api.md,code-intelligence.md}` with verified cold-workspace diagnostics and `memory_impact`/`memory_update` contracts.
- [x] 1.3 Update `skills/nano-brain/SKILL.md`, `skills/nano-brain/AGENTS_SNIPPET.md`, and `skills/nano-brain/references/{http-api.md,code-intelligence.md}` to stay in sync.
- [x] 1.4 Add `CHANGELOG.md` entry under `[Unreleased] -> ### Fixed`, run `go test -race -short ./...` and `git diff --check`, and commit locally via harness.
