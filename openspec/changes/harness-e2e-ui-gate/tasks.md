## 1. Script implementation

- [x] 1.1 Create `scripts/smoke-ui.sh` with shebang, set -euo pipefail, color codes
- [x] 1.2 Implement build step: `go build -o /tmp/nano-brain-smoke/nano-brain ./cmd/nano-brain` (skip if binary newer than source)
- [x] 1.3 Implement server start: background process with --serve-only --unsafe-no-auth on port 3199
- [x] 1.4 Implement health-wait loop (15 seconds max)
- [x] 1.5 Implement /ui/ check: fetch + assert Content-Type + parse script/link tags
- [x] 1.6 Implement asset loop: for each parsed URL, curl + assert status + Content-Type + size
- [x] 1.7 Implement teardown: kill PID + wait
- [x] 1.8 Implement output formatting: header, per-check line, final PASS/FAIL marker
- [x] 1.9 chmod +x scripts/smoke-ui.sh

## 2. HARNESS.md updates

- [x] 2.1 Add `smoke:ui` row to Validation Ladder code block
- [x] 2.2 Add Lane × smoke:ui column to lane matrix
- [x] 2.3 Add web-change annotation to Change Types section
- [x] 2.4 Add new subsection documenting `smoke:ui` invocation + evidence format

## 3. harness-check.sh updates

- [x] 3.1 Add new check `3.8 smoke:ui evidence` to `phase_pre_merge`
- [x] 3.2 Compute web-touching diff via `git diff --name-only origin/b-main...HEAD`
- [x] 3.3 Check evidence file existence + freshness (newer than scripts/smoke-ui.sh)
- [x] 3.4 Grep evidence for `smoke:ui PASS` marker
- [x] 3.5 Add appropriate add_check PASS/FAIL/SKIP

## 4. Self-test (eat own dogfood)

- [x] 4.1 Run `./scripts/smoke-ui.sh` against this PR's branch
- [x] 4.2 Capture output to `docs/evidence/harness-e2e-ui-gate/smoke-ui-output.log`
- [x] 4.3 Verify log ends with `smoke:ui PASS`
- [x] 4.4 Verify `harness-check.sh pre-merge` correctly SKIPs check 3.8 (this PR touches scripts/ + docs/, not web/)

## 5. Verification

- [x] 5.1 `bash -n scripts/smoke-ui.sh` (syntax check)
- [x] 5.2 `shellcheck scripts/smoke-ui.sh` (if shellcheck available, otherwise SKIP)
- [x] 5.3 `./scripts/smoke-ui.sh` against dev binary — exits 0, log contains PASS
- [x] 5.4 `./scripts/harness-check.sh pre-merge --no-color` — all checks PASS or SKIP

## 6. PR + Review

- [x] 6.1 Commit: `feat(harness): add smoke:ui validation layer + pre-merge gate (#285)`
- [x] 6.2 Push branch
- [x] 6.3 Open PR with smoke:ui evidence linked
- [x] 6.4 Gemini triage at `docs/evidence/harness-e2e-ui-gate/gemini-triage.md`
- [x] 6.5 Address findings
- [x] 6.6 Squash merge with delete-branch

## 7. Archive + Release

- [x] 7.1 Pull merged b-main
- [x] 7.2 `openspec archive harness-e2e-ui-gate --yes`
- [x] 7.3 Tag next v2026.6.X + push
