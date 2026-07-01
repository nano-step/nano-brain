# Phase 10: Interactive MCP client auto-configuration - Research

**Researched:** 2026-07-01
**Domain:** Go CLI interactive prompting + multi-format (JSON/TOML) config file read-modify-write, targeting three external AI-client config schemas
**Confidence:** HIGH (Claude Code, OpenCode location/schema, existing-code hooks, workspace-name gap, JSON/TOML merge approach) / MEDIUM (Codex CLI exact schema nuances — single official-doc source, not cross-verified against a second independent source)

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Trigger point and non-interactive contract**
- D-01: Hook into the `--root` branch of `runInitCmd` (`cmd/nano-brain/commands.go`), after the existing success output (`Workspace registered: ...`, `AgentsSnippet`) and before/alongside `triggerInitBackground`. Do NOT touch the no-`--root` interactive server-setup wizard (`runInteractiveInit` in `init.go`) — that's a different, already-complete flow for DB/embedding/server config, not MCP client config.
- D-02: Skip all new prompting when `--json` is passed (existing non-interactive contract, already used by scripts/CI) or when stdin is not a TTY. Non-interactive callers get no behavior change.

**Config target: project-local, not global**
- D-03: Write to each client's **project-local** MCP config file (e.g. Claude Code's `.mcp.json` in the registered project root), NOT the global `~/.claude.json`. Rationale: nano-brain is one shared daemon serving every registered project from the same base URL; a global config entry can only carry ONE `?workspace=` default at a time. Project-local config lets every registered project have its own correctly-scoped `?workspace=<this-project's-name>` binding simultaneously.
- D-04: The existing docs (`docs/SETUP_AGENT.md` Step 9) that recommend the global `~/.claude.json` are NOT wrong for a single-project user and are NOT being removed — this phase adds the automated, multi-project-safe alternative, not a replacement.

