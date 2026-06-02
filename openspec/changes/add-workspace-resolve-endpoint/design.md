## Context

The workspace_hash is a deterministic SHA-256 of the absolute project path (`internal/storage/workspace.go:9-16`). Every workspace-scoped HTTP/CLI/MCP call requires this hash, but agents have no path-to-hash lookup endpoint.

**Today's options for an agent in a fresh shell:**

```bash
# Option A — wasteful re-register
WS=$(curl -sX POST $BASE/api/v1/init -d "{\"root_path\":\"$PWD\"}" | jq -r .workspace_hash)

# Option B — list + filter (O(N) over all workspaces)
WS=$(curl -s $BASE/api/v1/workspaces \
  | jq -r ".workspaces[] | select(.root_path == \"$PWD\") | .hash")
```

Both are documented inconsistently across `SKILL.md`, `AGENTS_SNIPPET.md`, and `AGENTS.md`. The `AGENTS.md` examples are also broken (wrong endpoint, missing field) — see proposal.md.

The Anthropic Skills 2026 guideline (progressive disclosure: SKILL.md body + references/) and industry patterns (kubectl `current-context`, AWS `sts get-caller-identity`, gcloud `current-config`) all converge on a **dedicated "resolve current context" endpoint** rather than overloading list/init.

## Goals / Non-Goals

**Goals:**
- One HTTP call: path in → hash + registration status out. No side effects.
- One CLI call: `nano-brain workspaces current` auto-detects CWD, prints hash (or `export` line).
- One MCP tool: same surface for MCP-only agents.
- Fix the broken `AGENTS.md` snippet and rewrite the skill phase-based.

**Non-Goals:**
- Auto-register on resolve (`init` already idempotent — separate intent).
- Implicit `NANO_BRAIN_WORKSPACE` env reading in middleware (Phương án C, separate proposal).
- Multi-project / "active workspace" persisted state.
- Symlink resolution beyond `filepath.Abs`.
- Path canonicalization across host/container (the agent's `$PWD` is its own truth; if host and container see different paths, that's deployment config, not our problem).

## Decisions

### D1: Read-only `POST` endpoint (not `GET`)

**Decision:** `POST /api/v1/workspaces/resolve` with JSON body `{path: string}`, NOT `GET /api/v1/workspaces/resolve?path=...`.

**Rationale:**
- Paths can contain characters that need URL-encoding (spaces, unicode, `?`, `#`). POST body avoids the encoding mess.
- Consistent with `POST /api/v1/init` which also takes `{root_path}` in body.
- Frontend/CLI/MCP all use `application/json` body — uniform.
- POST is semantically fine for read-only operations when the input is rich/structured (RFC 9110 §9.3.3 allows it; common for search endpoints).

**Alternative considered:** `GET` with query param. Rejected for encoding fragility.

### D2: Public route group (no `workspaceMiddleware`)

**Decision:** Register at `api.POST("/workspaces/resolve", ...)` in the public group at `routes.go:32-35`, NOT through `data := api.Group("", workspaceMiddleware(s.db))` at line 60.

