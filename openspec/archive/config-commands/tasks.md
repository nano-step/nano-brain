## Tasks

- [ ] Add `case "config"` to main.go switch, pass configPath
- [ ] Create `cmd/nano-brain/config_cmd.go` with runConfigCmd, runConfigShow, runConfigCheck
- [ ] Implement config show with YAML output and sensitive value masking
- [ ] Implement config check as alias to runDoctorCmd
- [ ] Build + test
- [ ] E2E: `nano-brain config show`, `nano-brain config show --json`, `nano-brain config check`
