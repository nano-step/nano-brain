# Gemini Review Triage — PR #295

## Cycle 1

### Finding 1 — MEDIUM — `WorkspacesPanel.tsx:10` (defensive fmtNum)
**Verdict: ACCEPTED**

`fmtNum(n: number)` would throw if the API returned `undefined`/`null` for `doc_count` or `chunk_count`. We saw exactly this kind of FE/BE contract drift today on #277 / #279 — the TypeScript types said one thing, the wire format said another.

**Fix:** `fmtNum(n: number | undefined | null)` with `?? '0'` fallback. Also updated the dialog description (line 164) which had a direct `pending.doc_count.toLocaleString()` to use `fmtNum()` consistently.

### Finding 2 — MEDIUM — `WorkspacesPanel.tsx:14` (defensive truncHash)
**Verdict: ACCEPTED**

Same defense-in-depth: `truncHash(hash: string)` would throw on `null`/`undefined`. Added explicit null check + fallback to empty string.

### Finding 3 — MEDIUM — `WorkspacesPanel.tsx:61` (bypassing api/workspace abstraction)
**Verdict: ACCEPTED (with refinement)**

Gemini correctly flagged that `localStorage.removeItem('nano-brain.workspace')` duplicates the storage key from `api/workspace.ts` and bypasses the abstraction. If `setCurrentWorkspace` ever grew side effects (cookie, broadcast channel, etc.) this direct removal would desync.

Gemini suggested `setCurrentWorkspace('')` which writes empty string. Better fix: add an explicit `clearCurrentWorkspace()` helper to `api/workspace.ts` that maps to `localStorage.removeItem`. This keeps "set" and "clear" as distinct semantics (some consumers may want to distinguish between "no workspace set" vs "empty hash").

**Fix:**
- `api/workspace.ts`: added `export function clearCurrentWorkspace()`.
- `WorkspacesPanel.tsx`: replaced inline `try { localStorage.removeItem(...) }` with `clearCurrentWorkspace()`.

## Cycle 1 verification

- `cd web && npm run build` → built in 31s, bundle `index-*` regenerated
- `go build ./...` exit 0
- `bash scripts/smoke-ui.sh` → `=== smoke:ui PASS ===`

All 3 findings real and useful, accepted in 1 cycle.
