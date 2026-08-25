# Design — fix-install-ux-regressions

## Context

Four defects, one of which (logical-collection walking) sits next to a mechanism that deletes documents. The design decisions below were settled by two independent adversarial reviews plus live probes; each records the alternatives that were rejected and why, so the Review Gate does not re-litigate them.

Live measurements referenced throughout: `sessions` holds 2003 documents, 2002 with `summary://…` source paths; `memory` holds 185, 98 with empty source paths.

## Goals

1. Make the orphan-deletion loop structurally unreachable for DB-backed collections.
2. Make `printUsage()` and the dispatcher unable to drift apart in either direction, and bring root `SKILL.md` under the same guarantee.
3. Make `nano-brain init` parse its full flag set, and reject a `--root` that cannot be indexed.
4. Make npm postinstall fail actionably at unpublished versions, with integrity enforcement intact.

## Non-goals

- **`~/.nano-brain/memory` and `~/.nano-brain/sessions` MUST NOT be created and their roots MUST NOT become walkable.** That is the trigger, not the fix.
- Deleting logical-collection rows. Consumers filter on the names (`internal/server/handlers/wakeup.go:130`, `internal/mcp/tools.go:1791`) and deletion is irreversible.
- Restoring the six commands documented in `SKILL.md` but absent from the dispatcher. `git log -S 'case "<cmd>":' -- cmd/nano-brain/main.go` returns **0 commits** for all six (`update`, `embed`, `focus`, `graph-stats`, `symbols`, `impact`) — they were never implemented. The fix is to correct the docs.

## Decision 1 — Skip logical collections inside `triggerIncremental`, keyed on reserved names

**Chosen:** a guard at the top of the `for _, col := range targets` body in `triggerIncremental` (`internal/server/handlers/reindex.go:154`), before `walkCollectionFiles`, skipping collections named `memory` or `sessions`.

**Why this and not the alternatives:**

| Option | Force-wipe | Warning removed | Migration | Registration determinism |
|---|---|---|---|---|
| Do not register absent roots *(original proposal)* | **Breaks** — no row, so `triggerForceWipe` never resets chunks | yes | no | **No** — depends on incidental FS state |
| Filter in `collectionsToReindex` (`:330-354`) | **Breaks** — `targets` is computed once at `:83` and passed to *both* branches (`:94-95` force, `:98` incremental) | yes | no | yes |
| `path = ''` | intact | **No** — trades `root path inaccessible` for `disk walk returned empty for non-empty collection`, `indexed: 2003`, every reindex | one-statement | yes |
| `update_mode` sentinel | intact | yes | **Yes** — `NOT NULL DEFAULT 'auto'` (`migrations/00001_initial_schema.sql:64`), existing rows need backfill | yes |
| **Skip inside `triggerIncremental`** | **intact** — `triggerForceWipe` untouched | **yes — neither warning fires** | **none** | yes |

The chosen option is the only one satisfying all four constraints. It also leaves `internal/server/handlers/workspace_test.go:249-251` (exactly three `UpsertCollection` calls) green, because registration is not modified.

Keying on the reserved names adds no new coupling: `memory` and `sessions` are already load-bearing string constants in at least six places — `wakeup.go:130`, `mcp/tools.go:1791`, `ticket.go:31`, `workspace.go:87,173-174`, `harvest/engine.go:264`, `summarize/persist.go:128,174`.

**Paired edit, required regardless of mechanism:** `internal/server/handlers/workspace.go:171-176` builds the post-init watcher attach list from hardcoded literals, not from the DB, and attaches all three unconditionally at `:178-182`. `memory` and `sessions` must be dropped from that list. Without this they are watched immediately after init but not after a daemon restart — `cmd/nano-brain/main.go:532` and `internal/watcher/watcher.go:334-338` both already skip collections whose path fails `os.Stat`. Those two skips are evidence that the DB-only intent is already encoded in production code; the reindex walker is the only component that does not know it.

**Why the current state is not safe to leave alone.** Today no deletion occurs, but only because two conjuncts are accidentally false. The trigger condition is four conjuncts: (1) non-empty `path` — else `:290-292` early-returns; (2) the directory exists — else the stat guard at `:297-299` errors and the caller warns and continues at `:155-160`; (3) it contains ≥1 file ≤10 MiB (`maxIncrementalFileSize`, `:24`, applied at `:312`) — else the guard at `:182-189` fires, whose condition `len(diskFiles) == 0 && len(indexedRows) > 0` a single file defeats; (4) an incremental reindex runs. `walkCollectionFiles` (`:300-326`) applies **no ignore filter at all** — no `.nano-brainignore`, no `.gitignore` — so an ignored file still disarms the guard. On macOS, opening `~/.nano-brain/memory` in Finder writes a `.DS_Store` and satisfies conjunct 3; `~/.nano-brain` is an Obsidian vault on the reporting machine, so `memory/` is a visible, invitingly-named empty folder inside a vault the user already browses.

