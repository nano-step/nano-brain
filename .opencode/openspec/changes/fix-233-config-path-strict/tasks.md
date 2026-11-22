# Tasks

- [x] Write red tests in `cmd/nano-brain/config_cmd_test.go`,
      `cmd/nano-brain/doctor_test.go`, and `cmd/nano-brain/commands_test.go`
      covering missing explicit config warnings in `config show`, `doctor`
      `Config... FAIL` ("file does not exist") vs zero-config `OK`, and
      `--config` whitespace trimming + threading across CLI commands.
- [x] Implement strict config resolution, `--config` whitespace trimming,
      subcommand `configPath` threading, and `doctor` missing-file `FAIL` check
      across `internal/config`, `internal/health/doctor`, and `cmd/nano-brain`.
- [x] Add `CHANGELOG.md` entry under `[Unreleased] -> ### Fixed` for #233.
- [x] Run `go build ./...`, `go vet ./...`, `go test -race -short ./...`,
      `openspec validate fix-233-config-path-strict --strict`, `gofmt -l`, and
      `git diff --check`, then commit locally through the repo harness.
