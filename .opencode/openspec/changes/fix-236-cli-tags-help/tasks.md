# Tasks

- [x] Add red unit tests in `cmd/nano-brain/commands_test.go` covering `--help`, `-h`, empty-arg usage for `query`, `search`, `vsearch`, and top-level `printUsage` `--tags` summary.
- [x] Support `-h`/`--help` and complete `printStubUsage` in `cmd/nano-brain/commands.go`.
- [x] Add `[--tags t1,t2,...]` to `query`, `search`, and `vsearch` lines in `cmd/nano-brain/ops.go` and update `CHANGELOG.md`.
- [x] Run `go build ./...`, `go vet ./...`, `go test -race -short ./cmd/nano-brain/...`, `gofmt -l`, and `git diff --check`.
