# CCS Claude session harvesting (#745)

Tracking: #745

## Why

Claude Code sessions launched through CCS profiles are isolated under CCS-managed config homes, so users can lose session memory when those directories are not discovered. The multi-directory and glob mechanism requested by the issue already exists as `harvester.claudecode.session_dirs` (#643), but CCS relocation environment variables, zero-config init behavior, harvest-time discovery, and user-facing setup documentation still leave CCS users uncovered or confused.

Independent adversarial review also showed that the current startup-only leaf-harvester construction cannot satisfy the issue's verification flow: a fresh workspace or first Claude/CCS session often appears only after the daemon has already built its fixed harvester list. This change therefore includes bounded harvest-cycle rediscovery and workspace-attribution hardening rather than merely adding more startup roots.

## What Changes

- Reuse the existing `harvester.claudecode.session_dirs` key as the glob-capable list; do not add a duplicate `session_dir_globs` key.
- Extend Claude Code storage auto-detection so CCS instance project dirs are discovered not only under the default `~/.ccs` root but also under CCS roots configured through `CCS_DIR` and `CCS_HOME` (`$CCS_HOME/.ccs`), with deterministic ordering and deduplication.
- Auto-detect stable projects-parent patterns (including CCS instance globs), not only point-in-time concrete profile dirs.
- Replace fixed startup-only Claude leaf instantiation with dynamic harvest-cycle discovery: re-expand configured globs and re-read registered workspaces on each harvest cycle so first sessions, new CCS profiles, newly registered workspaces, and manual `nano-brain harvest` are picked up without a daemon restart.
- Harden workspace attribution by validating transcript `cwd` against the registered workspace path when `cwd` is present, reducing encoded-path collision/cross-ingestion risk while preserving legacy behavior for transcripts without `cwd`.
- Make `nano-brain init` enable Claude Code session harvesting with zero additional questions when detectable Claude/CCS storage exists, while leaving directory fields empty so daemon auto-detection remains authoritative. Print a concise visible notice; never rewrite an existing kept config.
- Document the CCS layout, `session_dirs` glob configuration, custom CCS root/`--config-dir` behavior, explicit-list versus auto-detection semantics, restart/reload boundary, privacy implications of multi-profile recall, and project-local `.mcp.json` as the MCP wiring that survives CCS profile isolation.
- Add regression tests for CCS root detection, dynamic discovery, workspace `cwd` validation, config rendering, backward compatibility, glob expansion/deduplication, and documentation-visible configuration shape.

## Capabilities

### New Capabilities

- `ccs-claude-session-harvesting`: Discovery, zero-config setup, and safe workspace attribution of Claude Code session directories managed by CCS profiles.

### Modified Capabilities

- Claude Code harvesting lifecycle: configured projects roots and registered workspaces are rediscovered on harvest cycles rather than only once at daemon startup.

## Non-goals

- No Codex rollout harvester or CCS `codex-instances` support; that remains blocked on #739.
- No Cursor or Droid harvesting (#740 or future work).
- No migration to the unfinished generic `SessionSource`/`Engine` architecture; this change hardens the existing production harvester instead.
- No rewrite/migration of existing user configs. Existing users get documentation for the explicit opt-in upgrade.
- No reload of changed `harvester.claudecode.*` config without daemon restart; dynamic discovery applies only within the already-loaded patterns.
- No new `session_dir_globs` compatibility alias.
- No scanning of arbitrary home-directory trees outside documented Claude/CCS project-dir patterns.
- No attempt to infer per-invocation CCS `--config-dir` values; they require explicit `session_dirs` configuration.
- No redesign of active/in-progress session re-ingestion semantics, which are pre-existing and independent of CCS discovery.

## Impact

- Code: `cmd/nano-brain/detect.go`, `cmd/nano-brain/init.go`, `cmd/nano-brain/main.go`, `cmd/nano-brain/claudecode_init.go`, `internal/config/template.go`, `internal/harvest/claudecode.go`, `internal/harvest/claudecode_discovery.go`, `internal/server/server.go`, focused tests.
- Config/docs: `docs/CONFIGURATION.md`, `docs/SETUP_AGENT.md`, `docs/ROADMAP.md`, generated config template comments/examples.
- Runtime: harvest-cycle discovery and daemon startup auto-detection; no database migration and no REST/MCP response schema change.
