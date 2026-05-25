# Tasks: `doctor` Command

## Task 1: Create `cmd/nano-brain/doctor.go`

- [x] Define `runDoctorCmd(args []string)` function
- [x] Parse `--json` flag
- [x] Load config via `config.Load(configPath)`
- [x] Implement 6 checks in order (config, pg, pgvector, migrations, ollama, model)
- [x] Each check: name, status (ok/fail), detail string, hint on failure
- [x] Human-readable output with aligned dots
- [x] JSON output when `--json` flag is set
- [x] Exit code 0/1 based on results
- [x] Per-check timeout: 3 seconds
- [x] Handle Voyage AI provider (skip Ollama, check API key instead)

## Task 2: Wire into `main.go`

- [x] Add `case "doctor":` to the switch in `main()`
- [x] Call `runDoctorCmd(args[1:])`

## Task 3: Tests

- [x] Unit test for JSON output format
- [x] Unit test for human-readable output format
- [x] Test with missing config (should fail gracefully)

## Validation

- [x] `go build ./...` passes
- [x] `go test -race -short ./...` passes
- [x] Manual: `nano-brain doctor` shows all checks
- [x] Manual: `nano-brain doctor --json` outputs valid JSON
