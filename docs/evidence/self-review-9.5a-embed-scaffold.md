# Self-Review Gate 2.4 — Story 9.5a Embed Scaffold

**Reviewer:** Oracle  
**Date:** 2026-05-30  
**Implementer:** Sisyphus-Junior  

## Verdict: PASS

---

## Per-AC Table

| AC | Description | Status | Evidence |
|----|-------------|--------|----------|
| AC1 | GET /ui → 200 + text/html | ✅ PASS | `serveIndex` sets `text/html; charset=utf-8`. TestUIServesIndex verifies 200, content-type, body. |
| AC2 | SPA fallback to index.html | ✅ PASS | `spaFallback` returns index.html on file-not-found. TestUISPAFallback verifies /ui/memory/abc-123 → 200 + index.html. |
| AC3 | Hashed assets get Cache-Control immutable | ✅ PASS | `isHashedAsset` checks `assets/` prefix + dash/dot in base name. TestUIHashedAssetCache verifies immutable header. |
| AC4 | index.html no-cache | ✅ PASS | `serveIndex` line 43 and `spaFallback` line 59 both set `Cache-Control: no-cache`. TestUIServesIndex verifies. |
| AC5 | Security headers on /ui | ✅ PASS | `RegisterUIRoutes` applies `securityMW` to `/ui` group. `SecurityHeaders()` sets CSP, X-Content-Type-Options, X-Frame-Options, Referrer-Policy. TestUISecurityHeadersApplied + TestSecurityHeaders verify. |
| AC6 | Security headers NOT on /api/v1/* | ✅ PASS | TestUISecurityHeadersNotOnAPI verifies header absence on /api/v1/test. Middleware is scoped to `/ui` group only. |
| AC7 | Missing dist → fallback page (NOT 404) | ✅ PASS | `hasIndex` check at registration time. If false, `serveMissingUI` returns 200 + instructional HTML. TestUIMissingFallback and TestUIMissingFallbackSubpath verify. |
| AC8 | Non-loopback without --unsafe-no-auth → error | ✅ PASS | `checkBindSafety` returns error naming the host + flag. main.go line 205-208 prints error and exits. TestCheckBindSafety_RejectsNonLoopback verifies. |
| AC9 | Non-loopback with --unsafe-no-auth → warning | ✅ PASS | main.go line 216-218 emits logger.Warn. TestCheckBindSafety_UnsafeFlagBypasses verifies no error returned. |
| AC10 | Loopback variants work | ✅ PASS | TestIsLoopback covers: localhost, 127.0.0.1, 127.0.0.5, ::1, [::1], "", LOCALHOST. All return true. |
| AC11 | make web-build graceful no-op | ✅ PASS | `cd web 2>/dev/null && npm run build || echo "web/ not present"`. Verified: outputs message, exit 0. |
| AC12 | go build exit 0, binary ≤ +8 MB | ✅ PASS | `go build ./...` succeeds. Binary: 52 MB (baseline ~54 MB, no dist payload yet). |
| AC13 | go test -race -short all pass | ✅ PASS | 26 packages, 0 failures. Verified by reviewer with fresh `-count=1` run. |

---

## Additional Findings (a–j)

### a. isLoopback edge cases ✅ PASS
- `0.0.0.0` → `net.ParseIP` succeeds, `IsLoopback()` = false → correctly rejected. Test at line 19 confirms.
- `[::]` → brackets stripped → `::` → `ParseIP` succeeds → `IsLoopback()` = false → rejected.
- IPv6 zone IDs (`fe80::1%eth0`) → `net.ParseIP` returns nil for zone IDs → `ip != nil` fails → returns false → rejected.
- No gaps found.

### b. fs.FS vs embed.FS ✅ PASS
- `RegisterUIRoutes` signature uses `fs.FS` (handler.go line 16) for testability.
- `routes.go` line 121 passes `webui.EmbedFS` (type `embed.FS`) which satisfies `fs.FS`.
- Godoc on `RegisterUIRoutes` documents purpose clearly.

### c. CSP allows the SPA ✅ ACCEPTABLE
- CSP: `default-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'`
- `script-src` inherits `'self'` from `default-src` — inline scripts blocked, external .js files allowed.
- Modern Vite React apps use external script references, not inline scripts.
- `'unsafe-inline'` on `style-src` covers CSS-in-JS (MUI, styled-components).
- `connect-src 'self'` allows API calls to same origin.
- No issue expected for Story 9.5b SPA.

### d. MIME detection ✅ ACCEPTABLE
- `mimeFor` handles: .html, .js, .mjs, .css, .json, .svg, .png, .ico, .woff2, .woff, .map
- Covers all Vite build output types. Missing .jpg/.gif/.webp/.ttf/.eot fall to `application/octet-stream`.
- Sufficient for scaffold scope; can be extended if needed.

### e. SPA fallback for non-HTML requests ⚠️ MEDIUM
- `spaFallback` returns index.html for ANY non-existent path, including `/ui/missing-asset.js`.
- This is the **standard SPA pattern** (used by CRA, Vite preview, nginx try_files). Real asset references from index.html will exist in dist/.
- A stricter approach (404 for non-existent .js/.css) would be more correct but adds complexity and breaks convention.
- **Verdict:** Acceptable for this story. Can be tightened in a future iteration if needed.

### f. Race between dist-check and route registration ✅ NOT APPLICABLE
- `embed.FS` is compiled into the binary — immutable at runtime. No race possible.
- `hasIndex` is evaluated once at startup. Correct behavior.

### g. /ui vs /ui/ ✅ ACCEPTABLE
- Echo `Group("/ui")` + `GET("")` handles `/ui` exactly.
- `GET("/*")` handles `/ui/anything`.
- `/ui/` would match wildcard → spaFallback → empty path → falls back to index.html → 200.
- Both routes work. No test for `/ui/` specifically, but behavior is correct per Echo semantics.

### h. Bind-safety order ✅ PASS
- `checkBindSafety` (main.go line 205) runs AFTER `config.Load` (line 198) but BEFORE `server.New` (line 302).
- Correct ordering — config needed to know the host, but server must not start if check fails.

### i. --unsafe-no-auth flag parsing ⚠️ MINOR
- daemon.go line 46-47: `case "--unsafe-no-auth": unsafeNoAuth = true`
- Simple string match — `--unsafe-no-auth=true` would not match (hits default → error).
- Consistent with `-d`/`--detach` pattern in same switch block.
- Usage documents it as a presence flag: `Usage: nano-brain serve [-d] [--unsafe-no-auth]`.
- **Verdict:** Consistent with existing patterns. Not a bug, just a style choice.

### j. No globals modified at test time ✅ PASS
- All 3 bind-safety tests save/restore `unsafeNoAuth` via `old := unsafeNoAuth` + `defer`.
- No test pollution risk.

---

## Findings Summary

### Critical: None

### Medium (1)
- **(e) SPA fallback serves index.html for any non-existent path** including .js/.css 404s. Standard SPA pattern but worth documenting. Not a blocker — follows industry convention (CRA, Vite, nginx `try_files`).

### Minor (1)
- **(i) `--unsafe-no-auth=true` form not supported** — only `--unsafe-no-auth` (presence flag). Consistent with `--detach` pattern. Document in help text if desired.

---

## Test Evidence

```
$ go build ./...
(exit 0)

$ go test -race -short ./...
ok  github.com/nano-brain/nano-brain/cmd/nano-brain           (26 tests)
ok  github.com/nano-brain/nano-brain/internal/server/webui     (7 tests)
ok  github.com/nano-brain/nano-brain/internal/server/middleware (1 test)
... (26 packages total, 0 failures)

$ make web-build
web/ not present (Story 9.5b)
(exit 0)

$ ls -lh /tmp/nb-test-9.5a
52M (baseline ~54 MB, delta < 8 MB)
```

---

VERIFIED
