# Tasks: `doctor` Command

## Task 1: Create `cmd/nano-brain/doctor.go`

- [ ] Define `runDoctorCmd(args []string)` function
- [ ] Parse `--json` flag
- [ ] Load config via `config.Load(configPath)`
- [ ] Implement 6 checks in order (config, pg, pgvector, migrations, ollama, model)
- [ ] Each check: name, status (ok/fail), detail string, hint on failure
- [ ] Human-readable output with aligned dots
- [ ] JSON output when `--json` flag is set
- [ ] Exit code 0/1 based on results
- [ ] Per-check timeout: 3 seconds
- [ ] Handle Voyage AI provider (skip Ollama, check API key instead)

## Task 2: Wire into `main.go`

- [ ] Add `case "doctor":` to the switch in `main()`
- [ ] Call `runDoctorCmd(args[1:])`

## Task 3: Tests

- [ ] Unit test for JSON output format
- [ ] Unit test for human-readable output format
- [ ] Test with missing config (should fail gracefully)

## Validation

- [ ] `go build ./...` passes
- [ ] `go test -race -short ./...` passes
- [ ] Manual: `nano-brain doctor` shows all checks
- [ ] Manual: `nano-brain doctor --json` outputs valid JSON
