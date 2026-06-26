## 1. Diagnostics and reconciliation foundation

- [x] 1.1 Diagnose actual stored graph edges for failing Rails benchmark nodes (`BillingWorker#perform`, `StoryPrintPerformer`, `Story#create_print_orders`, `DropboxUploadManager`) and record only sanitized source/target shape evidence.
- [x] 1.2 Identify the smallest shared location for node candidate expansion used by trace, impact, and graph traversal.
- [x] 1.3 Implement exact-first expansion for `file::symbol`, `Class#method`, bare class, and bare method inputs where diagnostics show traversal naming is the blocker.
- [x] 1.4 Add fanout guards for generic Ruby/Rails names to avoid noisy traversal, using `8` as the default maximum candidate expansion to match flow builder behavior.
- [x] 1.5 Add unit tests for Ruby node expansion with file-qualified and bare inputs.

## 2. Trace and impact score lift

- [x] 2.1 Preserve existing trace exact/symbol source lookup behavior and extend it only where diagnostics show missing multi-hop bare-callee reconciliation.
- [x] 2.2 Update trace traversal so bare callee targets can continue to matching file-qualified source nodes on later hops when matching graph edges exist.
- [x] 2.3 Update impact traversal to use symbol-aware target expansion before querying incoming edges.
- [x] 2.4 Add SQL/query helper coverage for symbol-aware target matching in both `GetImpactorsByTargets` and `GetImpactors`, regenerating sqlc if SQL changes are made.
- [x] 2.5 Add tests showing `BillingWorker#perform`-style and `Story#create_print_orders`-style inputs return non-empty traversal when matching graph edges exist.

## 3. Rails flow entry and Ruby symbol coverage

- [x] 3.1 Add non-HTTP class/job/service entry fallback for Rails flow traversal when no HTTP entry matches.
- [x] 3.2 Extend Ruby symbol extraction to emit `KindConst` for constant assignments.
- [x] 3.3 Add tests for `STATUS_ORDER_*`-style constants and concern files.
- [x] 3.4 Verify symbol extraction remains unchanged for existing methods/classes/modules except for additional constants.

## 4. Benchmark evidence and privacy

- [x] 4.1 Run the Rails capability benchmark before implementation and record score-only baseline evidence.
- [x] 4.2 Run the Rails capability benchmark after implementation and confirm overall score is at least 0.35 or document remaining blockers.
- [x] 4.3 Ensure `results_current.json`, real workspace hashes, real private workspace names, and private filesystem paths are not committed.
- [x] 4.4 Update benchmark docs only with privacy-safe instructions and score summaries.

## 5. Validation and harness

- [x] 5.1 Run `go build ./... && go test -race -short ./...`.
- [x] 5.2 Run targeted graph/trace/impact/symbol tests for modified packages.
- [x] 5.3 Run `openspec validate improve-rails-capability-score --strict --no-interactive`.
- [x] 5.4 Run `./scripts/harness-check.sh in-progress --issue 489 --no-color` after proposal updates and again after implementation work changes tracked files.
- [x] 5.5 Save review/evidence artifacts before PR per harness rules.
