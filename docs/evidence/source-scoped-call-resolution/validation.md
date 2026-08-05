# Task 1.1 Validation: Source-Scoped Call Resolution

## Commands

| Command | Result |
| --- | --- |
| `openspec status --change source-scoped-call-resolution --json` | Exit 0. `schemaName` is `spec-driven`; `isComplete` is `true`; proposal, design, specs, and tasks are all `done`; `applyRequires` is `["tasks"]`. |
| `openspec validate --change source-scoped-call-resolution --strict` | Exit 1 because this installed CLI does not support `--change` for `validate`: `error: unknown option '--change'`. The command was run as specified before using the CLI-supported positional form below. |
| `openspec validate source-scoped-call-resolution --strict` | Exit 0: `Change 'source-scoped-call-resolution' is valid`. `openspec validate --help` confirms the positional `[item-name]` syntax. |
| `gh pr view 504 --repo nano-step/nano-brain --json state,mergeable,files` | Exit 0. `state` is `OPEN`, `mergeable` is `CONFLICTING`, and the response lists 18 changed files. The ownership conclusion is recorded in `prework-ownership.md`. |
| Repository privacy scan over the new OpenSpec and durable evidence paths | Exit 1 with no output, meaning the scoped committed docs contain none of the prohibited private workspace identifiers or paths. The sensitive search terms are intentionally not reproduced in committed evidence. |
| `git diff --check` | Exit 0 with no output. |
| `./scripts/harness-check.sh in-progress --json` | Exit 0. Gate status `PASS`: branch, active GSD phase, validation ladder, and self-review-evidence checks all passed. |

Gate-completion additions:

- `docs/evidence/deep-design-source-scoped-call-resolution.md` records the
  bounded canonical-target, resolver/catalog, consumer, lifecycle, privacy,
  scope, and risk decisions.
- `docs/stories/609-source-scoped-call-resolution.md` is the high-risk story
  packet using the repository template.
- The intake records `$omo:start-work issue 609` as the planning/execution
  go-ahead without claiming a separate human product approval.

No product code, tests, database, server, GitHub issue, GitHub pull request, or
stale #501 artifact was changed by this task.

## Implementation and delivery evidence (2026-08-05)

| Check | Result |
| --- | --- |
| `CGO_ENABLED=0 go build ./... && go test -race -short ./...` | PASS. |
| Targeted source-scoped storage integration (`TestSourceScopedReaderQueriesKeepCanonicalExactAndDropUnresolved`, `TestGraphImpactQueriesMatchSymbolPart`) | PASS against `nanobrain_test`; canonical exact lookups, legacy bare fallback, source-scope isolation, unresolved filtering, and graph counts are covered. |
| Focused graph, flow, symbol, codesummary, handler, watcher, and storage controls | PASS, including canonical collision isolation, exact reader matching, unresolved exclusion, Ruby compatibility, source replacement, and the AST import-name `from` regression. |
| `go test -race -count=1 -tags=integration ./internal/graph ./internal/storage ./internal/watcher ./internal/flow ./internal/codesummarize` | PASS. |
| Streamable HTTP MCP integration and server route/health controls | PASS through `nanobrain_test`-backed test fixtures. |
| Full repository integration suite | FAIL from pre-existing/unrelated workspace-resolution read-only assertions, harvest/summary compatibility assertions, and the benchmark package's absent live `:3199` server. The issue-related graph/storage/watcher/flow/codesummary integration packages passed. |
| Live REST/MCP `:3199` probe | BLOCKED: `http://127.0.0.1:3199/api/status` had no listener; repository policy prohibits starting the server in this environment. Port 3100 was not touched. |
| Applicable capability benchmark | SKIP with exit 0 because the required isolated `:3199` server was unreachable. |
| `graphify update .` | PASS; code graph refreshed locally. Generated `graphify-out/` is excluded from the change. |
