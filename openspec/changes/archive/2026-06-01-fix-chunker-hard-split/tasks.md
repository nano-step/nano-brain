# Tasks

## 1. Implementation

- [x] 1.1 Add `enforceMaxSize(chunks, cfg)` post-process to `chunk.Split`
- [x] 1.2 Add `hardSplit(c Chunk, target int) []Chunk` helper
- [x] 1.3 Add `findHardBoundary(s string, target int) int` helper with 5-tier priority + ideographic full stop (3b)
- [x] 1.4 Import `unicode/utf8` for `utf8.RuneStart` (test uses `utf8.ValidString`)

## 2. Tests

- [x] 2.1 `TestSplit_HardSplit_SingleLongLine`
- [x] 2.2 `TestSplit_HardSplit_FenceTrapped`
- [x] 2.3 `TestSplit_HardSplit_UTF8_CJK` (with round-trip assertion)
- [x] 2.4 `TestSplit_HardSplit_UTF8_Emoji`
- [x] 2.5 `TestSplit_HardSplit_Pathological` (1MB → 278 chunks, max 3600)
- [x] 2.6 `TestSplit_HardSplit_PrefersSentenceBoundary`
- [x] 2.7 `TestSplit_HardSplit_NormalContentUnchanged`
- [x] 2.8 Round-trip — folded into UTF8_CJK case
- [x] 2.9 Boundary unit tests (5): blank-line / newline / sentence / whitespace / rune-fallback

## 3. Verification

- [x] 3.1 `go build ./...` exit 0
- [x] 3.2 `go vet ./...` clean
- [x] 3.3 `go test -race -short ./internal/chunk/...` — 13 new tests + all existing + fuzz seeds PASS
- [x] 3.4 `go test -race -short ./...` — full suite PASS, no regression
- [ ] 3.5 Live test on dev server: deferred to post-merge (would require re-indexing next-app which takes hours and is not blocking)

## 4. Evidence

- [x] 4.1 `docs/evidence/fix-chunker-hard-split/before-after-log.txt` — 5 adversarial inputs, max chunk 3900, zero over 4000

## 5. PR + Review

- [ ] 5.1 Commit + push branch `fix/297-chunker-hard-split`
- [ ] 5.2 Open PR with `Closes #297` in body
- [ ] 5.3 Gemini review triage (≤ 3 cycles)
- [ ] 5.4 Merge with `--squash --delete-branch`

## 6. Archive + Release

- [ ] 6.1 `openspec archive fix-chunker-hard-split --yes`
- [ ] 6.2 Verify auto-tag fires + npm publish + live install