**Idempotency and merge safety**
- D-05: Read-modify-write: if the target config file exists, parse it, add/update only the `nano-brain` key under the relevant section (`mcpServers` for Claude Code, `mcp` for OpenCode, whatever Codex CLI's schema calls it), and write back preserving every other key/server untouched. If the file doesn't exist, create it with just the `nano-brain` entry. Re-running the command against an already-configured client must be a no-op (or a clean update if the workspace binding changed), never a duplicate entry or a corrupted file.
- D-06: Before overwriting an existing `nano-brain` entry with a different `?workspace=` value, show the user what will change and let them confirm (matches `runInteractiveInit`'s existing "Config exists... Overwrite?" pattern) — silently clobbering a user's prior manual configuration is not acceptable.

**UX pattern**
- D-07: Reuse the existing `bufio.Scanner` + `promptWithDefault` interactive-prompt pattern already established in `init.go` (no new TUI/prompt library). Ask per-client Y/N ("Configure Claude Code for this workspace? [Y/n]"), matching the existing style of `ocSessionDir`/`ccSessionDir` prompts in `runInteractiveInit`, rather than building a new multi-select widget.
- D-08: Reuse the existing `detect.go` path-detection convention (env var override → `platformXPaths()` candidate list → `os.Stat` check) already used for `detectOpenCodeStorageDir`/`detectClaudeCodeStorageDir`, adapted to detect each client's *config* file location (not session-storage location — these are different paths) so the prompt can show a sensible default path or auto-detect "is this client even installed" to decide whether to ask at all.

### Claude's Discretion
- Exact wording of prompts and success/skip messages.
- Whether to also offer configuring the global `~/.claude.json` as a secondary option for users who only ever work in one project (nice-to-have, not required for the acceptance criteria).

### Deferred Ideas (OUT OF SCOPE)
- `curl | bash` one-line distribution installer downloading pre-built platform binaries from GitHub Releases (already cross-built by `.github/workflows/release.yml`) — a separate, largely-independent capability (packaging/distribution, not MCP config) explicitly named by the user in the same message but scoped out of this phase per the intake issue (#525). Candidate for its own future phase.
- Supporting additional clients beyond Claude Code/OpenCode/Codex CLI (Cursor, Hermes Agent, etc., per codegraph's fuller list) — start with the 3 the user named, extend later if there's demand.
- Auto-configuring the global `~/.claude.json` as an alternative/addition to project-local — noted as Claude's Discretion, not a hard requirement.
</user_constraints>

<phase_requirements>
## Phase Requirements

No formal REQ IDs are mapped to this phase (feature phase, tracked via GitHub issue #525, not `.planning/REQUIREMENTS.md`). All work is scoped by CONTEXT.md's decisions D-01 through D-08 above.
</phase_requirements>

## Summary

This phase adds a small, well-bounded slice of CLI functionality to `cmd/nano-brain/commands.go`'s `runInitCmd` `--root` branch: after workspace registration succeeds, interactively ask which of three AI clients (Claude Code, OpenCode, Codex CLI) to configure, then read-modify-write each selected client's **project-local** MCP config file with a `nano-brain` server entry bound to `?workspace=<name>`.

All four of the open research questions from CONTEXT.md are answered with HIGH-to-MEDIUM confidence:

1. **Claude Code** reads `.mcp.json` in the project root with the identical `{"mcpServers": {...}}` shape as the global `~/.claude.json` — same top-level key, same server-entry schema (`type`, `command`/`args`/`env` for stdio, or `type: "http"` + `url` for HTTP). This is the simplest of the three: same schema the codebase's `docs/SETUP_AGENT.md` already documents for the global file, just written to a different path.

2. **OpenCode** reads a project-local `opencode.json` at the **project root** (highest precedence, distinct from `~/.config/opencode/opencode.json` global config) — this is a **different file from `detectOpenCodeStorageDir`'s target** (`~/.local/share/opencode/storage` or platform equivalent), confirming CONTEXT.md's suspicion. The MCP section lives under the `mcp` key, but the **schema differs from Claude Code's**: OpenCode requires an explicit `"type": "remote"` (for HTTP) or `"type": "local"` field — there is no `"http"`/`"stdio"` naming — and the URL field is `url` with optional `headers`, `enabled`, `oauth`, `timeout` fields. Existing `docs/SETUP_AGENT.md` documents `"type": "http"` for OpenCode, which **appears to be stale/incorrect** against current OpenCode docs — flagged as a pitfall below; the planner should verify against a live OpenCode install if possible, and this phase's generated JSON should use `"type": "remote"` for the nano-brain HTTP entry, not `"http"`.

3. **Codex CLI** reads `config.toml` — **TOML, not JSON** — either globally at `~/.codex/config.toml` or **project-scoped** at `.codex/config.toml` in the project root (trusted projects only). This is a brand-new file *format* for this codebase. Good news: `github.com/BurntSushi/toml v1.6.0` is **already present in go.mod as an indirect dependency** (pulled in transitively, not currently imported by any nano-brain code) — no new dependency needs to be added to go.sum, only promoted from `// indirect` to a direct `require` if imported. MCP servers live under `[mcp_servers.<name>]` tables; an HTTP entry needs only a `url` field (transport is inferred: presence of `url` = HTTP, presence of `command` = stdio) plus optional `bearer_token_env_var`/`http_headers`.

4. **Workspace "name" gap (critical, must be planned for):** `runInitCmd`'s `--root` branch currently unmarshals only `WorkspaceHash`, `RootPath`, `AgentsSnippet` from `POST /api/v1/init`'s JSON response (`cmd/nano-brain/commands.go:100-104`; see `internal/server/handlers/workspace.go:30-34`'s `initResponse` struct). The workspace's **`name`** field (`filepath.Base(absPath)`, computed server-side at `workspace.go:129` and persisted via `UpsertWorkspace`) is **never returned** to the CLI today. Per Phase 9's contract (D-02 there: never write `"all"` as the workspace binding), the generated `?workspace=<name>` needs this name — but there is no round trip to fetch it currently in the `--root` init flow. **This is a required addition, not an existing helper to reuse**: `initResponse` (server-side struct) needs a new `Name string` JSON field, and the CLI's local unmarshal struct in `runInitCmd` needs the matching field. This is the single most load-bearing finding in this research — the planner must add a task for it, not discover it mid-implementation.

**Primary recommendation:** Implement per-client "detect config path → prompt Y/N → read-modify-write" as three small, structurally parallel helper functions (one per client) called from `runInitCmd`'s `--root` branch, guarded by `!jsonFlag && isTTY()`. Use `encoding/json` + `map[string]any` for Claude Code and OpenCode (both JSON); use `github.com/BurntSushi/toml`'s `Decode`/`Encode` into `map[string]any` for Codex CLI (TOML). Add `Name` to both the server-side `initResponse` struct and the CLI's local decode struct as a prerequisite task.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| Interactive Y/N prompting per client | CLI (`cmd/nano-brain/`) | — | Pure terminal I/O, no server involvement; matches existing `runInteractiveInit` pattern |
| Client config file detection (path exists?) | CLI (`cmd/nano-brain/detect.go`) | — | Filesystem probing (env var → candidate paths → `os.Stat`), same tier as existing `detectOpenCodeStorageDir` |
| Config file read-modify-write (JSON/TOML) | CLI (`cmd/nano-brain/`) | — | Local file I/O on the user's machine (`.mcp.json`, `opencode.json`, `.codex/config.toml`); no server round trip needed once the workspace name is known |
| Workspace `name` retrieval | API / Backend (`/api/v1/init` response) | CLI (decode struct) | `name` is computed and persisted server-side (`internal/server/handlers/workspace.go`); the CLI is a pure consumer of the response field — this is a response-contract addition, not new business logic |
| `?workspace=<name>` URL construction | CLI (`cmd/nano-brain/`) | — | String formatting using data already available post-registration; no new resolution logic (reuses Phase 9's contract, does not reimplement it) |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `encoding/json` (stdlib) | go1.23 | Read-modify-write for Claude Code `.mcp.json` and OpenCode `opencode.json` | Already used throughout the codebase (`commands.go`, `workspace.go`); no reason to add a JSON library for two simple, flat-ish config files |
| `github.com/BurntSushi/toml` | v1.6.0 [VERIFIED: go.mod, currently `// indirect`] | Read-modify-write for Codex CLI's `config.toml` | Already present in `go.mod`/`go.sum` as a transitive dependency (pulled in by another module, not currently imported by nano-brain code) — promoting it to direct `require` avoids introducing any new supply-chain surface. Confirmed present on the Go module proxy (`proxy.golang.org`) with version history back to v0.1.0; `pkg.go.dev` returns HTTP 200 for the package page. [VERIFIED: go proxy + pkg.go.dev, this session] |
| `bufio.Scanner` + `promptWithDefault` (existing, `cmd/nano-brain/init.go:255`) | n/a (in-repo) | Y/N and free-text prompting | D-07 mandates reuse; already the established pattern, no new TUI/prompt library |
| `isTTY()` (existing, `cmd/nano-brain/client_helpers.go:38`) | n/a (in-repo) | Detect non-interactive stdin/stderr to satisfy D-02 | Already implemented and tested for this exact purpose (`isCharDevice(os.Stdin) && isCharDevice(os.Stderr)`) — do not reimplement |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| None | — | — | No supporting libraries needed beyond the two parsers above |

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| `BurntSushi/toml` for Codex TOML | `github.com/pelletier/go-toml/v2` | Rejected: not currently in the dependency graph at all (would be a genuinely new dependency); `BurntSushi/toml` is already vetted and present transitively, satisfying "don't add a new JSON/TOML library dependency if the stdlib/existing approach works" from CONTEXT.md's open question #4 |
| `map[string]any` generic read-modify-write | Typed structs per client config schema | Rejected: typed structs would silently drop or corrupt any unknown/user-added keys on re-marshal (D-05 explicitly requires preserving unrelated keys/servers untouched) — `map[string]any` is the only approach that round-trips unknown structure safely with stdlib `encoding/json` |

**Installation:**
```bash
# No `go get` needed for BurntSushi/toml — already resolved in go.sum.
# If code imports it, `go mod tidy` will move it from indirect to direct
# in go.mod automatically on next build.
```

**Version verification:**
```bash
grep BurntSushi go.mod
# github.com/BurntSushi/toml v1.6.0 // indirect
```
Confirmed present, version v1.6.0, currently unused directly by any nano-brain source file (`grep -rn "BurntSushi" internal/ cmd/` returns no hits outside `go.mod`/`go.sum`). [VERIFIED: local grep, this session]

## Package Legitimacy Audit

> This phase does NOT add any new external dependency to `go.mod`/`go.sum`. `github.com/BurntSushi/toml` is already present (indirect) and will only be promoted to direct if the planner's tasks import it. Because it is a **Go module, not an npm package**, the `gsd-tools query package-legitimacy check --ecosystem npm` seam does not apply (it is npm-registry-specific and incorrectly reports Go module names as nonexistent npm packages — confirmed by testing `burntsushi-toml` against the npm seam, which returned `SLOP`/`does-not-exist`, a false signal caused by ecosystem mismatch, not an actual problem with the real Go module).

| Package | Registry | Age | Downloads | Source Repo | Verdict | Disposition |
|---------|----------|-----|-----------|-------------|---------|-------------|
| `github.com/BurntSushi/toml` | Go module proxy (proxy.golang.org) | 10+ yrs (versions since v0.1.0) | N/A (Go modules don't report npm-style download counts; widely used — canonical Go TOML parser, referenced by Go's own tooling ecosystem) | github.com/BurntSushi/toml | OK [VERIFIED: go proxy + pkg.go.dev HTTP 200, this session] | Approved — already in dependency graph, no new install needed |

**Packages removed due to [SLOP] verdict:** none.
**Packages flagged as suspicious [SUS]:** none.

*The npm-ecosystem legitimacy seam's `SLOP` result for `burntsushi-toml` is a false positive from ecosystem mismatch (Go module name checked against the npm registry) — not a finding about the actual package. Documented here for audit-trail transparency, not as a real risk signal.*

## Architecture Patterns

### System Architecture Diagram

```
  nano-brain init --root=<path>
            │
            ▼
  ┌─────────────────────────────────────────────────────────┐
  │ cmd/nano-brain/commands.go: runInitCmd (--root branch)   │
  │   POST /api/v1/init  →  result{WorkspaceHash,RootPath,   │
  │                                  Name, AgentsSnippet}    │  <-- Name is NEW
  │   fmt.Println(result.AgentsSnippet)                      │
  └───────────────────┬───────────────────────────────────────┘
                       │ if !jsonFlag && isTTY()
                       ▼
  ┌─────────────────────────────────────────────────────────┐
  │ NEW: promptMCPClientConfig(result.RootPath, result.Name) │
  │   for each client in [ClaudeCode, OpenCode, CodexCLI]:   │
  │     1. detect config path (env var → candidates → Stat)  │  <-- clone of
  │     2. if found: show default; ask "Configure X? [Y/n]"  │      detect.go
  │     3. read existing file (or start empty)                │      pattern (D-08)
  │     4. if "nano-brain" key exists with different          │
  │        ?workspace=: show diff, confirm overwrite (D-06)   │
  │     5. merge: set only the "nano-brain" key, preserve     │
  │        all other keys/servers untouched (D-05)             │
  │     6. write file back (0600, indented)                   │
  └───────────────────┬───────────────────────────────────────┘
                       │
        ┌──────────────┼───────────────┬─────────────────────┐
        ▼                              ▼                     ▼
  .mcp.json (JSON)             opencode.json (JSON)   .codex/config.toml (TOML)
  key: mcpServers              key: mcp                table: mcp_servers.nano-brain
  {"type":"http",              {"type":"remote",        url = "...?workspace=..."
   "url":"...?workspace=..."}   "url":"...?workspace=..."}
```

### Recommended Project Structure

New file, following the existing one-concern-per-file convention seen in `claudecode_init.go` / `opencode_file_init.go` (which handle harvester registration, a related but distinct concern):

```
cmd/nano-brain/
├── commands.go              # MODIFY: call new prompt function from runInitCmd --root branch
├── detect.go                # MODIFY (or new file): add detectClaudeCodeConfigPath,
│                             #   detectOpenCodeConfigPath, detectCodexConfigPath +
│                             #   platformXConfigPaths() helpers (D-08 clone)
├── mcp_client_config.go      # NEW: per-client prompt + read-modify-write logic
│                             #   (promptMCPClientConfig, writeClaudeCodeMCPConfig,
│                             #    writeOpenCodeMCPConfig, writeCodexMCPConfig)
└── mcp_client_config_test.go # NEW: table-driven tests per client + merge-safety cases

internal/server/handlers/
└── workspace.go              # MODIFY: add Name field to initResponse struct (prerequisite)
```

### Pattern 1: Detect-then-prompt-then-merge (per client)

**What:** For each of the three clients, run the same three-step shape: detect whether the client's config file/directory context exists (signal: "is this client installed?"), prompt only if a plausible path was found or the user opted to configure it anyway, then read-modify-write preserving unknown structure.

**When to use:** Any time a new client needs equivalent support added later (Cursor, Hermes Agent per the deferred list) — this pattern should generalize.

**Example — detection clone of `detect.go`'s established shape:**
```go
// Source: pattern cloned from cmd/nano-brain/detect.go:12-24 (detectOpenCodeStorageDir)
// NOTE: this is a NEW function for config-file detection, distinct from the
// existing detectOpenCodeStorageDir which finds session-storage, not config.
func detectOpenCodeConfigPath(projectRoot string) string {
	if v := os.Getenv("OPENCODE_CONFIG"); v != "" {
		if _, err := os.Stat(v); err == nil {
			return v
		}
	}
	// Project-local opencode.json has highest precedence per OpenCode docs.
	local := filepath.Join(projectRoot, "opencode.json")
	if _, err := os.Stat(local); err == nil {
		return local
	}
	// Not found does NOT mean "OpenCode not installed" — project-local config
	// is commonly absent even when OpenCode itself is installed globally.
	// Still offer the project-local path as the *write target* default.
	return local // return the candidate path even if it doesn't exist yet
}
```

**Example — JSON merge preserving unknown keys (Claude Code / OpenCode):**
```go
// Source: pattern derived from CONTEXT.md D-05; encoding/json stdlib docs
// (https://pkg.go.dev/encoding/json#Unmarshal — unmarshaling into map[string]any
// preserves all keys as generic values)
func mergeNanoBrainEntry(configPath, sectionKey, entryKey string, entry map[string]any) error {
	raw := map[string]any{}
	if data, err := os.ReadFile(configPath); err == nil {
		if err := json.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("parse existing config %s: %w", configPath, err)
		}
	}
	section, ok := raw[sectionKey].(map[string]any)
	if !ok {
		section = map[string]any{}
	}
	section[entryKey] = entry
	raw[sectionKey] = section

	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return os.WriteFile(configPath, out, 0600)
}
```

**Example — TOML merge for Codex CLI:**
```go
// Source: pattern derived from BurntSushi/toml README
// (https://pkg.go.dev/github.com/BurntSushi/toml#Decode /
//  https://pkg.go.dev/github.com/BurntSushi/toml#NewEncoder)
func mergeCodexMCPEntry(configPath string, entry map[string]any) error {
	raw := map[string]any{}
	if data, err := os.ReadFile(configPath); err == nil {
		if _, err := toml.Decode(string(data), &raw); err != nil {
			return fmt.Errorf("parse existing config %s: %w", configPath, err)
		}
	}
	servers, ok := raw["mcp_servers"].(map[string]any)
	if !ok {
		servers = map[string]any{}
	}
	servers["nano-brain"] = entry
	raw["mcp_servers"] = servers

	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(raw); err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return os.MkdirAll(filepath.Dir(configPath), 0700), os.WriteFile(configPath, buf.Bytes(), 0600)
}
```
**Caveat (pitfall, see below):** `BurntSushi/toml`'s `Encode` does **not** preserve comments in the original file, and re-serializes the whole map — acceptable per D-05's "preserve keys" requirement (values survive) but user comments in an existing `.codex/config.toml` will be lost on write. Flag this in the confirmation prompt (D-06) if feasible, or at minimum document it.

### Anti-Patterns to Avoid
- **Defining typed Go structs for each client's full config schema:** tempting for type safety, but any unknown/future field in the user's existing file (e.g. Codex's `tools.<tool>.approval_mode`, OpenCode's `oauth` block) would be silently dropped on re-marshal through a typed struct with `json:"-"` gaps. Use `map[string]any` for the whole file; only type the `nano-brain` entry being written.
- **Writing the global `~/.claude.json` in this phase:** explicitly out of scope per D-03 (Claude's Discretion allows offering it as a secondary/optional path, but it's not the default behavior).
- **Guessing the workspace name from `filepath.Base(root)` client-side in `commands.go`:** the CLI has `root` in scope already, so it's *tempting* to just recompute `filepath.Base(root)` locally instead of round-tripping through the API response. **Do not do this** — the server-side `name` in the `workspaces` table is the canonical value (set via `UpsertWorkspace`, and on a second `init` call for the same hash, `ON CONFLICT (hash) DO UPDATE SET name = EXCLUDED.name` means the server always recomputes it the same way today, but future server-side logic — sanitization, collision handling, user-renaming via an API — could diverge). Always read `Name` from the API response, never recompute it in the CLI.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| TOML parsing/writing for Codex CLI config | A hand-rolled TOML string builder (since only one table needs writing) | `github.com/BurntSushi/toml`'s `Decode`/`Encode` into `map[string]any` | TOML has real escaping/quoting rules (dotted keys, string escaping, array-of-tables) that are easy to get subtly wrong by hand; the library is already vetted and present in the dependency graph |
| TTY / non-interactive detection | A new `term.IsTerminal()`-style check via `golang.org/x/term` | Existing `isTTY()` in `client_helpers.go` | Already implemented, tested, and matches this exact requirement (D-02) — adding `golang.org/x/term` would be a genuinely new dependency for a solved problem |
| Y/N prompt with default | A new prompt/survey library (e.g. `survey`, `promptui`) | Existing `promptWithDefault` in `init.go` | D-07 explicitly mandates reuse; the existing helper already handles default-on-empty-input correctly |

**Key insight:** Every "don't hand-roll" item in this phase is about **not introducing new dependencies or new patterns** when the codebase already has a working, tested equivalent one file away. The only genuinely new piece of infrastructure this phase needs is the TOML merge helper, and even that reuses an already-present library.

## Common Pitfalls

### Pitfall 1: OpenCode's MCP schema uses `"type": "remote"`, not `"type": "http"`
**What goes wrong:** `docs/SETUP_AGENT.md`'s existing OpenCode example (Step 9) shows `{"mcp": {"nano-brain": {"type": "http", "url": "..."}}}`. Current OpenCode documentation (opencode.ai/docs/mcp-servers) specifies `"type": "remote"` for HTTP/remote servers and `"type": "local"` for command-based servers — no `"http"`/`"stdio"` naming exists in OpenCode's schema.
**Why it happens:** OpenCode's naming may have changed since SETUP_AGENT.md was written (or SETUP_AGENT.md was written from a different/incorrect reference); either way, this phase's *generated* JSON must not blindly copy the possibly-stale example already in the docs.
**How to avoid:** Generate `"type": "remote"` for the nano-brain entry (HTTP transport) per current official docs. Treat this as a MEDIUM-confidence finding (single-source WebFetch, not independently cross-verified against a live OpenCode binary) — the planner should add a verification step (e.g., a `checkpoint:human-verify` task, or a quick `opencode --version` + schema-validation check if the CLI supports `opencode.json` schema validation) before shipping, and separately consider filing a docs-fix for `SETUP_AGENT.md`'s stale example (existing docs are out of scope for correctness fixes per D-04, but a known-stale example is worth flagging).
**Warning signs:** If OpenCode fails to load the generated config or shows an "unknown MCP type" error, the `type` field naming is the first thing to check.

### Pitfall 2: Codex CLI project-scoped config requires a "trusted project" — auto-write may be silently ignored
**What goes wrong:** Official docs state project-scoped `.codex/config.toml` is honored only for "trusted projects." If Codex CLI has its own trust-registration step (separate from this phase's file write), writing `.codex/config.toml` may have no effect until the user also runs a Codex-specific trust command.
**Why it happens:** This phase only writes the file; it doesn't know Codex CLI's trust model, which wasn't part of the researched scope (research covered config format/location, not runtime trust semantics).
**How to avoid:** Default to writing the **global** `~/.codex/config.toml` unless research/planning explicitly confirms the trust-registration step is a non-issue, OR surface a note in the success message ("If Codex CLI doesn't pick this up, check that this project is trusted: see Codex CLI docs") so the user isn't left confused by a config that appears correct but isn't loaded.
**Warning signs:** User reports nano-brain MCP tools not appearing in Codex CLI despite a correctly-written `.codex/config.toml`.

### Pitfall 3: Workspace `name` is not currently in the `/api/v1/init` JSON response
**What goes wrong:** If a task attempts to build the `?workspace=<name>` URL using only the fields already destructured in `runInitCmd` today (`WorkspaceHash`, `RootPath`, `AgentsSnippet`), there is no `Name` field to use — it will compile-fail or require falling back to the hash (functionally correct per Phase 9's contract, since either name or full hash works, but defeats the human-readable intent of D-03's rationale).
**Why it happens:** `initResponse` (`internal/server/handlers/workspace.go:30-34`) was defined before this phase's requirements existed; `name` is computed and stored (`workspace.go:129`, `UpsertWorkspace`) but was never added to the response DTO.
**How to avoid:** Add `Name string \`json:"name"\`` to `initResponse` server-side, populate it from `ws.Name` (already returned by `UpsertWorkspace`'s `RETURNING` clause — no new query needed), and add the matching field to the CLI's local anonymous decode struct in `runInitCmd`. This is a two-line server change plus a one-line CLI change — small, but easy to miss since it's not in the "existing code to extend" list in CONTEXT.md's canonical_refs.
**Warning signs:** Generated MCP config URLs use the workspace hash instead of the friendlier name, or a task tries to call `filepath.Base(root)` client-side to work around the missing field (anti-pattern, see above).

### Pitfall 4: `BurntSushi/toml`'s `Encode` does not preserve comments or original formatting
**What goes wrong:** If a user has hand-written comments in `~/.codex/config.toml` (e.g., explaining why a server is disabled), a full decode-into-map → encode-from-map round trip will silently drop them, even though all data keys/values survive.
**Why it happens:** TOML's data model (as decoded into a generic map) has no concept of comments; they exist only in the original text and are discarded on parse.
**How to avoid:** This is an accepted, documented tradeoff (same class of limitation as `encoding/json`, which also drops comments — though JSON doesn't support comments in the first place, so this is more of a TOML-specific regression). Mention it in the D-06 confirmation prompt if the existing file has non-empty content, or accept it as a known limitation and document it in the phase's user-facing changelog/release notes.
**Warning signs:** User reports "my comments disappeared" after running `nano-brain init --root=...`.

## Code Examples

### Detecting Claude Code's project-local config path
```go
// Source: pattern derived from CONTEXT.md D-08 + cmd/nano-brain/detect.go's
// existing detectClaudeCodeStorageDir shape (different target path, same shape)
func detectClaudeCodeConfigPath(projectRoot string) string {
	return filepath.Join(projectRoot, ".mcp.json")
}
```
Unlike OpenCode/Codex, Claude Code's project-local `.mcp.json` path has no env-var override or platform-specific candidate list documented — it is always `<project-root>/.mcp.json`. [CITED: code.claude.com/docs/en/mcp]

### Claude Code nano-brain entry shape (HTTP transport)
```json
// Source: code.claude.com/docs/en/mcp (mcpServers schema) + Phase 9's
// ?workspace= URL contract (09-CONTEXT.md)
{
  "mcpServers": {
    "nano-brain": {
      "type": "http",
      "url": "http://localhost:3100/mcp?workspace=nano-brain"
    }
  }
}
```

### OpenCode nano-brain entry shape (remote transport)
```json
// Source: opencode.ai/docs/mcp-servers (type: "remote" schema, MEDIUM confidence —
// single-source, not independently cross-verified against a live install)
{
  "mcp": {
    "nano-brain": {
      "type": "remote",
      "url": "http://localhost:3100/mcp?workspace=nano-brain",
      "enabled": true
    }
  }
}
```

### Codex CLI nano-brain entry shape (TOML, HTTP transport)
```toml
# Source: developers.openai.com/codex/mcp (mcp_servers.<name> table, url field
# implies HTTP transport — no explicit type/transport key needed)
[mcp_servers.nano-brain]
url = "http://localhost:3100/mcp?workspace=nano-brain"
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|---------------|--------|
| Manual per-client config editing (SETUP_AGENT.md Step 9, human-driven) | Automated, interactive, idempotent CLI prompt (this phase) | This phase | Reduces onboarding friction from "read docs, hand-edit 3 JSON/TOML files across 3 different schemas" to "answer Y/N per client" |
| Global-only `~/.claude.json` MCP binding (single `?workspace=` for all projects) | Project-local config, unlocked by Phase 9's `?workspace=` param | Phase 9 (PR #524) → this phase automates it | Multiple registered projects can each have a correctly-scoped MCP connection simultaneously |

**Deprecated/outdated:**
- `docs/SETUP_AGENT.md`'s OpenCode `"type": "http"` example may be stale against OpenCode's current `"type": "remote"` schema — not being fixed in this phase (D-04: docs are additive, not being corrected), but the planner should be aware the generated config and the doc's own example will diverge intentionally.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | OpenCode's MCP schema uses `"type": "remote"` (not `"http"`) for HTTP-transport servers | Common Pitfalls #1, Code Examples | If OpenCode actually still accepts `"type": "http"` (e.g., doc was updated ahead of a shipped release, or backward-compat was kept), the generated config could still work by accident — but if OpenCode strictly validates `"type"` against an enum, a wrong value breaks tool discovery entirely. Medium risk; single-source WebFetch, not cross-verified against a second independent source or a live OpenCode binary. |
| A2 | Codex CLI's project-scoped `.codex/config.toml` "trusted projects only" constraint won't silently block this phase's auto-written config from being picked up | Common Pitfalls #2 | If Codex CLI requires a separate manual trust step nano-brain can't automate, users may see "it didn't work" despite a correctly-written file, generating support burden. Low-medium risk; only one official doc page was consulted, and Codex's trust mechanism itself wasn't researched in depth (out of the explicit research scope in CONTEXT.md). |
| A3 | `BurntSushi/toml`'s `Encode` on a `map[string]any` round-trips all existing keys/values correctly (only comments are lost) | Standard Stack, Pitfall #4 | If `Encode` mishandles some TOML value type (e.g., nested inline tables, arrays of tables) when round-tripping through `map[string]any`, an existing user's `.codex/config.toml` could be corrupted or restructured unexpectedly on write. Low risk (well-established, decade-old library) but not verified against an actual complex real-world Codex config file in this session — recommend the planner add a unit test with a realistic multi-server `.codex/config.toml` fixture, not just an empty-file case. |

**None of these assumptions are compliance/security/retention-policy claims** — all three are technical schema/behavior questions with a single-source or partially-verified basis. The planner and discuss-phase (if reopened) should treat A1 in particular as needing either a live-install check or a `checkpoint:human-verify` task before this phase ships, since it directly determines whether the generated OpenCode config actually works.

## Open Questions

1. **Does OpenCode still accept the older `"type": "http"` naming for backward compatibility?**
   - What we know: current official docs specify `"remote"`/`"local"`.
   - What's unclear: whether `"http"` also still parses (silently accepted, deprecated-but-working) or is now a hard validation error.
   - Recommendation: Generate `"type": "remote"` (current docs) but consider a `checkpoint:human-verify` task in the plan asking the user to confirm nano-brain tools appear in OpenCode after this phase runs, rather than blocking implementation on further research.

2. **Does Codex CLI need a separate trust-registration step for project-scoped `.codex/config.toml` to be honored?**
   - What we know: docs say project-scoped config is honored "for trusted projects only."
   - What's unclear: whether this trust is automatic (e.g., based on being inside a git repo, or the user having previously run `codex` in that directory) or requires an explicit command this phase would also need to invoke.
   - Recommendation: Default to writing `~/.codex/config.toml` (global) rather than the project-scoped variant unless/until this is clarified — global config has no trust gate mentioned in the docs, and still achieves "Codex CLI is configured for nano-brain," even if it can't carry a project-specific `?workspace=` default as cleanly as the other two clients. Alternatively, write project-scoped AND surface the trust caveat in the CLI's success message. Either is planner's discretion; document the choice in the plan.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| `github.com/BurntSushi/toml` (Go module) | Codex CLI config read-modify-write | Yes (already in go.sum, indirect) | v1.6.0 | None needed — already resolved |
| Claude Code binary | End-user verification that generated `.mcp.json` works | Not checked (not required for this phase — file generation doesn't need the client installed) | — | N/A — config is written regardless of whether the client is installed; D-08's detection is a UX nicety (default path / skip-if-absent), not a hard requirement |
| OpenCode binary | Same as above | Not checked | — | N/A, same reasoning |
| Codex CLI binary | Same as above | Not checked | — | N/A, same reasoning |

**Missing dependencies with no fallback:** none.
**Missing dependencies with fallback:** none — this phase writes config files independent of whether the target client binary is actually installed on the machine (D-08 only uses detection to decide whether to *prompt*, not to gate whether writing is possible).

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + `github.com/stretchr/testify` (existing, see go.mod) |
| Config file | none — table-driven tests, no separate test framework config |
| Quick run command | `go test -race -short ./cmd/nano-brain/...` |
| Full suite command | `go test -race -tags=integration ./...` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| D-02 | Skip prompting when `--json` or non-TTY | unit | `go test -race -run TestRunInitCmd_JSONFlagSkipsPrompt ./cmd/nano-brain/...` | ❌ Wave 0 — new test in `commands_test.go` or new `mcp_client_config_test.go` |
| D-05 | Read-modify-write preserves unrelated keys | unit | `go test -race -run TestMergeNanoBrainEntry_PreservesUnknownKeys ./cmd/nano-brain/...` | ❌ Wave 0 |
| D-05 | Re-running against already-configured client is a no-op | unit | `go test -race -run TestMergeNanoBrainEntry_Idempotent ./cmd/nano-brain/...` | ❌ Wave 0 |
| D-06 | Overwrite confirmation shown when `?workspace=` value differs | unit | `go test -race -run TestPromptMCPClientConfig_ConfirmsOverwrite ./cmd/nano-brain/...` | ❌ Wave 0 |
| Workspace `name` gap | `initResponse` includes `Name`, CLI decodes it | unit | `go test -race -run TestInitWorkspace_ResponseIncludesName ./internal/server/handlers/...` | ❌ Wave 0 — extend existing `workspace_test.go` if present, else new |
| Codex TOML merge | Round-trips a realistic multi-server `.codex/config.toml` fixture without corruption | unit | `go test -race -run TestMergeCodexMCPEntry_RealisticFixture ./cmd/nano-brain/...` | ❌ Wave 0 |

### Sampling Rate
- **Per task commit:** `go build ./... && go test -race -short ./cmd/nano-brain/... ./internal/server/handlers/...`
- **Per wave merge:** `go test -race -tags=integration ./...`
- **Phase gate:** Full suite green before `/gsd-verify-work`

### Wave 0 Gaps
- [ ] `cmd/nano-brain/mcp_client_config_test.go` — covers D-05, D-06 merge/idempotency/overwrite-confirmation behavior for all three clients
- [ ] `cmd/nano-brain/detect_test.go` extension — covers new `detectClaudeCodeConfigPath`/`detectOpenCodeConfigPath`/`detectCodexConfigPath` helpers (existing file, add cases)
- [ ] `internal/server/handlers/workspace_test.go` extension (or new) — covers `initResponse.Name` field addition
- [ ] Realistic TOML fixture file (e.g. `testdata/codex_config_multi_server.toml`) with 2+ existing `[mcp_servers.*]` tables, comments, and mixed stdio/HTTP entries — needed to test Pitfall #4/Assumption A3 meaningfully (an empty-file test alone does not validate round-trip fidelity)
- [ ] Framework install: none — `testing` + `testify` already present, no new install needed

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | No | This phase writes local config files; no new auth surface |
| V3 Session Management | No | N/A |
| V4 Access Control | No | N/A — file writes are scoped to files the invoking user already owns/can write |
| V5 Input Validation | Yes | Workspace `name` (from `filepath.Base(absPath)`, server-controlled) is interpolated into a URL query string (`?workspace=<name>`); Go's `net/url.Values.Encode()` (or manual `url.QueryEscape`) should be used when constructing the URL to avoid injecting unescaped characters into the generated JSON/TOML config if a workspace name ever contains special characters (unlikely for `filepath.Base` output, but directory names can contain spaces, unicode, etc.) |
| V6 Cryptography | No | N/A — no secrets are generated or stored by this phase; the MCP URL itself carries no credentials (matches existing SETUP_AGENT.md examples, which are also unauthenticated localhost URLs by default) |

### Known Threat Patterns for this stack

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Config file permission leakage (world-readable `.mcp.json`/`config.toml` containing a workspace-scoped URL) | Information Disclosure | Write with `0600` permissions (matches existing `os.WriteFile(configPath, []byte(yaml), 0600)` pattern already used in `runInteractiveInit`) — the URL itself is low-sensitivity (no auth token, localhost-only by default) but consistent-permissions hygiene still applies |
| Path traversal via a maliciously crafted `--root` value influencing the derived config path | Tampering | `filepath.Join(projectRoot, ".mcp.json")` where `projectRoot` is already `filepath.Abs`-resolved server-side (`workspace.go:120`) before this phase's code ever sees it — no additional sanitization needed since the abs-path resolution already happened upstream in the existing `/api/v1/init` handler |
| TOCTOU on read-modify-write (another process modifies the config file between read and write) | Tampering | Low risk for a single-user interactive CLI flow (not a multi-process daemon writing this file); explicitly out of scope — no file locking needed for a one-shot interactive `init` command |

## Sources

### Primary (HIGH confidence)
- code.claude.com/docs/en/mcp — Claude Code `.mcp.json` project-scope schema, `mcpServers` shape
- Local codebase inspection: `cmd/nano-brain/commands.go`, `init.go`, `detect.go`, `client_helpers.go`, `claudecode_init.go`, `opencode_file_init.go`, `internal/server/handlers/workspace.go`, `internal/storage/sqlc/workspaces.sql.go`, `internal/storage/sqlc/models.go`, `go.mod`
- go proxy (proxy.golang.org) + pkg.go.dev — `github.com/BurntSushi/toml` existence/version verification

### Secondary (MEDIUM confidence)
- opencode.ai/docs/mcp-servers, opencode.ai/docs/config — OpenCode config location, precedence order, `mcp` schema (`type: "remote"`/`"local"`) — single-source WebFetch, not cross-verified against a second independent doc or live install
- developers.openai.com/codex/mcp — Codex CLI `config.toml` location (global + project-scoped) and `[mcp_servers.<name>]` schema — single-source WebFetch

### Tertiary (LOW confidence)
- None used as load-bearing claims — all WebSearch-only results were superseded by the WebFetch of primary docs above.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — `BurntSushi/toml` presence verified directly via go.mod/go proxy; `encoding/json`, `bufio.Scanner`, `isTTY()` are existing, verified in-repo code
- Architecture: HIGH — trigger point, existing helper reuse, and the workspace-name gap are all directly verified against current source
- Pitfalls: MEDIUM — OpenCode schema and Codex trust-model pitfalls are single-source findings (see Assumptions Log A1/A2); JSON/workspace-name pitfalls are HIGH (directly verified in code)

**Research date:** 2026-07-01
**Valid until:** 30 days (stable domain — CLI config schemas for established tools change slowly, but OpenCode is a fast-moving project; re-verify A1 if implementation is delayed more than a few weeks)
</content>
