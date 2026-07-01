# Phase 10: Interactive MCP client auto-configuration - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-07-01
**Phase:** 10-Interactive MCP client auto-configuration
**Areas discussed:** Trigger point, config target (global vs project-local), idempotency/merge safety, UX pattern

Mode: autonomous (per project's Autonomous Delivery Protocol) — decisions locked from direct codebase research performed in-session rather than interactive prompts, consistent with how Phase 9 was discussed.

---

## Trigger point — where does the new prompting hook in?

| Option | Description | Selected |
|--------|-------------|----------|
| `--root` branch of `runInitCmd` | Fires right after workspace registration succeeds, when the workspace hash/root path are already known | ✓ |
| The no-`--root` interactive server wizard (`runInteractiveInit`) | Wrong lifecycle stage — that flow configures DB/embedding/server settings before any workspace exists | |

**Selected:** Hook into `runInitCmd`'s `--root` branch, after the existing success output. Skip entirely when `--json` is passed or stdin isn't a TTY (preserves the existing non-interactive/scripted contract).

---

## Config target — global `~/.claude.json` or project-local `.mcp.json`?

| Option | Description | Selected |
|--------|-------------|----------|
| Project-local config file per registered project | Each project can carry its own correctly-scoped `?workspace=` binding | ✓ |
| Global `~/.claude.json` | Can only hold ONE `?workspace=` default across all projects sharing the daemon | |

**Selected:** Project-local. This is the direct payoff of Phase 9's `?workspace=` feature — a global entry can't bind more than one project at a time, so automating the global file would actually be less useful than automating project-local files, one per registered project. The existing docs recommending the global file for single-project users are left as-is (not wrong for that case), this phase adds the project-local alternative.

---

## Idempotency — how does re-running avoid corrupting existing config?

| Option | Description | Selected |
|--------|-------------|----------|
| Read-modify-write, touch only the `nano-brain` key | Preserves any other MCP servers/keys already in the file | ✓ |
| Always overwrite the whole file | Risks destroying a user's other MCP server entries or hand-edited config | |

**Selected:** Read-modify-write. Confirm-before-overwrite when an existing `nano-brain` entry would change (mirrors `runInteractiveInit`'s existing "Config exists... Overwrite?" pattern).

---

## UX pattern — new prompt library or reuse existing?

| Option | Description | Selected |
|--------|-------------|----------|
| Reuse `bufio.Scanner` + `promptWithDefault` from `init.go` | Zero new dependencies, consistent with the rest of the CLI | ✓ |
| New multi-select TUI component | More polished but unrequested complexity for a 3-client list | |

**Selected:** Reuse existing pattern — per-client Y/N prompts, same style as the existing `ocSessionDir`/`ccSessionDir` prompts.

## Claude's Discretion

- Exact prompt/message wording.
- Whether to also offer configuring the global `~/.claude.json` as a secondary option.

## Deferred Ideas

- `curl | bash` one-line installer (separate distribution/packaging concern, explicitly named by the user but scoped to a future phase per issue #525).
- Additional client support beyond Claude Code/OpenCode/Codex CLI (Cursor, Hermes Agent, etc.).
- Global `~/.claude.json` auto-configuration as an addition.
