# Gemini Review Triage — PR #293

## Cycle 1 (push 4eb16fb)

### Finding 1 — HIGH — `npm/postinstall.js:105` (API fallback normalization)
**Verdict: ACCEPTED**

`r.tag_name.replace(/^v/, "").replace(/^0+/, "") === version` only strips leading zeros from the start of the entire string, not from each version component. Comparing `v2026.6.0101` (→ `2026.6.0101`) against `2026.6.101` fails because there are no leading zeros at position 0.

**Fix:** Extracted `normalizeVersion()` helper that strips leading zeros from EACH component independently. Now `v2026.6.0101` and `2026.6.101` both normalize to `2026.6.101` → match.

### Finding 2 — MEDIUM — `npm/postinstall.js:76` (prerelease patch handling)
**Verdict: ACCEPTED**

`parseInt("1-beta", 10)` returns `1`, then `.padStart(4, "0")` → `"0001"`, silently dropping the `-beta` suffix and producing a wrong candidate tag `v2026.6.0001` instead of `v2026.6.1-beta`.

**Fix:** Replaced `parseInt + isNaN` with `/^\d+$/` regex test. Only pad when the patch is purely numeric. Verified: `2026.6.1-beta` now produces `["v2026.6.1-beta"]` (no spurious padded candidate).

### Finding 3 — MEDIUM — `npm/postinstall.js:83` (socket leak on non-200)
**Verdict: ACCEPTED**

Per Node.js docs, response streams must be consumed even when returning early on non-200. Otherwise the underlying socket isn't freed and can lead to slow connection-pool exhaustion.

**Fix:** Added `res.resume()` before reject in the non-200 branch.

## Cycle 1 verification

- `node -c npm/postinstall.js` syntax OK
- Unit-test against all historic patch widths PASS:
  - `2026.6.101` → `["v2026.6.101", "v2026.6.0101"]` ✓
  - `2026.6.8` → `["v2026.6.8", "v2026.6.0008"]` ✓
  - `2026.5.3105` → `["v2026.5.3105"]` (no padding for 4-digit) ✓
  - `2026.6.1-beta` → `["v2026.6.1-beta"]` (no padding for prerelease) ✓
- `normalizeVersion` test: all (tag, npm-version) pairs match correctly
- End-to-end against live npm package `2026.6.101`:
  ```
  Downloading nano-brain v2026.6.101 for linux-arm64...
  nano-brain v2026.6.101 installed successfully from v2026.6.0101.
  ```

All 3 Gemini findings real and useful — accepted in 1 cycle.