`triggerIncremental` is the **sole** mass-deletion vector: `cleanupDeletedDocument` (`internal/watcher/watcher.go:683-731`) and `cleanupIgnoredDocument` (`:639-678`) resolve one specific path each, and `cleanupPathPrefix` (`:736+`) deletes by prefix, which `summary://…` never matches.

## Decision 2 — AST-parse the dispatch switch in the test; do not refactor the dispatcher

**Chosen:** `cmd/nano-brain/ops_usage_test.go` parses `cmd/nano-brain/main.go` with `go/parser`, walks to the `switch` inside `func main`, and collects the `case` label string literals as the expected set. Assertions anchor on the command column of a usage line, not `strings.Contains`.

**Rejected:** a `[]struct{name, summary, run}` command table that `main` dispatches from and `printUsage` renders. It kills the bug class outright, but it is a 35-arm refactor of the dispatch switch — and `runXCmd` signatures are not uniform (`runServeCmd(args, configPath)` `main.go:78`, `runInitCmd(args, configPath)` `:93`, `runConfigCmd` `:132`, `runDoctorCmd` `:135` take `configPath`; ~30 others take only `args`), so it needs an adapter closure per entry or a signature change. That is the highest-regression-risk work in the change, undertaken for a help-text bug, on the branch that ships #615. It also cannot be a pure render: `ops.go:557` `(no command)`, `:559` `serve -d`, `:566-567` `config show`/`config check`, `:571` `(alias: ls)` are not dispatch cases and must survive, so the table needs a free-form preamble in which drift can still live. A CI gate that fails on drift is prevention for this class — you cannot merge with drift.

**Substring matching must be fixed in the same change**, not deferred: `get` is matched by `multi-get` (`ops.go:579`) and `config` by the `--config` global-flags line (`ops.go:596`). (`serve` is *not* matched by `service` — after `serv` comes `i`, not `e` — and `ops.go:558` has a literal `serve` line anyway.) Column anchoring requires the indentation at `ops.go:576-584` to be normalized first: those lines use three leading spaces where the rest use two.

**Reverse direction (root `SKILL.md` ⟷ dispatch) is a separate requirement, not the same test.** `printUsage` ⟷ dispatch is a Go test in one package; SKILL.md ⟷ dispatch is a markdown-table lint against a Go source file — different tooling and failure mode. Bundling them invites an implementer to do the easy half.

## Decision 3 — Validate `--root` at the HTTP handler, with `os.Stat` + `IsDir`

**Chosen:** validate in `internal/server/handlers/workspace.go`, after `filepath.Abs` (`:129`) and before `WorkspaceHash` (`:134`). Three distinct errors: path does not exist; path is a regular file; path is a directory that cannot be read.

**Why the handler and not the CLI:** it is the single choke point covering the CLI, the interactive wizard (`cmd/nano-brain/init.go:190-193`), the MCP tools, and raw API callers. A CLI-only check leaves the API hole open.

**Why validation is required at all:** `internal/storage/workspace.go:9-16` is `filepath.Abs` + `sha256` with no filesystem access, and the handler rejects only an empty `root_path` (`:125-127`). Once `--root` parses, `init --root /typo/path` registers a `code` collection whose root fails `reindex.go:297-299` on every reindex — reopening the exact defect Decision 1 closes, through a newly-working command.

**Remote-daemon constraint, considered and dismissed as a blocker:** `docs/SETUP_AGENT.md:393-409` documents a VPS topology where the daemon is remote. A local path sent to a remote daemon will now be rejected. That is correct: the watcher walks `col.Path` on the daemon's filesystem (`internal/watcher/watcher.go:587`), as does `walkCollectionFiles`, so a path the daemon cannot stat can never be indexed. A clear error beats a permanently-empty ghost workspace. The one ambiguous case — a shared mount present on both sides under different prefixes — is misconfiguration, and the same argument applies.

## Decision 4 — `init` parses once, both flag forms; copy the existing house pattern

**Chosen:** collapse `cmd/nano-brain/commands.go:19-40` and `:70-96` into a single pass handling `--`, `--yes`, `--root`, `--workspace`, `--json`, `--force`, accepting both `--flag value` and `--flag=value` for value-taking flags. Extract a pure `parseInitArgs(args) (initOpts, error)` so the parser is testable without `os.Exit` (currently called at `:34,38,76,84,94`).

