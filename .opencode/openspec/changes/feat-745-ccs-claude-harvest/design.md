# Design: CCS Claude session harvesting

## Configuration contract

Keep `harvester.claudecode.session_dirs` as the only multi-root key. Entries are Claude **projects parent directories** such as `~/.claude/projects` or `~/.ccs/instances/*/projects`, never leaf session directories. The legacy singular `session_dir` remains first in the union for backward compatibility.

A non-empty `session_dir` or `session_dirs` is a complete explicit source list and disables startup auto-detection. This preserves existing operators' ability to restrict ingestion. Users of per-invocation CCS `--config-dir` must add that root's `instances/*/projects` pattern explicitly; nano-brain cannot infer invocation-scoped CLI flags after the fact. No environment-variable interpolation is added to config values.

## CCS root auto-detection

When harvesting is enabled and both directory fields are empty, daemon startup derives candidate CCS roots in this deterministic order:

1. `CCS_DIR` — already the CCS root.
2. `CCS_HOME/.ccs` — CCS's legacy home rule.
3. `<user home>/.ccs` — default root.

Detection scans the union, not strict precedence, because harvesting is retrospective: users may have historical sessions under roots they no longer use. The order only controls deterministic dedupe/logging. Roots are cleaned, tilde-expanded where applicable, converted to absolute form when already absolute or tilde-based, and deduplicated lexically. Relative env values are not guessed against the daemon's cwd; operators should use absolute env paths or explicit `session_dirs`.

For each candidate root, the stable pattern is `<root>/instances/*/projects`. Evidence-based detection includes a CCS pattern when the root exists or the corresponding env var explicitly names it, and includes platform Claude projects dirs when the projects dir or its immediate Claude config parent exists. Runtime auto-detection additionally retains the standard platform Claude patterns and default `~/.ccs` instance glob even before those directories exist; this does not make init treat absent storage as detectable, but it lets dynamic discovery handle a first install after daemon startup. `CLAUDECODE_STORAGE_DIR` is explicit operator intent, so it is retained even before the directory exists; dynamic discovery can pick it up when created.

Auto-detection records these stable patterns in memory and injects them as the effective startup harvester config, so `/api/status` reflects what the daemon scans while reload comparison continues to use the file-backed config. It does not rewrite the YAML file.

## Dynamic harvest-cycle discovery

Replace fixed startup-only leaf construction with one `ClaudeCodeDiscoveryHarvester` implementing the existing `Harvester` interface. On every serialized harvest cycle it:

1. re-runs the configured resolver, so glob patterns are re-expanded;
2. filters missing/non-directory base dirs and canonical-dedupes symlink-equivalent concrete dirs;
3. re-reads registered workspaces from PostgreSQL;
4. iterates base dir × workspace in deterministic order;
5. checks `<base>/<encoded-workspace-path>` only at that moment;
6. constructs an ephemeral leaf harvester for newly visible or existing leaf dirs and delegates the established parsing/summarization/raw-fallback path.

This makes first sessions, new CCS profiles, newly registered workspaces, and manual `POST /api/harvest` discoverable without daemon restart. It does not make YAML changes reloadable: the resolver closes over the startup-loaded config, and `harvester.claudecode.*` remains restart-required.

Errors in one leaf remain isolated to that leaf. Missing roots are non-errors because globs may match in the future.

## Workspace attribution hardening

Encoded Claude project leaf names are not injective. Claude replaces every non-alphanumeric UTF-16 code unit with a dash and, for names over 200 UTF-16 units, truncates to 200 units plus a deterministic base-36 Java-style hash suffix. Punctuation, spaces, separators, and literal dashes therefore collapse. Production discovery probes both the registered spelling and its canonical symlink-resolved spelling. Ephemeral leaf harvesters receive both workspace hash and registered workspace path. After parsing a transcript and before persistence/summarization, the first explicit `cwd` is compared with that workspace path using clean/absolute symlink-aware path normalization. A mismatch is skipped and logged. Discovery groups registered workspaces by physical encoded leaf before harvesting. A missing `cwd` preserves legacy behavior only for a single-workspace leaf; when multiple distinct workspaces collide in one leaf, the transcript is ambiguous and is skipped fail-closed instead of being copied into every candidate workspace.

This is a bounded hardening of the production harvester, not a migration to the unfinished generic `SessionSource`/`Engine` pipeline. It prevents modern cross-workspace ingestion in collision cases and avoids ambiguous legacy cross-copying, while still preserving cwd-less ingestion for ordinary non-colliding leaves.

## Init rendering

`nano-brain init` remains zero-question for harvesting. During config rendering it runs the same detection source scan. If any Claude/CCS source is detectable, the generated YAML contains:

```yaml
harvester:
  claudecode:
    enabled: true
    session_dir: ""
```

`session_dirs` remains absent/commented so runtime auto-detection stays active and no concrete CCS profile names or stale point-in-time paths are persisted. The template includes commented examples for standard and CCS manual roots and documents that explicit lists replace auto-detection.

The wizard prints one concise notice when it enables harvesting. It does not ask a question and does not print every profile path. Interactive init's existing keep path never rewrites config. Existing users upgrade manually by setting `enabled: true` and leaving the directory fields empty, or by writing an explicit complete `session_dirs` list.

## Privacy and operational boundaries

Scanning multiple CCS profiles merges those profiles' sessions into the same workspace memory. That is the requested recall behavior but is documented explicitly. Operators requiring separation must configure a restrictive explicit source list or keep harvesting disabled. Raw transcripts may be embedded or summarized according to the user's existing embedding/summarization configuration; rollback of config does not delete already-ingested documents.

## Verification design

No real CCS installation is required for layout verification. Tests fabricate `instances/<profile>/projects/<encoded-fake-workspace>/<session>.jsonl` under temporary roots with controlled `HOME`, `CCS_DIR`, and `CCS_HOME`. Unit tests cover root derivation/rendering. Isolated `nanobrain_test` integration tests cover dynamic first-session discovery, newly registered workspaces, cwd mismatch skipping, and query-visible persisted content. The dev database and port 3100 are not used.
