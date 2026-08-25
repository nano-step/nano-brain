## Why

Dogfooding nano-brain on a real multi-repo workspace surfaced four install/first-run defects. Adversarial review then established that one of them is not a cosmetic defect at all — it is a live data-loss trap, and the original framing of the fix pointed straight at the trigger.

### The trap (found during review, now the primary driver)

`memory` and `sessions` are **DB-backed logical collections**. Their documents are inserted directly by the harvester and summarizer (`internal/harvest/engine.go:264`, `internal/summarize/persist.go:128,174`, `internal/server/handlers/ticket.go:31`) and carry source paths that are identifiers, not files — `buildSourcePath` returns `summary://claude/<session-id>` (`internal/summarize/persist.go:254-263`), and CLI-written memory documents pass an empty `source_path` straight through (`internal/server/handlers/document.go:145`). A full audit of every non-test `MkdirAll` in `internal/` and `cmd/` confirms **no code path ever creates `~/.nano-brain/memory` or `~/.nano-brain/sessions`**; the summarizer's on-disk output goes to `~/.nano-brain/summaries` (`internal/config/defaults.go:95`).

Yet `POST /api/v1/init` registers both as ordinary filesystem-backed collections (`internal/server/handlers/workspace.go:72-93`), so reindex walks them. The reindex orphan loop deletes every indexed document whose `source_path` fails `os.Stat` (`internal/server/handlers/reindex.go:227-252`). `os.Stat("summary://…")` and `os.Stat("")` can never succeed.

Two accidents are all that stand between that loop and the corpus:

1. `~/.nano-brain/sessions` does not exist, so `walkCollectionFiles` errors at its stat guard (`reindex.go:295-299`) and the caller warns and `continue`s (`:155-160`).
2. `~/.nano-brain/memory` exists but is empty, so the catastrophic-deletion guard fires — and that guard's condition is `len(diskFiles) == 0 && len(indexedRows) > 0` (`reindex.go:182-189`), which **a single file defeats**.

Measured on a live install: `sessions` holds 2003 documents, 2002 with `summary://…` paths; `memory` holds 185, 98 with empty paths. So `mkdir -p ~/.nano-brain/sessions && touch ~/.nano-brain/sessions/x` — the obvious way an operator silences a recurring warning — deletes ~2100 documents and their chunks on the next reindex. `collectionsToReindex` returns *all* collections when the requested root matches nothing (`reindex.go:351-353`), so a plain workspace-root reindex sweeps them too.

**The recurring `root path inaccessible` warning is therefore load-bearing.** It is the guard reporting, not noise.

### The three install defects

- **`nano-brain service …` is invisible.** Shipped by #615 and dispatched at `cmd/nano-brain/main.go:86`, it is absent from `printUsage()` (`cmd/nano-brain/ops.go:551-601`). This is a recurrence of issue #527, and `cmd/nano-brain/ops_usage_test.go` — written to prevent exactly that — cannot catch it: its `dispatchedCommands` slice is hand-maintained (`:13-20`, 34 of 35 commands, `service` missing) and its assertion is `strings.Contains` (`:27-31`). The test passes green today with the drift present (verified by running it). Worse, the gap is not only help text: **nothing in the product ever recommends `service install`.** Every start affordance points at the unsupervised `serve -d` — `suggestStartCommand()` (`cmd/nano-brain/client_helpers.go:67-72`), the dead-daemon message (`:76-83`), both wizard branches (`cmd/nano-brain/init_serve.go:72,82`), and the wizard itself, which has no service step at all (`cmd/nano-brain/init.go:128-215`). A user who follows `install.sh` → `nano-brain init` is handed a daemon that does not survive reboot and is never told otherwise.
- **`nano-brain init` cannot parse its own flags.** `cmd/nano-brain/commands.go:19-40` pre-scans for booleans and never consumes the value after `--root`, so the path falls to `default:` and exits. The correct parser at `:70-96` is **entirely unreachable** — not just for `--root` but for `--workspace`, `--json`, and `--force`, the last of which fronts a destructive reset-workspace flow (`:98-117`). Verified against a HEAD build: all six invocation forms exit 1. The `=` form is documented in the agent-facing skill (`SKILL.md:56`), the team runbook (`docs/SETUP_AGENT.md:279,410`), and the V1 migration note (`CHANGELOG.md:541`), and has never worked in any code path.
- **npm postinstall dead-ends at unpublished versions.** `candidateTagsForVersion()` (`npm/postinstall.js:182-193`) requires an all-digit patch segment, so `0.0.0-dev` produces one candidate tag, the API fallback finds nothing (`:215-228`), and the run aborts at `:345` with no next step.

