# PHASE 1: PREPARE — feature/config-env-var

**Feature**: `feat/config-env-var` (commit `4787ea3`, closes #219)
**Date**: 2026-05-30
**Tester**: Sisyphus (RRI-T methodology)
**Test instance**: nano-brain on port 8899 (custom config at `/tmp/nano-brain-custom/config.yml`)
**Bug policy (user directive)**: NO FIX — file GitHub issue with labels `bug,rrit` + attach log file, continue.

## Feature Under Test

`config.ResolveConfigPath(flagValue string) string` in `internal/config/config.go:371`.

Precedence (high → low):
1. `--config` CLI flag (non-empty)
2. `NANO_BRAIN_CONFIG` env var (non-empty)
3. `DefaultConfigPath()` → `~/.nano-brain/config.yml`

Refactored across 12 call sites in `cmd/nano-brain/*.go`.

## Scope of Test

| In scope | Out of scope |
|---|---|
| Precedence (flag > env > default) | Re-testing legacy `--config` flag (pre-existing) |
| Empty-string handling (env="" should fall to default) | Config file content validation (separate concern) |
| Non-existent path handling | Schema migration |
| Symlink / relative path resolution | Performance of YAML parser |
| Interaction with all 12 refactored commands | Internal `koanf` library |
| Container scenario (`host.docker.internal`) | Host-side scenarios |
| Security: path traversal, perm bits | Auth / network |

## Dimensions Targeted (6 of 7)

| Dimension | Applicable | Reason |
|---|---|---|
| UI/UX | ❌ | CLI tool — no UI; covered by "DX" within other dims |
| API | ✅ | `/api/status`, `/health`, server boot |
| Performance | ✅ | Config load time, startup latency |
| Security | ✅ | Path traversal, perm checks, env var leakage |
| Data Integrity | ✅ | Config values land where expected (port, DB URL, etc.) |
| Infrastructure | ✅ | Container scenario, file system edge cases |
| Edge Cases | ✅ | Empty string, missing file, malformed YAML, both set |

## Environment

- **Repo**: `/Users/tamlh/workspaces/self/AI/Tools/nano-brain`
- **Branch**: `feat/config-env-var` @ `4787ea3`
- **Binary**: `./nano-brain` (rebuilt 2026-05-30 04:12, size 53147485 bytes)
- **PostgreSQL**: `host.docker.internal:5432` (PG17 + pgvector 0.8.2)
- **Ollama**: `host.docker.internal:11434`
- **Test port**: **8899** (documented in `AGENTS.md` "RRI-T Test Instance" block)
- **Test config**: `/tmp/nano-brain-custom/config.yml`
- **Log dir**: `ai/test-case/rri-t/config-env-var/logs/`

## Pre-flight Checks

- [x] Binary builds cleanly (`go build ./cmd/nano-brain`)
- [x] Default 3100 instance reachable (sibling process)
- [x] Test instance 8899 already running (verified `/api/status` returns healthy JSON)
- [x] Output dir `/ai/test-case/rri-t/config-env-var/` created
- [x] AGENTS.md updated with RRI-T test instance block
- [x] Bug policy understood: file issue, don't fix

## Success Criteria for RRI-T Pass

Per skill release gates:
- **GO**: all 6 dims ≥ 70%, OR 5/6 ≥ 85%, AND zero P0 FAIL
- **NO-GO**: any dim < 50%, OR > 2 P0 FAILs, OR critical MISSING
