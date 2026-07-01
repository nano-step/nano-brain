---
phase: 10-interactive-mcp-client-auto-configuration-after-workspace-re
reviewed: 2026-07-01T00:00:00Z
depth: deep
files_reviewed: 9
files_reviewed_list:
  - cmd/nano-brain/mcp_client_config.go
  - cmd/nano-brain/mcp_client_config_test.go
  - cmd/nano-brain/testdata/codex_config_multi_server.toml
  - cmd/nano-brain/detect.go
  - cmd/nano-brain/detect_test.go
  - cmd/nano-brain/commands.go
  - go.mod
  - internal/server/handlers/workspace.go
  - internal/server/handlers/workspace_test.go
findings:
  critical: 1
  warning: 3
  info: 2
  total: 6
status: issues_found
---

# Phase 10: Code Review Report

**Reviewed:** 2026-07-01
**Depth:** deep
**Files Reviewed:** 9 (+ AGENTS.md, README.md, docs/SETUP_AGENT.md, docs/reference-readme.md reviewed as docs-only diffs)
**Status:** issues_found

## Summary

Reviewed the full diff for PR #526 (`feat/mcp-client-auto-config`) against `origin/master`, covering `cmd/nano-brain/mcp_client_config.go` (new), `detect.go`, `commands.go` wiring, the `internal/server/handlers/workspace.go` API surface change, and the doc/type fixes. `go build ./...` and `go test -race -short -count=1 ./cmd/nano-brain/... ./internal/server/handlers/...` both pass cleanly. The read-modify-write merge logic for JSON (Claude Code, OpenCode) and TOML (Codex) is solid — unrelated keys/sections/servers are preserved, idempotency holds (verified with an additional test against the realistic multi-server TOML fixture, not just the trivial single-entry case already in the test suite), and workspace names are properly URL-escaped before being embedded in the generated URL (mitigates injection into the URL/query string).

The one BLOCKER found is behavioral, not in the code the author wrote directly but in how the new interactive flow *uses* an existing helper (`promptWithDefault`): on stdin EOF (dropped SSH session, Ctrl-D, script that closes stdin early), the prompt silently defaults to "yes" for every remaining unanswered prompt in the sequence — including the D-06 overwrite-confirmation prompt — meaning a non-interactive termination of the input stream can cause unattended writes/overwrites to three separate client config files. This directly undermines the PR's own stated contract ("must never prompt/write without genuine interactive confirmation") and is reproducible (see PoC below). It was not caught by the existing test suite because every test always supplies exactly the right number of "y\n"/"n\n" lines.

Three WARNING-level and two INFO-level items round out the report — none block functionality on the happy path, but they represent real gaps: permissions aren't tightened on pre-existing config files, an empty/missing workspace name from a version-skewed server is not guarded against, and an AGENTS.md doc reference points at a nonexistent slash command.

## Critical Issues

### CR-01: Stdin EOF during the MCP client prompt sequence silently defaults every remaining prompt to "yes" and writes/overwrites config files without real user consent

**File:** `cmd/nano-brain/mcp_client_config.go:326-355` (via `promptWithDefault` in `cmd/nano-brain/init.go:255-267`)

**Issue:** `promptAndWrite` asks each per-client Y/N question — and, when an existing differing nano-brain entry is found, a second D-06 "Overwrite existing nano-brain entry?" confirmation — via `promptWithDefault(scanner, prompt, "Y")`. That helper treats `scanner.Scan() == false` (EOF, i.e. the underlying reader has no more data) exactly the same as the user pressing Enter to accept the default:

```go
func promptWithDefault(scanner *bufio.Scanner, prompt, defaultVal string) string {
	...
	scanner.Scan()
	input := strings.TrimSpace(scanner.Text())
	if input == "" {
		return defaultVal   // also reached when Scan() returned false (EOF)
	}
	return input
}
```

`isAffirmative("Y")` is `true`, so every prompt after stdin closes is treated as an affirmative "yes" — both the initial "Configure X?" question and the D-06 overwrite confirmation. Reproduced directly against the real functions (not a hypothetical): feeding `promptMCPClientConfig` a scanner over `"y\n"` (one answer, then EOF, simulating a dropped session after the first prompt) causes OpenCode's and Codex's configs to be silently written even though the user never answered those prompts:

