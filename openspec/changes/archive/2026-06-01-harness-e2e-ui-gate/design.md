## Context

The validation ladder in HARNESS.md currently has:

- `validate:quick` — go build + go test -race -short
- `self-review:response-shape` — manual struct + mapping audit
- `self-review:staged-files` — git status review
- `test:integration` — go test -tags=integration
- `smoke:e2e` — manual build + start server + curl endpoints

`smoke:e2e` is defined as "real usage test" but is implemented as **manual** agent execution. It's not enforced because agents can skip it without evidence. Plus it doesn't specifically test the UI asset embedding path, which is where 4 of 5 recent bugs lived.

## Goals / Non-Goals

**Goals:**
- Detect missing/corrupt JS assets embedded in binary (root cause of #275)
- Detect API response shape mismatches that frontend depends on (#277, #278, #279)
- Detect frontend-only contract drift (#281: endpoint missing entirely)
- Enforce evidence file existence via harness-check.sh pre-merge gate
- Make smoke:ui a fast (<30s) deterministic check runnable by agents locally

**Non-Goals:**
- Replace smoke:e2e (still required for normal/high-risk; smoke:ui complements it)
- Browser-based interactive testing (out of scope; this is asset+API level)
- CI integration with Postgres (deferred; local agent workflow first)
- Full FE/BE contract validation (deferred to schema-driven approach)

## Decisions

### D1: Script location + invocation pattern

**Decision:** `scripts/smoke-ui.sh` — matches existing `scripts/harness-check.sh` location. Bash, no external deps beyond curl + jq.

**Invocation:**
```bash
./scripts/smoke-ui.sh > docs/evidence/<change-slug>/smoke-ui-output.log 2>&1
```

Script exits 0 on PASS, non-zero on FAIL. Last line must contain "smoke:ui PASS" or "smoke:ui FAIL".

**Rationale:** Bash + curl + jq are already required for `harness-check.sh`. No new tooling needed.

### D2: Port 3199 (standard convention)

**Decision:** Hardcode port 3199 in smoke-ui.sh, matching existing smoke:e2e convention.

**Rationale:** Avoids collision with production 3100. Documented in HARNESS.md line 313.

### D3: Use --serve-only mode to skip background work

**Decision:** Start dev binary with `--serve-only --unsafe-no-auth` flags.

**Rationale:**
- `--serve-only` (#282) avoids embedding work + harvester polling, making the check deterministic and fast
- `--unsafe-no-auth` allows binding to 0.0.0.0 without auth (required for container access)
- Container test env: localhost or host.docker.internal works the same with 0.0.0.0 bind

### D4: Asset content-type AND size assertion

**Decision:** For each JS asset referenced in `/ui/`:
- HTTP status must be 200
- `Content-Type` header must be `application/javascript`
- Body size > 1024 bytes

**Rationale:**
- Content-type catches MIME mismatch (browser refuses HTML-served-as-JS)
- Body size > 1 KB catches the index.html fallback bug from #275 (HTML is 578 bytes)
- Cheap to verify, deterministic

### D5: Enforcement scope — paths that trigger smoke:ui

**Decision:** Pre-merge gate in `harness-check.sh` enforces evidence when PR diff includes any of:

- `web/src/**` (frontend source)
- `web/package.json` (frontend deps)
- `internal/server/handlers/**` (API endpoints that frontend may consume)
- `internal/server/webui/**` (embed FS + handler)
- `internal/server/routes.go` (route wiring)

**Rationale:** Conservative scope. False positive (smoke:ui required when not needed) costs 30s. False negative (smoke:ui skipped when needed) costs a broken release. The five recent bugs all touched at least one of these paths.

### D6: Evidence format

**Decision:** Plain text log file at `docs/evidence/<change-slug>/smoke-ui-output.log`. Format:

```
=== smoke:ui run YYYY-MM-DD HH:MM:SS ===
Binary built: /tmp/nano-brain-smoke/nano-brain (size=NN bytes)
Server started on port 3199 (PID=N)
GET /health → 200 OK ready=true
GET /ui/ → 200 OK content-type=text/html size=NN
Asset checks:
  GET /ui/assets/index-<hash>.js → 200 OK content-type=application/javascript size=NN ✓
  GET /ui/assets/index-<hash>.css → 200 OK content-type=text/css size=NN ✓
  ...
Server stopped
=== smoke:ui PASS ===
```

**Rationale:** Greppable plain text. harness-check.sh pre-merge greps for "smoke:ui PASS" at end.

### D7: harness-check.sh integration

**Decision:** Add new check `3.8 smoke:ui evidence (if web change)` to `phase_pre_merge`:

```bash
if git diff --name-only "origin/b-main...HEAD" | grep -qE '^(web/src/|web/package\.json|internal/server/handlers/|internal/server/webui/|internal/server/routes\.go)'; then
    evidence=$(find docs/evidence -name "smoke-ui-output.log" -newer scripts/smoke-ui.sh 2>/dev/null | head -1)
    if [[ -z "$evidence" ]]; then
        add_check "FAIL" "3.8 web change but no docs/evidence/*/smoke-ui-output.log"
    elif ! grep -q "smoke:ui PASS" "$evidence"; then
        add_check "FAIL" "3.8 $evidence does not contain 'smoke:ui PASS'"
    else
        add_check "PASS" "3.8 smoke:ui evidence: $evidence"
    fi
else
    add_check "SKIP" "3.8 no web change in PR diff"
fi
```

**Rationale:** Lightweight grep on `git diff`. Newer-than-script check prevents stale evidence from passing the gate.

## Risks / Trade-offs

| Risk | Mitigation |
|---|---|
| Smoke script requires Postgres + Ollama to start binary | Use `--serve-only` (no embed worker needed) + skip ollama check (server still starts without it) |
| Path enforcement too narrow → misses bug | Conservative scope (5 path patterns). Easy to extend. |
| Path enforcement too wide → noise on docs-only PRs | Specific paths only — `*.md` excluded. |
| Evidence file becomes stale | `find -newer scripts/smoke-ui.sh` forces re-run when script changes |
| Agent runs smoke:ui but doesn't commit log | Pre-merge check catches missing file. |
| Script differs from real CI | This IS the local CI for now. Future: add to .github/workflows/ci.yml. |
| Bash portability | Stick to portable bash (no GNU-only flags). Tested on Linux + macOS. |

## Migration

- New PRs touching web/server-handlers: must produce smoke:ui evidence
- Existing in-flight PRs: not retroactively required (only enforced once this change merges)
- Documentation: README + HARNESS.md updated in same PR
