# Tasks — fix-install-ux-regressions

Ordered by dependency. Group 1 closes the data-loss path and goes first.

## 1. Logical-collection isolation

- [x] 1.1 Add a reserved-name helper identifying `memory` and `sessions` as logical (DB-backed) collections, in `internal/server/handlers/reindex.go`.
- [x] 1.2 Guard the top of the `for _, col := range targets` body in `triggerIncremental` (`internal/server/handlers/reindex.go:154`) to `continue` for logical collections, before `walkCollectionFiles` is called. Do not touch `collectionsToReindex` and do not touch `triggerForceWipe`.
- [x] 1.3 Drop `memory` and `sessions` from the hardcoded `colSpec` watcher-attach list at `internal/server/handlers/workspace.go:171-176` so init-time attach matches the startup skip at `cmd/nano-brain/main.go:532`.
- [x] 1.4 Add a regression test: a collection with a valid, non-empty root whose indexed documents carry `summary://…` source paths deletes **zero** documents on incremental reindex. This is the test that would have caught the trap.
- [x] 1.5 Add a test asserting incremental reindex on a workspace with a missing `sessions` root logs neither `root path inaccessible` nor `disk walk returned empty for non-empty collection`.
- [x] 1.6 Add a test asserting force-wipe still calls `ResetAndReturnChunkIDsByCollection` for `sessions`.
- [x] 1.7 Verify `internal/server/handlers/workspace_test.go:215-262` still passes unmodified — registration behavior must be unchanged.
- [x] 1.8 Add a release note: operators must not create `~/.nano-brain/memory` or `~/.nano-brain/sessions`, and must not otherwise make those roots walkable.

## 2. Root validation at the registration handler

- [x] 2.1 In `internal/server/handlers/workspace.go`, after `filepath.Abs` and before `WorkspaceHash`, reject a `root_path` that fails `os.Stat`, is not a directory, or cannot be read — with a distinct message per case.
- [x] 2.2 Add handler tests for nonexistent path, regular file, and unreadable directory, each asserting the status code and the message.
- [x] 2.3 Add a test asserting a relative root is stored as its absolute resolved form.
- [x] 2.4 Document the `POST /api/v1/init` contract change (200 → 400 for unusable roots) in the release note, calling out the remote-daemon topology explicitly.

## 3. `init` argument parsing

- [ ] 3.1 Extract `parseInitArgs(args) (initOpts, error)` from `cmd/nano-brain/commands.go`, handling `--`, `--yes`, `--root`, `--workspace`, `--json`, `--force` in one pass, accepting both `--flag value` and `--flag=value`. Copy the dual-form shape already used at `cmd/nano-brain/cmd_reset_embeddings.go:27-41`.
- [ ] 3.2 Replace the two-pass structure at `commands.go:19-40` and `:70-96` with a single call to the new parser; keep `os.Exit` at the call site only.
- [ ] 3.3 Add a table-driven `TestParseInitArgs` covering every form in the spec, including `-- opencode` and `--yes --root <path>`.
- [ ] 3.4 Verify `nano-brain init --root=<path>` — the form documented in `SKILL.md:56`, `docs/SETUP_AGENT.md:279,410`, and `CHANGELOG.md:541` — now succeeds.

## 4. Usage parity

- [ ] 4.1 Normalize the three-space indentation at `cmd/nano-brain/ops.go:576-584` to two spaces so a column matcher can be strict.
- [ ] 4.2 Add a `service` line to `printUsage()` describing the managed-service subcommands.
- [ ] 4.3 Rewrite `cmd/nano-brain/ops_usage_test.go` to derive its expected set by parsing the dispatch switch in `cmd/nano-brain/main.go` with `go/parser`, replacing the hand-maintained `dispatchedCommands` slice.
- [ ] 4.4 Replace the `strings.Contains` assertion with a command-column match, and add the reverse assertion: every documented command is dispatched.
- [ ] 4.5 Verify the rewritten test fails when `service` is removed from `printUsage()`, and fails when a fake `case "zzz":` is added to the dispatch switch.

## 5. Skill command table

- [ ] 5.1 Remove `update`, `embed`, `focus`, `graph-stats`, `symbols`, and `impact` from the command table in root `SKILL.md`. `git log -S` confirms none was ever implemented, so this is a docs correction, not a removal of shipped behavior.
- [ ] 5.2 Correct the `--root=<path>` row's description — it registers or re-registers a workspace; it does not "re-index a workspace".
- [ ] 5.3 Add a check that the skill command table names only dispatched commands, runnable in CI.
- [ ] 5.4 Align the package name used in skill examples with the scoped form used by `README.md:68` and `cmd/nano-brain/client_helpers.go:69`.

## 6. Service recommendation path

- [ ] 6.1 In the interactive wizard's serve step (`cmd/nano-brain/init_serve.go`), offer `nano-brain service install` when the platform service backend reports usable; fall back to the current `serve -d` suggestion otherwise.
- [ ] 6.2 Extend the wizard's closing summary (`cmd/nano-brain/init.go:206-214`) to state whether the daemon will survive a reboot.
- [ ] 6.3 Extend `TestSuggestStartCommand` (`cmd/nano-brain/commands_test.go:284`) and `TestRunInteractiveInit_Summary` (`cmd/nano-brain/init_test.go:336`) to assert the supervised option is surfaced on a service-capable platform and absent otherwise.

## 7. npm diagnostic

- [ ] 7.1 Extract and export `planDownload(version, assetName)` from `npm/postinstall.js`, returning the candidate tags and the guidance message.
- [ ] 7.2 Detect a version from which no release tag can be derived before attempting any download, and emit the guidance message naming the installer script, `NANO_BRAIN_BIN`, and the source build.
- [ ] 7.3 Preserve every integrity path unchanged: the unwrapped `SECURITY:` rethrow inside the candidate loop, the deliberately unwrapped API-fallback attempt, `safeUnlink` on mismatch, and the non-zero exit. No `SECURITY:` error may be folded into the tags-tried summary.
- [ ] 7.4 Ensure the guidance never mentions the checksum-skip environment variable.
- [ ] 7.5 Verify `npm/run.js`'s lazy first-invocation path prints the new message, and update `npm/run.test.js` accordingly.
- [ ] 7.6 Add offline tests to `npm/postinstall.test.js` for the placeholder version, a short-patch version, a canonical four-digit-patch version, and a checksum mismatch that must still hard-fail.

## 8. CI

- [ ] 8.1 Add a Node job to `.github/workflows/ci.yml` running `node --test npm/`.
- [ ] 8.2 Confirm the job fails the build when a postinstall test fails, and that no test depends on network access or a published release.

## 9. Validation

- [ ] 9.1 `go build ./...` and `go test -race -count=1 ./...` pass.
- [ ] 9.2 `node --test npm/` passes locally and in CI.
- [ ] 9.3 `openspec validate fix-install-ux-regressions --strict` passes.
- [ ] 9.4 Manually verify on a live daemon: register a fresh workspace, run an incremental reindex, and confirm the logs contain neither collection warning and that `sessions`/`memory` document counts are unchanged.
