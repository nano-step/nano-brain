# Claude/CCS session harvesting UX

This note describes how Claude Code harvesting differs when sessions are written through CCS, what the user-facing setup flow is, and what was verified against real CCS transcripts on this machine. It intentionally contains counts and behavior only — no real workspace names, session ids, or transcript content.

## Short answer

CCS does not need a separate harvester or a separate enable flag. It uses the same `harvester.claudecode` pipeline as normal Claude Code sessions. The difference is discovery:

- normal Claude Code commonly writes under a platform projects dir such as `~/.claude/projects`;
- CCS writes under `<ccs-root>/instances/<profile>/projects`;
- nano-brain therefore treats configured values as **projects parent directories** and uses the stable CCS glob `<root>/instances/*/projects`.

For the standard CCS layout, the zero-config flow is:

```yaml
harvester:
  claudecode:
    enabled: true
    session_dir: ""
    session_dirs: []
```

When both directory fields are empty, startup auto-detection scans the union of:

1. `CCS_DIR` directly as a CCS root, when absolute;
2. `$CCS_HOME/.ccs` as the legacy env-derived root, when absolute;
3. `~/.ccs` as the default root;
4. supported platform Claude projects locations.

A non-empty `session_dir` or `session_dirs` is a complete explicit replacement list and disables auto-detection. Include the standard locations explicitly as well if a custom root still needs to coexist with them.

## UX flow

### Fresh install / fresh init

1. The user runs `nano-brain init`.
2. Init detects existing Claude/CCS storage. If found, it renders `harvester.claudecode.enabled: true` without asking an extra harvesting question.
3. Init leaves both directory fields empty. It does not persist concrete per-profile paths into YAML.
4. Init prints the privacy-sensitive notice that Claude/CCS session harvesting was detected and enabled.
5. After the server starts, startup auto-detection resolves the current platform/CCS patterns and starts one dynamic discovery harvester.
6. Once the user registers workspaces, sessions whose transcript `cwd` matches a registered workspace can be ingested. Sessions for unregistered workspaces are not imported.
7. Once harvesting is enabled, new sessions, new CCS profiles, first-use creation of a standard Claude/CCS storage location or an explicitly named `CLAUDECODE_STORAGE_DIR`, and newly registered workspaces are reconsidered on later harvest cycles without restarting the daemon. If init found no Claude/CCS storage and therefore rendered `enabled: false`, installing CCS later does not flip that config flag by itself. YAML edits themselves still require restart.

### Day-to-day use

- Harvest runs once at daemon startup and then every `intervals.session_poll` seconds.
- `nano-brain harvest` or `POST /api/harvest` forces one serialized harvest cycle immediately.
- `/api/status` reports `harvester_status.claude_code.enabled` and the currently resolved session directories.
- Because `/api/status` currently exposes resolved directories rather than the raw glob string, the number of entries can grow when a `instances/*/projects` glob starts matching additional CCS profiles.
- Harvested sessions appear in the `sessions` collection and are searchable through BM25 immediately. Vector search additionally depends on the configured embedding provider.

### Existing installs upgrading to this version

Existing config files are not rewritten implicitly. A user who wants CCS harvesting opts in by setting `enabled: true` and leaving both directory fields empty, then restarting nano-brain. If harvesting was already enabled with blank directory fields, upgrading widens discovery from normal Claude locations to the documented Claude/CCS union. Users who need a narrower privacy boundary should pin explicit `session_dirs`.

### Custom CCS roots

A per-invocation CCS `--config-dir` cannot reliably be inferred after the fact. Add its projects glob explicitly, for example:

```yaml
harvester:
  claudecode:
    enabled: true
    session_dir: ""
    session_dirs:
      - ~/.claude/projects
      - ~/.ccs/instances/*/projects
      - /absolute/path/to/custom-ccs/instances/*/projects
```

Relative `CCS_DIR`/`CCS_HOME` values are rejected rather than guessed relative to the daemon process cwd. Use absolute paths.

## Multi-profile behavior and privacy boundary

Scanning multiple CCS profiles merges sessions from those profiles into workspace memory according to the transcript `cwd` and registered workspace path. It does not create a separate memory namespace per CCS profile. If profile separation is required, use an explicit restrictive `session_dirs` list or separate nano-brain workspaces/instances.

Claude's encoded leaf directory names are lossy. For modern transcripts containing `cwd`, nano-brain validates that transcript `cwd` against the registered workspace before persistence. This prevents lossy encoded-name collisions from attributing a transcript to the wrong workspace. Older transcripts without `cwd` retain the legacy encoded-directory behavior.

## Real-transcript E2E measurement

Measured on this machine on 2026-11-28.

### Real CCS inventory

The default CCS root existed and contained:

- 4 CCS profiles;
- 116 project leaf directories;
- 1,356 JSONL transcript files;
- about 4.3 GB of transcript data;
- cwd fields present in the sampled modern transcripts.

A full ingest of all 1,356 files was not attempted because the point of this check was correctness of the harvest path, not a multi-GB performance or storage benchmark.

### Isolation method

The E2E check used real CCS transcript bytes but copied only a small privacy-safe sample into a temporary CCS root:

- 10 copied transcript files;
- 7 unique sessions;
- 4 CCS profiles represented;
- 3 duplicate cross-profile copies;
- 3 distinct transcript workspace paths;
- about 2.0 MiB copied;
- all selected workspace directories were absent, so workspace registration could not scan real source trees;
- temporary `HOME`, temporary isolated PostgreSQL database, and a non-3100 local port;
- temporary database and copied transcripts were deleted after the run;
- the dev database and port 3100 were not used.

The server was started with blank Claude directory fields, so it exercised the same zero-config auto-detection path a fresh CCS user gets. Five files were present before startup/registration; five were added only after the server was already running to exercise dynamic rediscovery through `POST /api/harvest`.

### Result

| Check | Result |
|---|---:|
| Workspace registrations through `POST /api/v1/init` | 3 |
| First manual harvest | 5 harvested, 0 skipped, 0 errors |
| Second manual harvest after adding files | 2 harvested, 8 skipped, 0 errors |
| Final Claude session documents | 7 |
| Distinct Claude source paths | 7 |
| Workspaces receiving Claude docs | 3 |
| Raw chunks written | 25 |
| Fallback/raw session docs | 7 |
| BM25 `/api/v1/search` hit in first workspace | yes |
| `/api/status` Claude harvesting enabled | true |
| `/api/status` resolved Claude dirs | 6 |

The second-run counts are expected: the five already-ingested sessions were skipped, the two newly added unique sessions were harvested, and the three duplicate cross-profile copies were skipped by source-path/content dedupe.

### Production bug found and fixed by this E2E

The first real server-level run exposed a typed-nil enqueuer panic when embeddings were disabled (`embedding.provider: ""`). The harvest runner received a nil `*embed.Queue` wrapped in a non-nil interface, so raw fallback persistence crashed while enqueueing chunks. `internal/harvest/runner.go` now normalizes typed-nil enqueuers to nil at runner construction, and `internal/harvest/runner_test.go` covers both typed-nil and non-nil behavior. After the fix, the real CCS E2E completed with zero harvest errors.

## Remaining boundaries

- The real-data check was a correctness sample, not a benchmark over the full 4.3 GB local inventory.
- It exercised BM25-only raw fallback; it did not exercise paid/external summarization or embedding providers.
- Historical pre-current Claude leaf encodings and case-insensitive filesystem spelling variants remain outside the current discovery scope.
- `/api/status` still exposes resolved directories rather than a separate raw-pattern field.
