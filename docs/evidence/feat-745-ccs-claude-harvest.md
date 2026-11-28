# Evidence — #745 CCS Claude session harvesting

Change type: feature. Fixture smoke used isolated PostgreSQL schemas in `nanobrain_test` only. The dev database and port 3100 were not used. All fixture workspace paths are synthetic `/fake/...` values or temporary test directories.

## Fixture smoke results

The dynamic harvest tests fabricate CCS-style `instances/<profile>/projects/<encoded-workspace>/<session>.jsonl` layouts, run the real `ClaudeCodeDiscoveryHarvester`, persist into PostgreSQL, and verify document content plus BM25 visibility.

```text
=== RUN   TestClaudeDiscovery_FirstSessionAfterStartup
--- PASS: TestClaudeDiscovery_FirstSessionAfterStartup (0.37s)
=== RUN   TestClaudeDiscovery_WorkspaceRegisteredAfterStartup
--- PASS: TestClaudeDiscovery_WorkspaceRegisteredAfterStartup (0.32s)
=== RUN   TestClaudeDiscovery_GlobDedupesExplicitDuplicate
--- PASS: TestClaudeDiscovery_GlobDedupesExplicitDuplicate (0.35s)
=== RUN   TestClaudeDiscovery_RejectsTranscriptCwdMismatch
--- PASS: TestClaudeDiscovery_RejectsTranscriptCwdMismatch (0.34s)
=== RUN   TestClaudeDiscovery_EncodedCollisionUsesTranscriptCwd
--- PASS: TestClaudeDiscovery_EncodedCollisionUsesTranscriptCwd (0.32s)
=== RUN   TestClaudeDiscovery_SymlinkAliasedWorkspaceDiscoversCanonicalLeaf
--- PASS: TestClaudeDiscovery_SymlinkAliasedWorkspaceDiscoversCanonicalLeaf (0.29s)
PASS
ok      github.com/nano-brain/nano-brain/cmd/nano-brain      3.637s
```

`TestClaudeDiscovery_FirstSessionAfterStartup` also executes a BM25 query for the fixture token and asserts at least one row, proving the harvested session is query-visible.

Focused detection/init/render/encoding tests:

```text
--- PASS: TestDetectClaudeCodeStoragePatterns_DefaultCCSRoot
--- PASS: TestDetectClaudeCodeStoragePatterns_CCSDirAndCCSHome
--- PASS: TestDetectClaudeCodeStoragePatterns_StorageDirNeedNotExist
--- PASS: TestDetectClaudeCodeStoragePatterns_ExplicitEnvRootNeedNotExist
--- PASS: TestDetectClaudeCodeStoragePatterns_RelativeCCSDirIgnored
--- PASS: TestDetectClaudeCodeStoragePatterns_PlatformParentExists
--- PASS: TestDetectClaudeCodeStoragePatterns_DedupesEquivalentCCSRoots
--- PASS: TestApplyClaudeCodeStorageAutoDetection_GatesAndPopulates
--- PASS: TestBuildRenderedConfig_EnablesClaudeHarvestWhenCCSDetected
--- PASS: TestBuildRenderedConfig_NoClaudeStorageLeavesHarvestDisabled
--- PASS: TestRenderConfig_ClaudeCodeEnabledConditional
--- PASS: TestExistingClaudeBaseDirsDedupesSymlinkAliases
--- PASS: TestSameFilesystemPath_NormalizesLexicalAndSymlinkPaths
--- PASS: TestEncodeClaudeProjectsDirMatchesClaudeCharacterRules
```

## Validation ladder

```text
CGO_ENABLED=0 go build ./...                                      PASS
go vet ./cmd/nano-brain/... ./internal/config/... \
  ./internal/harvest/... ./internal/server/...                   PASS
go test -race -short ./...                                       PASS
go test -race ./cmd/nano-brain -run 'TestClaudeDiscovery' -count=1  PASS
git diff --check                                                 PASS
(cd .opencode && openspec validate "feat-745-ccs-claude-harvest" --strict)  PASS
```

`golangci-lint` is not installed on this machine, so the repository Makefile lint lane could not be run locally; `go vet` was run instead.

## Real CCS transcript E2E

A server-level E2E check against privacy-safe copies of real CCS transcripts is documented in `docs/CCS_HARVESTING.md`. It exercised zero-config auto-detection, API workspace registration, manual `POST /api/harvest`, dynamic addition of transcripts after startup, PostgreSQL persistence, and BM25 visibility. The run initially exposed a typed-nil embed-enqueuer panic in BM25-only mode; `internal/harvest/runner.go` now normalizes typed-nil enqueuers and the rerun completed with 7/7 unique sampled sessions persisted, zero harvest errors, and search visibility.

## Format verification

The encoder implementation was checked against the locally installed Claude Code 2.1.332 bundle. The bundle contains the current project-dir rule: replace every non-alphanumeric character with `-`, cap at 200 UTF-16 units, and append `-` plus a base-36 Java-style string hash for longer paths. Hard-coded vectors cover spaces/dots/underscores, Windows separators, non-BMP UTF-16 behavior, the 200-unit boundary, and an over-200 hash suffix.

## Independent review disposition

Two independent read-only reviews were completed after implementation. Their blocking/actionable findings were addressed as follows:

- startup auto-detection gating is now extracted into `applyClaudeCodeStorageAutoDetection` and directly tested;
- Codex PR review triage is recorded in `docs/evidence/review-745-codex.md`; its valid P2 first-install-after-startup finding was fixed by retaining stable future Claude/CCS runtime patterns;
- long-path truncation/hash encoding is implemented and tested;
- symlink-canonical workspace spellings are probed during discovery and covered by an integration fixture;
- file-backed config and effective harvester config are separated so auto-detected patterns do not create false reload diffs;
- generated-template/custom-root examples, migration visibility wording, and stale documentation descriptions were corrected.

Residual non-blocking deferrals:

- cwd-mismatched collision files can still be reconsidered on later harvest cycles because no persistent negative cache was added; this wastes some work but does not affect correctness;
- `/api/status` still exposes the pre-existing `session_dirs` resolved-shape semantics rather than a separate raw-pattern field;
- historical pre-current Claude leaf encodings and case-insensitive APFS spelling variants are not probed.

These deferrals do not contradict the approved scenarios: they affect operational efficiency, observability polish, or legacy/case variants outside the issue's current-layout CCS flow. First-use-after-startup for standard Claude and default CCS locations is supported; only custom per-invocation CCS `--config-dir` layouts still require explicit configuration.
