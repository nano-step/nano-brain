# Review Gate Evidence — PR #265

**PR:** [#265 feat(web): Story 9.8 — Settings + Cmd+K + non-loopback banner + destructive confirm + a11y](https://github.com/nano-step/nano-brain/pull/265)
**Story:** `9.8-settings-cmdk`
**Issue:** #251

## Review Method

5-agent parallel review (review-work skill):
- Agent 1: Goal & constraint verification (Oracle)
- Agent 2: Code quality review (Oracle)
- Agent 3: Security review (Oracle)
- Agent 4: QA hands-on execution (explore)
- Agent 5: Context mining (explore)

## Agent Verdicts

| Agent | Verdict | Notes |
|---|---|---|
| 1. Goal Verification | FAIL (content mismatch) | Oracle reviewed reconstructed summary, not actual files. All 3 "blocking issues" were false positives verified against real code. |
| 2. Code Quality | FAIL (content mismatch) | Oracle reviewed reconstructed summary. All 3 "blocking" findings false positives: routes correct for basepath '/ui', `apiGetJSON` used (not `apiFetch`), `ConfigResponse` defined in types.ts. |
| 3. Security | PASS | No issues. Severity: LOW (localStorage search terms — acceptable). |
| 4. QA Execution | PARTIAL | Read actual files. 2 real findings assessed below. |
| 5. Context Mining | PASS | No missed requirements, deferred items properly documented. |

## False-Positive Resolution (Agents 1 & 2)

Oracle agents received summarised file content in their prompts, not direct file access. Cross-checked against actual files:

| Finding | Actual code | Status |
|---|---|---|
| "workspaceHash='default'" | `getCurrentWorkspace()` called at `SettingsPanel.tsx:362` | FALSE_POSITIVE |
| "Missing Actions group" | Group present at `CommandPalette.tsx:255` | FALSE_POSITIVE |
| "apiFetch used instead of apiGetJSON" | `apiGetJSON` imported at `NonLoopbackBindBanner.tsx:2` | FALSE_POSITIVE |
| "Routes prefixed /ui/" | Routes are `/dashboard`, `/memory` etc — correct for `basepath: '/ui'` at `router.tsx:76` | FALSE_POSITIVE |
| "ConfigResponse missing" | Defined at `types.ts:230` | FALSE_POSITIVE |

## Real Findings (Agent 4)

### Finding 1: All fields patched on save (not delta-only)

**Severity:** ACKNOWLEDGED (not blocking)
**Evidence:** `SettingsPanel.tsx:216-239` sends all 12 fields on submit.
**Assessment:** Spec (line 195 of web-ui-app/spec.md) says "render a form for safe-patch fields" — no delta-only requirement. Submit button gated by `isDirty` so save only fires when something changed. All POSTs are idempotent. Functionally correct.
**Action:** ACKNOWLEDGED — acceptable pattern.

### Finding 2: Doctor hints use custom expand button instead of `<details>`

**Severity:** ACKNOWLEDGED (not blocking)
**Evidence:** `SettingsPanel.tsx:58-67` — custom button + conditional div.
**Assessment:** Spec line 211 says "failed checks expand to show the hint text" — no `<details>` element required. Custom pattern has `aria-expanded` attribute and passes WCAG. Functionally equivalent.
**Action:** ACKNOWLEDGED — acceptable pattern.

### Finding 3: Missing debounce timing test for symbol search

**Severity:** MINOR — FIXED
**Evidence:** `CommandPalette.tsx:121` has 150ms debounce; no test verified timing.
**Fix:** Added test `'debounces symbol search'` at `CommandPalette.test.tsx:92-110` using `vi.useFakeTimers()`.
**Result:** 75 tests passing.

## Pre-Merge Gate Overrides

### [HARNESS-OVERRIDE]: integration test failures are pre-existing on b-main

Gate 3.3 (`go test -race -tags=integration`) fails due to:
1. `internal/search/isolation_test.go` — `HybridSearch` signature mismatch (pre-existing build failure on b-main, verified)
2. `TestEventsIntegration_ReindexPublishesSequence` — flaky timing test (pre-existing failure on b-main, verified)

Both verified to fail identically on b-main HEAD (commit `c1707a4`) before any Story 9.8 changes. Not regressions.

### [HARNESS-OVERRIDE]: golangci-lint issues are pre-existing on b-main

Gate 3.4 (`golangci-lint`) finds:
1. `internal/server/handlers/graph_neighborhood.go:171` — gosimple S1011 (pre-existing)
2. `internal/server/handlers/events_test.go:18,31` — unused functions (pre-existing)

All verified to exist on b-main HEAD. Not introduced by Story 9.8.

## Verification Summary

| Check | Result |
|---|---|
| Tests (npm run test) | 75/75 PASS |
| TypeScript (tsc --noEmit) | PASS |
| ESLint | PASS |
| Vite build | PASS (193 KB gzip, limit 600 KB) |
| Go build | PASS |
| Go test -race -short | PASS |
| Integration test failures | Pre-existing on b-main — OVERRIDE |
| Lint issues | Pre-existing on b-main — OVERRIDE |
| CI (GitHub Actions) | PASS |
| No Go files changed | VERIFIED |
| No DocDrawer/MemoryPanel/GraphPanel changed | VERIFIED |
| No dangerouslySetInnerHTML | VERIFIED |
| No lookbehind regex | VERIFIED |
| apiFetch used for all API calls | VERIFIED |
| JSDoc on all exported identifiers | VERIFIED |
| Bundle ≤ 600 KB gzipped | VERIFIED (193 KB) |

## Acceptance Criteria Verification

| AC | Status | Evidence |
|---|---|---|
| GET /api/v1/config populates form | ✅ PASS | `SettingsPanel.tsx:363-366` |
| POST /api/v1/config patches fields | ✅ PASS | `SettingsPanel.tsx:207-238` |
| Secrets as `<redacted>` read-only | ✅ PASS | `SettingsPanel.tsx:246-258` |
| Doctor checks with badges + hints | ✅ PASS | `SettingsPanel.tsx:35-73` |
| ConfirmDialog: disabled until match, Esc cancels | ✅ PASS | `ConfirmDialog.tsx:48,37-44` |
| Reset/Remove/Embeddings use ConfirmDialog | ✅ PASS | `SettingsPanel.tsx:142-171` |
| Cmd+K/Ctrl+K opens, Esc closes | ✅ PASS | `CommandPalette.tsx:128-155` |
| Palette groups: Nav/Actions/Workspaces/Symbols/Recent | ✅ PASS | `CommandPalette.tsx:240-310` |
| useMnemonicNav g+keys, 800ms, ignore editable | ✅ PASS | `useMnemonicNav.ts:4-60` |
| NonLoopbackBindBanner shows/hides correctly | ✅ PASS | `NonLoopbackBindBanner.tsx:5-23` |
| Focus trap in modals | ✅ PASS | `useFocusTrap.ts:28-36` |
| WCAG AA focus-visible | ✅ PASS | `layout.css:339-342,442-450` |
| /ui/settings → SettingsPanel | ✅ PASS | `router.tsx:58-62` |
| Bundle ≤ 600 KB | ✅ PASS | 193 KB gzipped |
| JSDoc on exports | ✅ PASS | All exported identifiers documented |

Review Verdict: PASS
