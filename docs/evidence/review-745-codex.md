# Codex Review — PR #749 / Issue #745

Reviewer: OpenAI Codex CLI (`codex review --base origin/master`), non-interactive read-only review passes.
Runtime evidence: codex-cli 0.172.0; sessions `01a21df7-bee1-7d93-ba7a-bf81df1b9c25`, `01a21dfd-7895-79b3-9a05-fc149c745cac`, `01a22072-db49-7ae2-b866-16150f072e0e`, `01a22076-c056-7901-89ce-abf5ac138793`, and final pass `01a2207e-c77f-7051-a0ab-cc86d53810de`; raw output captured locally under `/tmp/codex-review-745*.raw` during the runs.
Review Verdict: PASS

Final re-review output after the collision fix: "The review did not identify a discrete, actionable defect introduced by the patch."

## Findings and triage

| Finding | Severity | Disposition | Evidence |
|---|---:|---|---|
| Default CCS root created after daemon startup is omitted from startup patterns, so dynamic rediscovery cannot see it until restart | P2 | VALID — fixed | `cmd/nano-brain/detect.go` skipped a non-existent default root; runtime startup patterns are fixed in `cmd/nano-brain/claudecode_init.go` |
| Explicit `CLAUDECODE_STORAGE_DIR` created after daemon startup is dropped because it does not exist yet | P2 | VALID — fixed | explicit operator-configured storage is retained before existence in `cmd/nano-brain/detect.go`; covered by `TestDetectClaudeCodeStoragePatterns_StorageDirNeedNotExist` |
| Cwd-less transcripts in an encoded-leaf collision can be copied into multiple workspace memories | P1 | VALID — fixed | discovery now groups workspaces by physical encoded leaf; ambiguous cwd-less transcripts fail closed in `internal/harvest/claudecode.go` and `internal/harvest/claudecode_discovery.go`; covered by `TestClaudeDiscoveryCollisionAttribution` |

## Fix summaries

### Future storage discovery

Runtime auto-detection keeps evidence-based detection order and appends stable standard future install patterns (platform Claude project roots plus default `~/.ccs/instances/*/projects`) after deduplication. Explicit `CLAUDECODE_STORAGE_DIR` is treated as operator intent and is also retained before the directory exists. Init-time detection remains evidence-based, so machines with no Claude/CCS storage still render Claude harvesting as disabled rather than falsely treating absent storage as detectable.

Regression coverage:

- `TestApplyClaudeCodeStorageAutoDetection_GatesAndPopulates` asserts the future default CCS glob is retained.
- `TestDetectClaudeCodeStoragePatterns_StorageDirNeedNotExist` asserts explicit storage is retained before first creation.
- `TestBuildRenderedConfig_NoClaudeStorageLeavesHarvestDisabled` continues to prove that absent storage does not enable Claude harvesting during init.
- Existing dynamic first-session integration coverage proves that a configured glob with no initial matches is re-expanded on later cycles.

### Encoded-leaf collision attribution

Dynamic discovery now groups all registered workspaces that encode to the same physical leaf before instantiating leaf harvesters. Modern transcripts are still routed by explicit transcript `cwd`. A cwd-less transcript remains legacy-ingestible only when the leaf maps to one registered workspace; when multiple distinct workspaces collide, it is skipped fail-closed rather than copied into every candidate workspace memory.

Regression coverage:

- `TestDiscoverClaudeLeafBindingsGroupsEncodedCollisions` and `TestDiscoverClaudeLeafBindingsKeepsDistinctLeavesSeparate` cover grouping behavior.
- `TestClaudeDiscoveryCollisionAttribution` uses the real PostgreSQL schema to prove cwd-less collision transcripts are absent from both colliding workspaces while cwd-bearing transcripts reach only the matching workspace.
- `TestClaudeDiscovery_EncodedCollisionUsesTranscriptCwd` continues to prove modern colliding transcripts are routed by `cwd`.