## What Changes

- **Logical collections are never walked.** `memory` and `sessions` are excluded from filesystem walking and from orphan deletion, so the deletion path is unreachable for documents whose source paths are synthetic or empty. Their rows are kept — consumers filter on the names (`internal/server/handlers/wakeup.go:130`, `internal/mcp/tools.go:1791`) and `triggerForceWipe` operates on rows by name (`reindex.go:105-129`).
- **`printUsage()` and the dispatcher cannot drift, in either direction.** Every dispatched command is documented, every documented command is dispatched, enforced by a test whose expected set comes from the dispatcher rather than a maintained slice, and whose assertion anchors on the command column instead of a substring.
- **`service install` is recommended, not merely referenced.** Where the platform supports it, the install path surfaces the supervised option instead of only `serve -d`.
- **`nano-brain init` parses all of its flags in one pass** — `--`, `--yes`, `--root`, `--workspace`, `--json`, `--force` — accepting both `--flag value` and `--flag=value` for value-taking flags, and rejects a `--root` that does not resolve to a readable directory.
- **npm postinstall fails actionably.** When no release asset matches the package version it reports the tags it tried and the supported alternatives, while preserving the existing hard-fail on integrity errors.
- **The npm test suite runs in CI.** `.github/workflows/ci.yml` is Go-only today, so `npm/postinstall.test.js` and `npm/run.test.js` have never run in CI — recorded at `docs/evidence/review-npm.md:21` and never actioned.

### Non-goals

- **`~/.nano-brain/memory` and `~/.nano-brain/sessions` MUST NOT be created, and their collection roots MUST NOT be made walkable.** This is the trigger, not the fix.
- Collection rows for logical collections MUST NOT be deleted — consumers filter on the names and deletion is irreversible.

### Breaking changes

**BREAKING — `POST /api/v1/init` contract.** Root validation means inputs that returned `200` now return `400`: a `root_path` that does not exist, is a regular file, or is an unreadable directory. This is deliberate — the watcher and `walkCollectionFiles` both walk `col.Path` on the *daemon's* filesystem, so a path the daemon cannot stat can never be indexed, and registration would only have created a permanently-empty ghost workspace. The remote-daemon topology (`docs/SETUP_AGENT.md:393-409`) is the case to watch: a developer running `init --root <local-path>` against a remote daemon is now rejected rather than silently registered, which is the correct outcome for the same reason.

No currently-working CLI invocation changes behavior — every `init` flag form fails today. Registration behavior is unchanged, so `internal/server/handlers/workspace_test.go:215-262` (which asserts exactly three `UpsertCollection` calls at `:249-251`) stays green; the walk exclusion is applied at read time, not at registration.

## Capabilities

### New Capabilities
- `logical-collection-isolation`: `memory` and `sessions` are DB-backed namespaces that are never walked from disk and are unreachable from orphan deletion, regardless of whether their nominal root exists.
- `cli-usage-parity`: bidirectional parity between the command dispatcher, `printUsage()`, and the agent-facing skill docs, enforced by a derived, column-anchored test.
- `cli-init-argument-parsing`: `nano-brain init` parses its full flag set in a single pass, in both space and `=` forms, and validates `--root`.
- `npm-install-fallback`: postinstall produces an actionable, tag-aware diagnostic when no release asset matches the package version, without weakening integrity enforcement.

### Modified Capabilities

None. Two candidates were raised in review and both were rejected on evidence:

