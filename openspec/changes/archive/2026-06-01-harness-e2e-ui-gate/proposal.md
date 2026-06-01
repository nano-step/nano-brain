## Why

Five recent UI bugs shipped to production with green CI:

- #275: missing JS asset in binary embed (force-add fix)
- #277: workspaces API contract drift (shape mismatch)
- #278/#279: stats API contract drift (12 field mismatches)
- #281: documents endpoint missing (404 on Memory page)

Each had `go test -race -short ./...` PASSING. The validation ladder caught backend logic bugs but not the **FE-BE contract surface** — frontend assets missing from embed, response shape mismatches, method/route mismatches. All five were eventually caught by ad-hoc Chrome DevTools manual testing on dev port 3199.

This proposal formalizes that manual workflow into the harness so future web changes can't ship with broken UI.

## What Changes

- Add new validation layer `smoke:ui` to the Validation Ladder section in HARNESS.md.
- Add new helper script `scripts/smoke-ui.sh` that:
  - Builds dev binary
  - Starts it on port 3199 with `--serve-only --unsafe-no-auth`
  - Waits for `/health` to return 200 OK
  - Hits `/ui/` → asserts HTTP 200 + `Content-Type: text/html`
  - Parses asset references from `/ui/` HTML
  - Hits each `/ui/assets/*.js` → asserts `Content-Type: application/javascript` AND content size > 1 KB (catches HTML-fallback bug from #275)
  - Hits each `/ui/assets/*.css` → asserts `Content-Type: text/css`
  - Tears down server
  - Writes evidence log to stdout
- Update HARNESS.md change-type matrix: a new **web-change** annotation marks PRs where smoke:ui is required (any diff touching `web/src/**` OR `internal/server/handlers/**` OR `internal/server/webui/**`).
- Update `scripts/harness-check.sh` pre-merge phase to enforce: if PR diff touches the above paths, evidence file `docs/evidence/<change-slug>/smoke-ui-output.log` must exist AND contain "smoke:ui PASS".
- Add `smoke-ui-output.log` template guidance in HARNESS.md evidence section.

## Capabilities

### New Capabilities
- `harness-smoke-ui-gate`: Defines the smoke:ui validation layer and pre-merge enforcement for web-touching changes. Includes script contract (`scripts/smoke-ui.sh`), evidence format, and harness-check.sh enforcement logic.

### Modified Capabilities
None — this is a process/tooling additive change, no behavior of nano-brain itself changes.

## Impact

- **Process:** Future PRs touching web/server-handlers must produce smoke:ui evidence. Adds ~30 seconds per PR (script run time) but catches a known class of recurring bugs early.
- **Scripts:** New `scripts/smoke-ui.sh` (~100 lines bash). Update `scripts/harness-check.sh` pre-merge phase to enforce evidence file.
- **Docs:** HARNESS.md updated with new layer; HARNESS_GATES.md cross-references it.
- **CI:** No CI change in this PR (would require Docker PG service for binary E2E). Script is run locally by implementing agent.
- **Risk:** Low — additive process change. Existing PRs unaffected (only newly-opened PRs need the evidence).

## Out of Scope

- Browser automation (Playwright/Selenium) — separate proposal if needed
- CI integration with PostgreSQL service — separate proposal once smoke:ui local workflow proves stable
- Schema-driven FE/BE contract codegen (trpcgo/OpenAPI) — separate long-term proposal
- /ui/memory + /ui/dashboard interactive flows — out of scope; this gate is asset-level only
