# Adversarial review adjudication — #745

Reviewer inputs:

- Architecture adversary (background): PROCEED WITH CHANGES. Key evidence: reusing `session_dirs` is correct; fixed startup-only leaf harvester construction cannot satisfy first-session/workspace-registration flows; encoded workspace leaf names are non-injective; the repo has a safer transcript-`cwd` direction in the unfinished `SessionSource`/`Engine` seam; CCS root derivation and init rendering are underspecified.
- Delivery adversary: BLOCK. Key evidence: the packet initially lacked design/tasks/specs; fresh init starts/reuses the server before workspace registration while harvesters are fixed at startup; explicit configs disable auto-detection; existing configs need a migration/upgrade contract; `CCS_HOME` means `$CCS_HOME/.ccs`; `--config-dir` cannot be discovered; verification must use a fabricated CCS layout and isolated test DB.
- Architecture adversary (bounded second pass): PROCEED WITH CHANGES. Key evidence: never persist concrete per-profile dirs; prefer `enabled: true` plus runtime auto-detection for standard roots; document explicit-list replacement semantics; strict precedence versus union is a product/privacy decision; project-local `.mcp.json` docs must be fixed.

## Decisions

1. Reuse `harvester.claudecode.session_dirs`; do not add `session_dir_globs`.
2. Implement dynamic harvest-cycle discovery rather than expanding the fixed startup-only leaf-harvester design. Re-resolve configured globs and registered workspaces on every harvest cycle so first sessions, new profiles, newly registered workspaces, and manual `nano-brain harvest` work without a daemon restart. This is required for the issue's verification flow; it is not the unrelated live file-watcher/backlog item.
3. Keep configuration changes restart-required. Dynamic discovery applies within the already-loaded root patterns; it does not make `harvester.claudecode.*` reloadable.
4. Harden workspace attribution in the legacy harvester: production leaf harvesters receive the registered workspace path and skip a transcript whose explicit `cwd` does not match it after path normalization. Transcripts with no `cwd` keep legacy behavior for compatibility. Full migration to the unfinished `SessionSource`/`Engine` architecture is out of scope.
5. Implicit CCS discovery scans the union of known candidate roots in deterministic precedence order (`CCS_DIR`, then `$CCS_HOME/.ccs`, then `~/.ccs`) because harvesting is retrospective and users may have historical sessions in more than one root. Operators who need restrictive privacy boundaries can set an explicit `session_dirs` list. This rejects the strict-precedence prescription while accepting the underlying privacy concern as a documentation/contract requirement.
6. `--config-dir` is invocation-scoped and cannot be discovered. Document manual `session_dirs` configuration for it. An explicit non-empty `session_dir`/`session_dirs` remains a complete replacement list and disables auto-detection; no implicit broadening of existing restrictive configs.
7. Init writes only `enabled: true` when detectable Claude/CCS storage exists and leaves both directory fields empty, preserving daemon auto-detection and avoiding stale concrete profile paths/profile-name leakage. The generated template gains commented `session_dirs` examples for manual roots. “Silent” means zero additional questions, not invisible: init prints a concise enablement notice.
8. Existing configs are never rewritten implicitly. Interactive keep remains untouched; docs provide the two-line manual upgrade (`enabled: true`, leave dirs blank). `init --yes` retains its existing overwrite semantics and is not a migration path.
9. Auto-detection stores stable projects-parent patterns, including CCS instance globs, rather than only currently existing concrete profile dirs. Detection is performed before `server.New` copies config so `/api/status` reflects effective configured patterns/resolved dirs.
10. Raw/active-session re-ingestion semantics are pre-existing and out of scope. This change must not make them worse; verification uses a completed fixture session.
11. Codex-via-CCS harvesting remains out of scope (#739). Codex MCP wiring is not claimed as fixed.

## Net change required

- Extend detection for CCS env roots and stable glob patterns.
- Replace startup-only leaf instantiation with a dynamic multi-root Claude harvester.
- Add transcript-`cwd` validation for production workspace binding.
- Render conditional `claudecode.enabled` in init and document explicit/commented `session_dirs`.
- Update configuration/setup/roadmap docs and tests.
- Prove with unit tests plus isolated PostgreSQL integration/fixture tests; no real CCS install is required for file-layout verification.

## Reopened questions

- Whether/when to finish the broader `SessionSource`/`Engine` migration and active-session re-ingestion model. Deferred follow-up, not a blocker for #745.
- Whether profile/root identity should be added to session metadata. Deferred; current issue only requires discovery/harvest.
- Windows-specific Claude path mangling and CCS behavior. Not expanded by this change; tests remain platform-aware.

## Verdict

SEND BACK WITH exact item list:

1. Complete the OpenSpec packet before implementation.
   Prevented by: strict OpenSpec validation plus required design/tasks/spec artifacts.
2. Implement dynamic harvest-cycle discovery or narrow the user-facing verification contract.
   Prevented by: fixture integration proving first session/new workspace discovery via manual harvest without daemon restart.
3. Specify and test CCS root derivation, including `$CCS_HOME/.ccs` and manual `--config-dir` behavior.
   Prevented by: detection unit tests with controlled HOME/CCS_DIR/CCS_HOME.
4. Specify exact init rendering and existing-config upgrade behavior.
   Prevented by: template render/load round-trip tests and init tests with controlled detection environment.
5. Harden workspace attribution against encoded-leaf collisions using transcript `cwd` where present.
   Prevented by: mismatch/collision regression tests.
6. Update all user-visible docs consistently, including project-local `.mcp.json` guidance.
   Prevented by: docs diff review and smoke evidence.