**Rationale:**
- `workspaceMiddleware` extracts `workspace` from body and validates registration. This endpoint's input is `path`, not `workspace`. Middleware would reject every call.
- Same group as `/init` (`/init` also doesn't go through workspace middleware — it CREATES workspaces).

### D3: Compute hash even if not registered

**Decision:** Always return `workspace_hash` field, even when `registered: false`. Use `filepath.Abs` + `storage.WorkspaceHash` (same logic as `init`).

**Rationale:**
- Hash is a pure function of absolute path. Server can compute it without DB lookup. Returning it always means the agent can later call `init` with the path it already used — no second hash derivation needed client-side.
- Lets the CLI's `--export` flag work even pre-registration (agent can `export NANO_BRAIN_WORKSPACE=<hash>` then call `init` separately).

**Alternative considered:** Return 404 if not registered. Rejected — forces client to handle two response shapes; loses the "one call gives you everything" UX.

### D4: `name` field from DB if registered, from `filepath.Base` if not

**Decision:** When `registered: true`, return `name` from `workspaces.name` column. When `false`, fallback to `filepath.Base(absPath)`.

**Rationale:**
- Display-only field. Frontend/CLI uses it for human-readable workspace identification.
- Consistent fallback so the response shape is stable.

### D5: Response shape — flat object, no envelope

**Decision:** Return flat `{workspace_hash, root_path, name, registered}`. Not `{workspace: {...}}` wrapper.

**Rationale:**
- Single-object responses don't need wrapping (only list endpoints benefit from envelopes for pagination — see `fix-workspaces-api-contract` proposal).
- Matches existing `initResponse` shape (also flat).

```go
type workspaceResolveResponse struct {
    WorkspaceHash string `json:"workspace_hash"`
    RootPath      string `json:"root_path"`
    Name          string `json:"name"`
    Registered    bool   `json:"registered"`
}
```

### D6: CLI flags — `--path`, `--export`, `--json`, `--check`

**Decision:** Add to `runWorkspacesCmd` switch:

```bash
nano-brain workspaces current                  # bare hash to stdout
nano-brain workspaces current --path=/abs/path # override CWD
nano-brain workspaces current --export         # "export NANO_BRAIN_WORKSPACE=<hash>"
nano-brain workspaces current --json           # full JSON response
nano-brain workspaces current --check          # exit 2 if registered=false
```

**Exit codes:**
- `0` — resolved successfully (regardless of `registered`)
- `1` — HTTP error / server unreachable / `--path` invalid
- `2` — `--check` set AND `registered=false`

Flags are independent; can combine (e.g., `--export --check`).

### D7: MCP tool parity

**Decision:** Register `memory_workspaces_resolve` in `internal/mcp/tools.go` alongside other workspace tools. Args schema: `{path: string (required)}`. Response: same JSON shape as HTTP handler.

**Rationale:**
- MCP is a first-class consumer (per `internal/mcp/AGENTS.md`). Parity prevents "MCP agent has fewer capabilities than HTTP agent".
- Implementation can share the handler logic — extract to internal function, MCP tool wraps it.

### D8: Skill rewrite structure (Anthropic 2026 progressive disclosure)

**Decision:** `skills/nano-brain/SKILL.md` restructured into 4 phases:

| Phase | Purpose | Length |
|---|---|---|
| Phase 1 DISCOVER | Verify server, resolve workspace, register if needed. **Success criterion: `$NANO_BRAIN_WORKSPACE` is set and registered.** | ~50 lines |
| Phase 2 SELECT | User intent → operation decision tree (table). | ~50 lines |
| Phase 3 EXECUTE | Each of 7 operations (query/search/vsearch/write/wake-up/graph/symbols) with request, response, error table. | ~200 lines |
| Phase 4 RECOVER | Error catalog: HTTP code → meaning → recovery action. Retry limits (max 2). | ~30 lines |
| References | Pointer to `references/*.md` for deep dives. | ~10 lines |

**Rationale:**
- Anthropic guideline: skill body <500 lines, decision tree over prose, imperative voice.
- Phase-based reduces cognitive load for first-time agents.
- Each phase has explicit success criterion (testable by the agent itself).

### D9: AGENTS_SNIPPET.md is the agent's TLDR

**Decision:** AGENTS_SNIPPET.md (~40 lines) injected into project AGENTS.md must contain a working bootstrap one-liner and a minimal cheatsheet. Anything deeper points to `skills/nano-brain/SKILL.md`.

```markdown
### Bootstrap (once per shell)
eval "$(npx nano-brain workspaces current --export)"
```

**Rationale:**
- AGENTS.md is the first thing agents see. It must work out of the box.
- Current snippet has wrong endpoints + missing workspace field — agents fail before they start.

### D10: `--check` semantics for shell guards

**Decision:** `--check` makes the command exit 2 (distinct from 0/1) when `registered=false`. Enables shell guards:

```bash
npx nano-brain workspaces current --check 2>/dev/null \
  || npx nano-brain init --root="$PWD"
```

Exit code 2 (not 1) so users can distinguish "server unreachable" (1) from "valid path but not registered" (2).

**Rationale:**
- Standard convention: 0=success, 1=error, 2=specific condition (cf. grep no-match=1, jq no-match=1, but tools like `diff` use 0/1/2).
- The bash `||` chain works for either 1 or 2; users wanting fine-grained handling check `$?` explicitly.

## Risks / Trade-offs

| Risk | Mitigation |
|---|---|
| Endpoint leaks workspace metadata (name, doc count) | Already exposed by `GET /api/v1/workspaces`. No new attack surface. Auth middleware applies if enabled. |
| Path traversal via `path` field | `filepath.Abs` normalizes but doesn't restrict. Server returns hash regardless — there's no file read. Adversary already controls the path string; computing SHA-256 of arbitrary strings is not a vulnerability. |
| Agent uses `--export` then shell escapes the hash | Hash is 64 hex chars `[0-9a-f]`, safe in shell. Use `"$NANO_BRAIN_WORKSPACE"` quoting in skill examples. |
| MCP tool name collision | `memory_workspaces_resolve` — new name, no conflict (other tools: `memory_query`, `memory_search`, etc.) |
| CLI `--path` could be a relative path | Server handles `filepath.Abs` — same as `init`. Client doesn't need to pre-normalize. |
| Skill drift between canonical `skills/nano-brain/` and the 2 install paths (`.opencode/` + `~/.config/opencode/`) | Manual sync in this PR. Follow-up issue for `nano-brain skill sync` command. |
| Existing `AGENTS.md` is hand-written, not auto-generated from snippet | Manual edit this PR. Document the snippet→AGENTS.md sync responsibility in backlog. |

## Migration

- No DB migration.
- No config change.
- No API version bump.
- New endpoint is purely additive. Existing endpoints unchanged.
- Existing agents using `init`+filter or `list`+filter continue to work.
- New skill bootstrap one-liner requires the new CLI subcommand — only works after binary upgrade.
- For users on stale CLI: skill includes a fallback HTTP-direct bootstrap that uses `curl` against the new endpoint.