```
── MCP client configuration ──
  Configure Claude Code for this workspace? [Y]:   Claude Code configured: .../.mcp.json
  Configure OpenCode for this workspace? [Y]:   OpenCode configured: .../opencode.json
  Configure Codex CLI for this workspace? [Y]:   Codex CLI configured: .../config.toml
```
Both `opencode.json` and `~/.codex/config.toml` get written with zero real input. Worse, this same code path drives the D-06 "Overwrite existing nano-brain entry?" confirmation — meaning an EOF mid-sequence can silently clobber an existing, different `nano-brain` entry in a client's config (e.g. one pointed at a different workspace) that the user never agreed to overwrite. This directly contradicts the PR's own documented decision (D-06: "ask before overwriting") and the task's "never clobber without consent" requirement.

This is a pre-existing helper reused from the single-prompt `init.go` flow (where an EOF-as-default was low-stakes: "register workspace directory?" with no write side effect), but this PR is the first caller to chain it into a 3-6-step interactive sequence that writes files to disk on "yes", including a confirm-before-overwrite gate whose entire purpose is to prevent unconsented writes.

**Fix:** Distinguish "user pressed Enter" from "stdin closed" in `promptWithDefault` (or add a purpose-built variant for consequential prompts), and treat EOF as a decline/abort rather than an accept:

```go
// promptWithDefault reads one line; `ok` is false when the scanner hit
// EOF/error and no line could be read at all (distinct from an empty line).
func promptWithDefault(scanner *bufio.Scanner, prompt, defaultVal string) (answer string, ok bool) {
	if defaultVal != "" {
		fmt.Printf("  %s [%s]: ", prompt, defaultVal)
	} else {
		fmt.Printf("  %s: ", prompt)
	}
	if !scanner.Scan() {
		return "", false // EOF or read error — caller must not treat as "yes"
	}
	input := strings.TrimSpace(scanner.Text())
	if input == "" {
		return defaultVal, true
	}
	return input, true
}
```
And in `promptAndWrite`/the D-06 confirm, bail out (treat as decline, stop processing remaining clients) when `ok` is false, rather than falling through to `isAffirmative(defaultVal)`.

## Warnings

### WR-01: Writing to a pre-existing config file does not tighten its permissions to 0600 as documented

**File:** `cmd/nano-brain/mcp_client_config.go:72` (`mergeJSONMCPEntry`), `:175` (`mergeCodexTOMLEntry`)

**Issue:** Both merge functions doc-comment that they write "with 0600 permissions" (see lines 28 and 127-128), but `os.WriteFile(configPath, out, 0600)` only applies the mode when *creating* a new file — per POSIX `open(2)`/Go semantics, the mode argument is ignored when the file already exists. Verified directly: seeding `.mcp.json` at `0644` and running `mergeJSONMCPEntry` leaves it at `0644` after the "secure" 0600 write. Any pre-existing `.mcp.json`/`opencode.json`/`~/.codex/config.toml` created with looser permissions (common — many editors and other CLI tools default to 0644) stays world/group-readable even after nano-brain writes its entry into it. Low severity here because the URL written isn't itself a secret (localhost MCP endpoint + workspace name), but it contradicts the stated design intent and could matter more if any client later stores a token/bearer credential alongside the nano-brain entry (the Codex fixture already shows sibling servers using `bearer_token_env_var` in this same file).