- **`managed-daemon-service` — rejected.** `spec.md:8-13` requires the CLI to "expose" the service subcommands, and it does: dispatched at `cmd/nano-brain/main.go:86-88`, implemented at `cmd/nano-brain/service.go:85-105`, platform-guarded at `:93-98`. Its scenarios cover install targets and platform guards, not discoverability. Reading "expose" as "document in help" would retroactively make an archived, PASSed change non-compliant against a definition it never used. Discoverability belongs to the new `cli-usage-parity`.
- **`skill-distribution-docs` — rejected, and the finding is worse than a misattribution.** That spec names `README.md`, `.opencode/skills/nano-brain/SKILL.md`, and the copy shipped via `@nano-step/skill-manager`. Root `SKILL.md` — the file carrying the broken command table — is **none of those**: `package.json` `files` is `['npm/', 'README.md', 'LICENSE']`, and no workflow references it. It is an **unowned user-facing surface**. This change brings it under `cli-usage-parity` rather than attaching it to a spec that does not cover it.

## Impact

- `internal/server/handlers/reindex.go` — walk target selection and orphan-deletion reachability
- `internal/server/handlers/workspace.go` — collection registration, watcher attach loop (`:167-183`), root validation on `POST /api/v1/init` (`:121-133`)
- `internal/server/handlers/workspace_test.go` — the 3-collection contract test must be rewritten
- `cmd/nano-brain/main.go` (dispatch), `cmd/nano-brain/ops.go` (`printUsage`), `cmd/nano-brain/ops_usage_test.go`
- `cmd/nano-brain/commands.go` (`init` parsing), `cmd/nano-brain/client_helpers.go` + `cmd/nano-brain/init_serve.go` + `cmd/nano-brain/init.go` (service recommendation)
- `npm/postinstall.js`, `npm/postinstall.test.js`, `npm/run.js`, `npm/run.test.js` — `run.js` shares `ensureBinary` for the lazy first-invocation download and prints the same thrown message
- `.github/workflows/ci.yml` — add a Node job
- `SKILL.md`, `skills/nano-brain/SKILL.md`
- No database migration: the exclusion applies at read time to rows that already exist. No HTTP contract change, no MCP tool signature change.

### Corrections to earlier drafts of this proposal

Recorded so the Review Gate does not re-litigate them:

- **The published npm path is not broken.** `0.0.0-dev` is the intentional on-master placeholder; CI rewrites it before publish (`AGENTS.md:515`, `.github/workflows/release.yml:96`) and `scripts/check-npm-release.sh:166-193` gates on version-equals-tag. Registry check: both `nano-brain` and `@nano-step/nano-brain` are live at `2026.8.1201`, same repo and maintainer, and that version resolves on the first candidate tag. The failure is confined to postinstall running inside a source checkout. **No README caveat will be added** — it would warn users away from a working path.
- **`POST /api/v1/init` is not the only working registration path.** The interactive wizard registers an arbitrary path (`cmd/nano-brain/init.go:190-193`). The accurate gap is that no *non-interactive* CLI registration exists: the wizard is TTY-gated (`:137-142`) and `--yes` ignores `--root` (`commands.go:55-58`). Scripts, CI, and agents are the affected callers.
- **A dead daemon does not return silent empty results.** MCP is served by the daemon, so it is unreachable, and the CLI prints an explicit actionable connect error (`cmd/nano-brain/client_helpers.go:76-83`). Silent-empty-with-no-error is the signature of a live daemon over an empty index — nano-step/nano-brain#622.
- **The `service` gap is help-text-only, not a documentation gap.** `README.md:76-80` and `docs/SETUP_AGENT.md:236-240` both document the subcommands. Note that #615's own `tasks.md:45` task 7.1 documented `service` in those two files but not in `nano-brain help` — the drift passed a full Review Gate.

### Non-scope

- Newly registered workspaces persist nothing while `reindex` reports non-zero counts — nano-step/nano-brain#622. Probed during review: a fresh 3-file workspace returned `watcher_triggered: true` with `docs_total: 0`, which places the failure downstream of the watcher trigger rather than in registration wiring.
- Empty symbol table for JS/TS workspaces; no `.vue` symbol extractor — nano-step/nano-brain#624.
- `graph/impact` returning `impacted: null` for both "no results" and "unknown node" — nano-step/nano-brain#625.

**Release sequencing:** #622 must ship before the install experience is announced as fixed. A user who installs cleanly and then queries into a void has had the same experience as before, with better help text.