**`=`-form support is copying the house pattern, not inventing one.** Seven commands already split on `=`: `cmd_get.go:27-28`, `cmd_tags.go:25-26`, `cmd_multi_get.go:26-27` (plus `--paths=` `:45`, `--ids=` `:54`), `cmd_backfill_summaries.go:34-35` (plus `--since=` `:37`), `cmd_reset_embeddings.go:27-28`, `cmd_workspace_remove.go:34-35`. `cmd_reset_embeddings.go:27-41` supports **both forms for the same flag** — exactly the shape needed here — and `ops.go:589` advertises `--workspace=<hash>` for that command. Adding `=` to `init` restores consistency; it does not break it.

The `--` case at `commands.go:21-27` already uses the correct `i++`-then-consume idiom, which is what makes the `--root` omission a bug rather than a design choice. `init -- opencode` must keep working.

## Decision 5 — npm: export a pure planner; preserve every integrity path

**Chosen:** extract `planDownload(version, assetName) → { candidates, message }` from `npm/postinstall.js` and export it (`module.exports` at `:373` currently omits `candidateTagsForVersion`, so the buggy function cannot be unit-tested at all). Detect a non-release placeholder before attempting any download and print the tags tried plus `install.sh`, `NANO_BRAIN_BIN`, and the source build as alternatives.

**Hard constraints on the refactor — the failure mode is folding a `SECURITY:` error into the new summary string:**
- `postinstall.js:329` rethrows `SECURITY:`-prefixed errors unwrapped from inside the candidate loop.
- `:340-343` deliberately does *not* wrap the API-fallback attempt, with the comment at `:341` explaining that a SECURITY error must propagate.
- `verifySHA256` `:164-175` `safeUnlink`s on mismatch; the comment at `:165-167` explains that an unrelated fs error must not downgrade a SHA mismatch.
- `main()` `:354-359` exits 1 on any `SECURITY:` message.

**The diagnostic MUST NOT recommend `NANO_BRAIN_SKIP_SHA_VERIFY`** (`:286-289`, used at `:314-319`). It is unrelated to a 404 and would be a security regression in guidance form — and `openspec/specs/skill-distribution-docs/spec.md` requires SKILL.md to document that variable, so a spec-blessed escape hatch is sitting in easy reach of a naive author.

**Scope correction:** the published npm path is not broken. `0.0.0-dev` is the on-master placeholder (`AGENTS.md:515`), rewritten by CI before publish (`.github/workflows/release.yml:96`) and gated by `scripts/check-npm-release.sh:166-193`. Registry check: `nano-brain` and `@nano-step/nano-brain` are both live at `2026.8.1201`, same repo and maintainer; that version has an all-digit 4-char patch, so it resolves on the first candidate tag. No README caveat will be added.

**`npm/run.js` is in scope**: `run.js:105-112` calls `ensureBinary()` and prints `err.message`, so changing the throw at `postinstall.js:345` changes what the lazy first-invocation path prints.

## Decision 6 — Wire the npm suite into CI

`.github/workflows/ci.yml:29-39` is Go-only: checkout, setup-go, build, `go test`. `npm/postinstall.test.js` (425 lines) and `npm/run.test.js` (167) have never run in CI. `docs/evidence/review-npm.md:21` recorded this and recommended `node --test npm/` as a follow-up; it was not done. Adding the test without the job repeats that. The job goes in this change.

The suite is offline-capable — `postinstall.test.js:36-44` drives a full `ensureBinary()` rejection by patching `os.platform`, `:18-32` stubs a binary at `binaryPath()`, `:67-77` forces `ECONNREFUSED` — so no network or published release is needed. **No assertion may depend on a real GitHub 404.**

## Risks

- **Decision 3 is a live API contract change.** Existing automation that registers a path the daemon cannot see now gets a 400. Mitigated by explicit scenarios and a release note.
- **Decision 1 is a name-keyed skip.** If a future workspace legitimately wants a disk-backed collection named `memory` or `sessions`, it is silently excluded. Accepted: those names are already reserved by six other call sites.
- **Decision 2 couples a test to source layout.** An AST test breaks if `main.go`'s dispatch moves. Accepted as cheaper than the refactor; the failure is a loud test error, not a silent drift.

## Open questions

- Root `SKILL.md` has no owning spec and does not ship via npm (`package.json` `files` = `['npm/', 'README.md', 'LICENSE']`). This change brings it under `cli-usage-parity`, but the question of which artifact *should* own it is left open.
- Fix #1 documents a command introduced by the branch this change sits on (`feat/615-daemon-service-install-ux`). If #615 is unmerged, the honest home for the help-text and recommendation-path work is #615's own `tasks.md` — whose task 7.1 (`openspec/changes/archive/2026-08-08-cross-platform-daemon-auto-start/tasks.md:45`) documented `service` in `README.md` and `docs/SETUP_AGENT.md` but not in `nano-brain help`. That is where the defect was born.
