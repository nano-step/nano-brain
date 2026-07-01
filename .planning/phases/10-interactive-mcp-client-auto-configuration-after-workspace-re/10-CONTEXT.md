# Phase 10: Interactive MCP client auto-configuration - Context

**Gathered:** 2026-07-01
**Status:** Ready for planning

<domain>
## Phase Boundary

After `nano-brain init --root=<path>` successfully registers a workspace, interactively prompt which AI clients (Claude Code, OpenCode, Codex CLI) to configure MCP for, and write/patch each selected client's config file with a nano-brain MCP server entry bound to the just-registered workspace via the `?workspace=<name>` URL query param shipped in Phase 9 (PR #524). Does NOT include a `curl | bash` distribution installer (separate, deferred capability).

</domain>

<decisions>
## Implementation Decisions

### Trigger point and non-interactive contract
- **D-01:** Hook into the `--root` branch of `runInitCmd` (`cmd/nano-brain/commands.go`), after the existing success output (`Workspace registered: ...`, `AgentsSnippet`) and before/alongside `triggerInitBackground`. Do NOT touch the no-`--root` interactive server-setup wizard (`runInteractiveInit` in `init.go`) — that's a different, already-complete flow for DB/embedding/server config, not MCP client config.
- **D-02:** Skip all new prompting when `--json` is passed (existing non-interactive contract, already used by scripts/CI) or when stdin is not a TTY. Non-interactive callers get no behavior change.

### Config target: project-local, not global
- **D-03:** Write to each client's **project-local** MCP config file (e.g. Claude Code's `.mcp.json` in the registered project root), NOT the global `~/.claude.json`. Rationale: nano-brain is one shared daemon serving every registered project from the same base URL; a global config entry can only carry ONE `?workspace=` default at a time. Project-local config lets every registered project have its own correctly-scoped `?workspace=<this-project's-name>` binding simultaneously — this is the exact capability Phase 9 unlocked, and only project-local config can actually use it correctly across multiple projects.
- **D-04:** The existing docs (`docs/SETUP_AGENT.md` Step 9) that recommend the global `~/.claude.json` are NOT wrong for a single-project user and are NOT being removed — this phase adds the automated, multi-project-safe alternative, not a replacement.

