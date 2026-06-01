# Tasks

## 1. Implementation

- [x] 1.1 Change `DefaultConfig().TargetSize` from 3600 → 2600 in `internal/chunk/chunk.go`
- [x] 1.2 Added invariant docstring on `Config` and `DefaultConfig` documenting the contract

## 2. Test updates

- [x] 2.1 No test code needed changes — all #297 tests use `DefaultConfig().TargetSize + searchWindow/2` (parameterized; auto-adjusts to 3000)
- [x] 2.2 `TestSplit_HardSplit_Pathological` still passes; 1MB → more chunks (~385 now vs 278 before), all bounded
- [x] 2.3 New `TestSplit_DefaultConfig_MatchesEmbedBudget` — guards the contract invariant against future drift
- [x] 2.4 New `TestSplit_TraceJSON_NoOversize` — reproduces user's 3671-char scenario

## 3. Verification

- [x] 3.1 `go build ./...` exit 0
- [x] 3.2 `go vet ./...` clean
- [x] 3.3 `go test -race -short ./internal/chunk/...` all PASS (15 chunker tests + fuzz)
- [x] 3.4 `go test -race -short ./...` full suite PASS

## 4. Evidence

- [x] 4.1 `docs/evidence/fix-chunker-embed-contract/output.txt` — all `over_3000=0` including user's exact 3671-char shape

## 5. PR + Review

- [ ] 5.1 Commit + push `fix/300-chunker-embed-contract`
- [ ] 5.2 Open PR with `Closes #300`
- [ ] 5.3 Gemini review triage
- [ ] 5.4 Merge `--squash --delete-branch`

## 6. Archive + Release

- [ ] 6.1 `openspec archive fix-chunker-embed-contract --yes`
- [ ] 6.2 Verify auto-tag + npm publish + live install