**Fix:** Explicitly `os.Chmod(configPath, 0600)` after `os.WriteFile` (or before, doesn't matter) regardless of whether the file was pre-existing:
```go
if writeErr := os.WriteFile(configPath, out, 0600); writeErr != nil {
	return false, "", fmt.Errorf("write config %s: %w", configPath, writeErr)
}
if chmodErr := os.Chmod(configPath, 0600); chmodErr != nil {
	return false, "", fmt.Errorf("chmod config %s: %w", configPath, chmodErr)
}
```

### WR-02: Empty/missing workspace `name` from a version-skewed server writes a broken `?workspace=` URL with no guard or warning

**File:** `cmd/nano-brain/commands.go:117-119`, `cmd/nano-brain/mcp_client_config.go:19-21`

**Issue:** `promptMCPClientConfig(scanner, result.RootPath, result.Name)` is called unconditionally whenever `shouldPromptMCPConfig` is true, with no check that `result.Name` is non-empty. `result.Name` is populated by unmarshaling the `/api/v1/init` JSON response into a struct with a `Name` field that only exists as of this PR's `internal/server/handlers/workspace.go` change. Server and CLI binaries are released together via the npm auto-publish pipeline (per `AGENTS.md`'s release flow), but users can absolutely have an already-running older server process (`nano-brain serve` started before upgrading, or a long-running Docker container) fielding requests from a freshly-upgraded CLI. In that case the server's JSON response has no `name` key, `json.Unmarshal` leaves `result.Name` as its zero value `""`, and `buildWorkspaceURL` happily produces `.../mcp?workspace=` — an empty, non-functional workspace binding — which then gets written into all three accepted clients' configs with no error or warning to the user.

**Fix:** Guard before prompting:
```go
if result.Name == "" {
	fmt.Println("Warning: server did not return a workspace name (server may need restarting) — skipping MCP client auto-configuration.")
} else if shouldPromptMCPConfig(jsonFlag, isTTY()) {
	promptMCPClientConfig(bufio.NewScanner(os.Stdin), result.RootPath, result.Name)
}
```

### WR-03: `mergeJSONMCPEntry` and `mergeCodexTOMLEntry` duplicate near-identical read/decode/compare/write logic

**File:** `cmd/nano-brain/mcp_client_config.go:35-77` and `:138-180`

**Issue:** The two functions are structurally identical (read-if-exists → decode → get-or-create sub-map → diff existing "nano-brain" entry via `mapsEqual` → write back → mkdir/chmod), differing only in the decode/encode calls (`encoding/json` vs `BurntSushi/toml`) and the top-level key name (`sectionKey` parameter vs hardcoded `"mcp_servers"`). This isn't a bug, but the duplication means any future fix to one (e.g. the WR-01 chmod fix above, or error-wrapping conventions) has to be manually mirrored in the other, which is exactly the kind of drift this codebase's `AGENTS.md` conventions ("fix every sibling occurrence, not just one site") warn about.

**Fix:** Not blocking; consider extracting a shared helper parameterized by decode/encode functions if a third format is ever added, or at minimum leave a comment cross-referencing the sibling function so future edits stay in sync.

## Info

### IN-01: AGENTS.md's new GSD phase loop references a slash command that doesn't exist

**File:** `AGENTS.md:341` (`## Development Workflow` → step 5) and `:406` (Harness `### Flow` → step 7)

**Issue:** This PR replaces the old OpenSpec-First workflow section with a "GSD Core Phase Loop," whose final step reads `5. **Ship** → \`/gsd-ship-phase\` — create PR, archive phase` (and again as step 7 in the Harness Flow section). The actual installed command in `.claude/commands/` is `gsd-ship.md` (invoked as `/gsd-ship`), not `/gsd-ship-phase`. This is a new typo introduced by this diff (the prior OpenSpec text referenced `/opsx-apply`/`/opsx-propose`, which did exist), so anyone following the new AGENTS.md instructions verbatim will hit a "command not found."

**Fix:** `s/gsd-ship-phase/gsd-ship/` in both locations.

### IN-02: `promptAndWrite`'s D-06 overwrite-confirm message doesn't distinguish "declined the first prompt" from "declined the overwrite confirm" in its return value

**File:** `cmd/nano-brain/mcp_client_config.go:326-355`

**Issue:** `promptAndWrite` returns a single `bool` ("was the client's config actually written"), collapsing three distinct outcomes — declined the initial "Configure X?" prompt, declined the overwrite confirmation, and "already configured, no change" — into `false` in all three cases (only distinguished by which `fmt.Printf` line was already shown to the user). This is fine for the current single caller (`promptMCPClientConfig` only branches on the Codex case's `codexChanged` for the "global config" note), but it's a minor code-smell: a caller wanting to react differently to "user declined" vs "already up to date" has no way to do so without re-parsing stdout.

**Fix:** Not urgent; if more branching logic is ever needed here, consider a small result enum (`declined` / `declinedOverwrite` / `noChange` / `written`) instead of a bare bool.

---

_Reviewed: 2026-07-01_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: deep_