### Idempotency and merge safety
- **D-05:** Read-modify-write: if the target config file exists, parse it, add/update only the `nano-brain` key under the relevant section (`mcpServers` for Claude Code, `mcp` for OpenCode, whatever Codex CLI's schema calls it), and write back preserving every other key/server untouched. If the file doesn't exist, create it with just the `nano-brain` entry. Re-running the command against an already-configured client must be a no-op (or a clean update if the workspace binding changed), never a duplicate entry or a corrupted file.
- **D-06:** Before overwriting an existing `nano-brain` entry with a different `?workspace=` value, show the user what will change and let them confirm (matches `runInteractiveInit`'s existing "Config exists... Overwrite?" pattern) — silently clobbering a user's prior manual configuration is not acceptable.

### UX pattern
- **D-07:** Reuse the existing `bufio.Scanner` + `promptWithDefault` interactive-prompt pattern already established in `init.go` (no new TUI/prompt library). Ask per-client Y/N ("Configure Claude Code for this workspace? [Y/n]"), matching the existing style of `ocSessionDir`/`ccSessionDir` prompts in `runInteractiveInit`, rather than building a new multi-select widget.
- **D-08:** Reuse the existing `detect.go` path-detection convention (env var override → `platformXPaths()` candidate list → `os.Stat` check) already used for `detectOpenCodeStorageDir`/`detectClaudeCodeStorageDir`, adapted to detect each client's *config* file location (not session-storage location — these are different paths) so the prompt can show a sensible default path or auto-detect "is this client even installed" to decide whether to ask at all.

### Claude's Discretion
- Exact wording of prompts and success/skip messages.
- Whether to also offer configuring the global `~/.claude.json` as a secondary option for users who only ever work in one project (nice-to-have, not required for the acceptance criteria).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Existing code to extend/follow
- `cmd/nano-brain/commands.go:13-113` (`runInitCmd`) — the `--root` branch is the exact hook point; `result.WorkspaceHash`/`result.RootPath` are already available here.
- `cmd/nano-brain/init.go:26-253` (`runInteractiveInit`) — the established interactive-prompt style (`bufio.Scanner`, `promptWithDefault`) and its own "Config exists... Overwrite?" confirmation pattern (D-06 should mirror this).
- `cmd/nano-brain/detect.go:12-38` (`detectOpenCodeStorageDir`, `detectClaudeCodeStorageDir`) and `platformClaudeCodePaths`/`platformOpenCodePaths` — the established env-var-override + platform-path-candidate-list + `os.Stat` detection pattern (D-08).
- `internal/server/handlers/workspace.go:176-188` — where the current generic `AgentsSnippet` is generated; NOT necessarily where the new logic lives (this is server-side, the new prompting is CLI-side in `commands.go`), but shows what data is already available post-registration.
- `docs/SETUP_AGENT.md` Step 9 (current manual MCP config instructions per client, including the `?workspace=` example added in Phase 9) — the manual fallback this phase automates; do not delete, this phase is additive.
- Phase 9 artifacts (`.planning/phases/09-mcp-workspace-config-binding-.../09-CONTEXT.md`, `09-RESEARCH.md`) — defines the exact `?workspace=<name-or-hash>` URL contract this phase's generated config must produce correctly (D-02 there: never write `"all"`).

### Open research questions (for gsd-phase-researcher, not yet answered)
- Exact project-local MCP config file name/schema Claude Code actually reads (`.mcp.json` in project root — confirm exact schema: is it `{"mcpServers": {...}}` same as the global file, or different?).
- OpenCode's actual config file **location** (not just the JSON shape already documented in SETUP_AGENT.md) — is it project-local, global, or both? `detectOpenCodeStorageDir` only knows the *session storage* dir, which research must confirm is NOT the same file as the MCP config.
- Codex CLI's MCP config format and file location — completely undocumented in this repo today; codegraph (external reference analyzed in a prior session, github.com/colbymchenry/codegraph) supports it, so a real format exists to look up, but must be verified via docs/web, not assumed.
- Whether any of these config files have existing Go libraries/parsers in this codebase already that should be reused, or if raw `encoding/json` read-modify-write is the right approach (likely yes, given how simple these JSON shapes are, but confirm no existing structured type would help avoid clobbering unknown-shape user config).

No external specs/ADRs beyond Phase 9's own CONTEXT.md/RESEARCH.md.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `promptWithDefault` (`init.go:255`) — direct reuse for any new Y/N or free-text prompt.
- `detectOpenCodeStorageDir`/`detectClaudeCodeStorageDir` + `platformXPaths()` helpers (`detect.go`) — pattern to clone for config-file detection (different paths, same detection shape).
- `result.WorkspaceHash`/`result.RootPath` already parsed in `runInitCmd`'s `--root` branch — no new API call needed to get the workspace identity; only a "name" (vs hash) may need deriving — check whether `/api/v1/init`'s response or `nano-brain workspaces list`'s `name` field is derivable directly from `RootPath` (likely the directory basename) rather than requiring a new round trip.

### Established Patterns
- CLI commands in this codebase call the local HTTP API (`doRequest` helper) rather than touching the DB directly from `cmd/nano-brain/` — any NEW data needed (e.g. confirming the workspace's registered `name`) should go through the existing `/api/v1/init` response or a follow-up API call, not direct DB access from the CLI binary.
- Config-writing so far in this codebase (`runInteractiveInit`'s `os.WriteFile(configPath, []byte(yaml), 0600)`) always writes whole-file, never merges — this phase's write path is different in kind (must merge into a possibly-pre-existing, possibly-hand-edited JSON file) and needs its own read-modify-write helper, not a reuse of the whole-file-overwrite pattern.

### Integration Points
- `runInitCmd`'s `--root` success branch, right after the existing `fmt.Println(result.AgentsSnippet)` line and before/alongside `triggerInitBackground`.

</code_context>

<specifics>
## Specific Ideas

User's own framing: "tôi thấy các tool cli khác install rất dễ, curl...|bash hay gì đó rồi nano-brain init, sau đó sẽ show ra hỏi config mcp cho các tools nào, ví dụ claude, opencode, codex" (other CLI tools are easy to install — curl|bash then init, which then prompts which tools to configure MCP for: Claude, OpenCode, Codex). Directly modeled on `codegraph install`'s multi-agent auto-configuration (analyzed earlier this session from github.com/colbymchenry/codegraph — supports Claude Code, Cursor, Codex CLI, opencode, Hermes Agent).

</specifics>

<deferred>
## Deferred Ideas

- `curl | bash` one-line distribution installer downloading pre-built platform binaries from GitHub Releases (already cross-built by `.github/workflows/release.yml`) — a separate, largely-independent capability (packaging/distribution, not MCP config) explicitly named by the user in the same message but scoped out of this phase per the intake issue (#525). Candidate for its own future phase.
- Supporting additional clients beyond Claude Code/OpenCode/Codex CLI (Cursor, Hermes Agent, etc., per codegraph's fuller list) — start with the 3 the user named, extend later if there's demand.
- Auto-configuring the global `~/.claude.json` as an alternative/addition to project-local — noted as Claude's Discretion, not a hard requirement.

### Reviewed Todos (not folded)
None - discussion stayed within phase scope.

</deferred>

---

*Phase: 10-Interactive MCP client auto-configuration*
*Context gathered: 2026-07-01*
