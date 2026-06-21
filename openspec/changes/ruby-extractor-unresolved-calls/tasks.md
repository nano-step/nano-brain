## 1. Extractor Fix

- [ ] 1.1 Remove the `if !methodNames[callee] { continue }` guard in `extractCalls` (ruby_extractor.go ~line 186-188)
- [ ] 1.2 Run `go test -race -short ./internal/graph/...` — expect 1 test to fail (NoCrossFileCalls)

## 2. Test Updates

- [ ] 2.1 Invert `TestRubyGraphExtractor_NoCrossFileCalls` to assert cross-file calls DO appear
- [ ] 2.2 Verify all other existing tests still pass
- [ ] 2.3 Run full test suite: `go test -race -short ./...`

## 3. Verification

- [ ] 3.1 Rebuild binary, restart test server, reindex Phil workspace
- [ ] 3.2 Verify controller files now have outgoing call edges via `memory_graph`
- [ ] 3.3 Verify Phil-timeshel flows show 5+ nodes per controller action
- [ ] 3.4 Commit, push, create PR, run harness check, merge
