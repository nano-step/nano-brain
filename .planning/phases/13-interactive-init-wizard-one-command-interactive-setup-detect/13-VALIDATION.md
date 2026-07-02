---
phase: 13
slug: interactive-init-wizard-one-command-interactive-setup-detect
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-07-02
---

# Phase 13 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib testing, table-driven) |
| **Config file** | none — go.mod toolchain |
| **Quick run command** | `go test -race -short ./cmd/nano-brain/... ./internal/health/doctor/...` |
| **Full suite command** | `go test -race -short ./...` |
| **Estimated runtime** | ~60 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test -race -short ./cmd/nano-brain/... ./internal/health/doctor/...`
- **After every plan wave:** Run `go test -race -short ./...`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 90 seconds

---

## Per-Task Verification Map

*Filled by planner — tasks must map to doctor skip-path tests, docker provisioning fake-runner tests, wizard step tests via promptReader injection, and registration-helper extraction tests.*

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| TBD | — | — | D-01..D-18 | — | no unconsented file writes; DSN credentials never echoed | unit | `go test -race -short ./cmd/nano-brain/...` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `cmd/nano-brain/docker_provision_test.go` — fake execRunner seam stubs (name-collision exit 125, port-bind exit 125 + stray container rm, docker start recovery)
- [ ] `internal/health/doctor/doctor_test.go` — extend for provider=="" skip path (create if missing)

*Existing infrastructure (promptReader/isTTYFn injection, httptest.Server patterns in commands_test.go) covers wizard prompt-flow tests.*

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Fresh-machine end-to-end wizard (real Docker pull + container + serve + MCP write) | D-05..D-17 | Requires real Docker daemon and interactive TTY | Run `nano-brain init` on a machine with Docker but no nanobrain-pg; accept defaults; verify MCP tools respond after client restart |
| Windows serve-step degradation message | D-14/specifics | Windows runner not in CI | Cross-compile check `GOOS=windows go build ./cmd/nano-brain` must not regress further; message path unit-tested with runtime.GOOS seam if feasible |

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 90s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
