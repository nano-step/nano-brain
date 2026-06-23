# Tasks: cfg-if-continuation-paths

## 1. Fix buildIf continuation path

- [ ] 1.1 Move `elseExits = b.relabelPreds(elseExits, decisionID)` inside `if alternative != nil` block in `buildIf` (js_cflow.go ~line 383)
- [ ] 1.2 Keep `elseExits = map[string]bool{decisionID: true}` for the no-else case WITHOUT relabeling

## 2. Add tests

- [ ] 2.1 Add test: guard clause with early return + happy path code
- [ ] 2.2 Add test: nested guard clauses
- [ ] 2.3 Add test: if without else, then-block does NOT return
- [ ] 2.4 Verify existing `TestJSControlFlowExtractor_IfOnlyBranchLabels` still passes

## 3. Verify and test

- [ ] 3.1 Build: `go build ./...`
- [ ] 3.2 Test: `go test -race -short ./internal/graph/...`
- [ ] 3.3 Manual test: dogfood on express-app `setUserEmail` — verify DB update, email check, commit all appear in CFG
