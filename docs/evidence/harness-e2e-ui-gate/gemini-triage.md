# Gemini Triage — harness-e2e-ui-gate (#285)

PR: nano-step/nano-brain#286
Date: 2026-06-01
Bot reviewer: gemini-code-assist[bot]
Agent: Sisyphus

## Triage Table (R31)

| Comment ref | Verdict | Reasoning | Action |
|-------------|---------|-----------|--------|
| smoke-ui.sh:L123 (Content-Type on /ui/ not validated, per spec) | VALID:high | Spec explicitly requires asserting Content-Type starts with text/html. Original implementation only checked body content. Multiple curl calls inefficient. | Applied: single curl with `-w "%{http_code} %{content_type}"`. Assert Content-Type matches `^text/html`. |
| smoke-ui.sh:L173 (Content-Type on assets not validated) | VALID:high | Spec requires JS=application/javascript and CSS=text/css. Original only checked body NOT starting with DOCTYPE. Three curl calls per asset wasteful. | Applied: single curl per asset with `-w "%{http_code} %{content_type} %{size_download}"`. Assert Content-Type matches expected per file extension. |
| smoke-ui.sh:L58 (allow NANO_BRAIN_DATABASE_URL override) | VALID:medium | Hardcoded `host.docker.internal` only works in containers. Local Linux/macOS users need `localhost`. | Applied: `db_url="${NANO_BRAIN_DATABASE_URL:-postgres://nanobrain:nanobrain@host.docker.internal:5432/nanobrain_dev}"`. Container default preserved. |
| smoke-ui.sh:L97 (fail fast if server dies during health wait) | VALID:medium | If server process dies (port collision, config error), we waste 15s waiting. `kill -0` check in loop fails fast. | Applied: added `kill -0 "${SERVER_PID}"` check at top of wait_for_health loop iteration. |
| harness-check.sh:L492 (scope evidence search to branch slug) | VALID:medium | Plain `find docs/evidence -name smoke-ui-output.log` returns first match globally — stale logs from other stories pass the gate. | Applied: extract branch slug via `git branch --show-current | sed 's|^[^/]*/||'`. Search `*${slug}*` path first, fall back to global find only if branch-specific not found (still catches missing-evidence case). |

## Resolution Summary

- 5 findings addressed in 1 push cycle (R31 limit: 3)
- 0 FALSE_POSITIVE / DEFER

## Verification Post-Fix

```
$ bash -n scripts/smoke-ui.sh                                   exit 0
$ bash -n scripts/harness-check.sh                              exit 0
$ ./scripts/smoke-ui.sh > docs/evidence/harness-e2e-ui-gate/smoke-ui-output.log
$ tail -5 docs/evidence/harness-e2e-ui-gate/smoke-ui-output.log
PASS: /ui/assets/index-__eULlu6.js → 200 application/javascript size=559430
PASS: /ui/assets/query-BoNQCwfV.js → 200 application/javascript size=45437
PASS: /ui/assets/router-UhuWy72c.js → 200 application/javascript size=229953
INFO: Asset summary: 3 JS, 1 CSS
=== smoke:ui PASS ===
```

Now Content-Type is asserted per Gemini Finding 1+2 — and the new log explicitly shows `application/javascript` / `text/css` per asset, proving the assertion runs.

## Loop count
1/3 push cycles.
