package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

// buildWorkspaceURL sets a workspace=<name> query parameter on base via
// net/url rather than raw string concatenation, so a base URL that already
// carries a query string (e.g. a custom NANO_BRAIN_MCP_URL with its own
// ?token=... in a VPS/team setup) gets an additional &-joined parameter
// instead of a malformed double-"?" URL. Falls back to simple
// concatenation only if base fails to parse at all (V5 input-validation
// mitigation, T-10-06).
func buildWorkspaceURL(base, name string) string {
	u, err := url.Parse(base)
	if err != nil {
		return base + "?workspace=" + url.QueryEscape(name)
	}
	q := u.Query()
	q.Set("workspace", name)
	u.RawQuery = q.Encode()
	return u.String()
}

// mergeJSONMCPEntry reads configPath (if it exists), decodes it into a
// generic map[string]any (never a typed struct — see RESEARCH
// Anti-Patterns: typed structs silently drop unknown user keys), sets only
// raw[sectionKey]["nano-brain"] = entry preserving every other key/section
// untouched, and writes the result back via writeMCPConfigFile (mode rules
// there).
//
// Returns changed=true when the serialized nano-brain entry differs from
// what was already present (or was absent), and oldURL set to the prior
// entry's "url" field (if any) so the caller can drive the D-06 overwrite
// confirmation. changed=false + this being a no-op write is what makes
// re-running the command idempotent (D-05).
func mergeJSONMCPEntry(configPath, sectionKey string, entry map[string]any) (changed bool, oldURL string, err error) {
	raw := map[string]any{}
	if data, readErr := os.ReadFile(configPath); readErr == nil {
		if len(data) > 0 {
			if unmarshalErr := json.Unmarshal(data, &raw); unmarshalErr != nil {
				return false, "", fmt.Errorf("parse existing config %s: %w", configPath, unmarshalErr)
			}
		}
	} else if !os.IsNotExist(readErr) {
		return false, "", fmt.Errorf("read existing config %s: %w", configPath, readErr)
	}

	if raw == nil { // file content was JSON null
		raw = map[string]any{}
	}
	section, ok := raw[sectionKey].(map[string]any)
	if !ok {
		section = map[string]any{}
	}

	if existing, ok := section["nano-brain"].(map[string]any); ok {
		if u, ok := existing["url"].(string); ok {
			oldURL = u
		}
		changed = !mapsEqual(existing, entry)
	} else {
		changed = true
	}

	section["nano-brain"] = entry
	raw[sectionKey] = section

	out, marshalErr := json.MarshalIndent(raw, "", "  ")
	if marshalErr != nil {
		return false, "", fmt.Errorf("marshal config: %w", marshalErr)
	}

	if err := writeMCPConfigFile(configPath, out, entry); err != nil {
		return false, "", err
	}
	return changed, oldURL, nil
}

// writeMCPConfigFile writes data to configPath, creating the parent dir 0700.
// A new file is created 0600. An existing file keeps its mode (a committed
// .mcp.json in a shared repo must not flip to 0600 in git, #725) unless the
// entry may carry a credential, in which case it is tightened to 0600 (T-10-05).
func writeMCPConfigFile(configPath string, data []byte, entry map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(configPath), 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	// Tighten before writing so the token is never readable under the old mode.
	if entryHasSecret(entry) {
		if err := os.Chmod(configPath, 0600); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("chmod config %s: %w", configPath, err)
		}
	}
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("write config %s: %w", configPath, err)
	}
	return nil
}

// entryHasSecret fails closed: any string value that is not a plain URL
// (unparseable, has userinfo, or a query key other than workspace) may hold
// a credential, e.g. a NANO_BRAIN_MCP_URL carrying ?token=... .
func entryHasSecret(entry map[string]any) bool {
	for _, v := range entry {
		s, ok := v.(string)
		if !ok {
			continue
		}
		u, err := url.Parse(s)
		if err != nil || u.User != nil {
			return true
		}
		for k := range u.Query() {
			if k != "workspace" {
				return true
			}
		}
	}
	return false
}

// mapsEqual reports whether two map[string]any values are deeply equal by
// round-tripping both through JSON marshal, which normalizes key order and
// numeric representation, avoiding reflect.DeepEqual's sensitivity to
// differing concrete numeric types (float64 vs int) that commonly arise
// when comparing a freshly-built entry against one decoded from JSON.
func mapsEqual(a, b map[string]any) bool {
	aj, aErr := json.Marshal(a)
	bj, bErr := json.Marshal(b)
	if aErr != nil || bErr != nil {
		return false
	}
	return string(aj) == string(bj)
}

// writeClaudeCodeMCPConfig writes/updates the project-local Claude Code
// MCP config (.mcp.json) with a nano-brain entry bound to workspaceName.
// Claude Code's schema places server entries under "mcpServers", using
// "type": "http" for HTTP-transport servers.
func writeClaudeCodeMCPConfig(projectRoot, baseMCPURL, workspaceName string) (changed bool, oldURL string, configPath string, err error) {
	configPath = detectClaudeCodeConfigPath(projectRoot)
	entry := map[string]any{
		"type": "http",
		"url":  buildWorkspaceURL(baseMCPURL, workspaceName),
	}
	changed, oldURL, err = mergeJSONMCPEntry(configPath, "mcpServers", entry)
	return changed, oldURL, configPath, err
}

// writeOpenCodeMCPConfig writes/updates the project-local OpenCode MCP
// config (opencode.json) with a nano-brain entry bound to workspaceName.
// OpenCode's schema places server entries under "mcp", and REQUIRES
// "type": "remote" for HTTP-transport servers (NOT "http" — RESEARCH
// Pitfall 1 / Common Pitfalls #1). enabled:true is set explicitly.
func writeOpenCodeMCPConfig(projectRoot, baseMCPURL, workspaceName string) (changed bool, oldURL string, configPath string, err error) {
	configPath = detectOpenCodeConfigPath(projectRoot)
	entry := map[string]any{
		"type":    "remote",
		"url":     buildWorkspaceURL(baseMCPURL, workspaceName),
		"enabled": true,
	}
	changed, oldURL, err = mergeJSONMCPEntry(configPath, "mcp", entry)
	return changed, oldURL, configPath, err
}

// mergeCodexTOMLEntry reads configPath (if it exists), decodes it into a
// generic map[string]any via BurntSushi/toml, sets only
// raw["mcp_servers"]["nano-brain"] = entry preserving every other table and
// top-level key untouched, and writes the result back via writeMCPConfigFile.
// Same (changed, oldURL, err) contract as mergeJSONMCPEntry.
//
// KNOWN LIMITATION (RESEARCH Pitfall 4 / A3, threat T-10-07, accepted):
// BurntSushi/toml's Encode does not preserve comments or original
// formatting from the source file. All data keys/values survive the
// decode->encode round trip (verified by the realistic multi-server
// fixture test), but any hand-written comments in an existing
// ~/.codex/config.toml are lost on write. This is an accepted, documented
// tradeoff — callers should warn the user when overwriting a non-empty
// existing file.
func mergeCodexTOMLEntry(configPath string, entry map[string]any) (changed bool, oldURL string, err error) {
	raw := map[string]any{}
	if data, readErr := os.ReadFile(configPath); readErr == nil {
		if len(data) > 0 {
			if _, decodeErr := toml.Decode(string(data), &raw); decodeErr != nil {
				return false, "", fmt.Errorf("parse existing config %s: %w", configPath, decodeErr)
			}
		}
	} else if !os.IsNotExist(readErr) {
		return false, "", fmt.Errorf("read existing config %s: %w", configPath, readErr)
	}

	servers, ok := raw["mcp_servers"].(map[string]any)
	if !ok {
		servers = map[string]any{}
	}

	if existing, ok := servers["nano-brain"].(map[string]any); ok {
		if u, ok := existing["url"].(string); ok {
			oldURL = u
		}
		changed = !mapsEqual(existing, entry)
	} else {
		changed = true
	}

	servers["nano-brain"] = entry
	raw["mcp_servers"] = servers

	var buf bytes.Buffer
	if encodeErr := toml.NewEncoder(&buf).Encode(raw); encodeErr != nil {
		return false, "", fmt.Errorf("marshal config: %w", encodeErr)
	}

	if err := writeMCPConfigFile(configPath, buf.Bytes(), entry); err != nil {
		return false, "", err
	}
	return changed, oldURL, nil
}

// writeCodexMCPConfigTo writes/updates the Codex CLI MCP config at
// configPath with a nano-brain entry whose url is mcpURL exactly as given
// (the caller decides whether it carries a ?workspace= binding). Codex's
// schema infers HTTP transport from the presence of the "url" field alone —
// no explicit "type" key is used (RESEARCH Code Examples).
func writeCodexMCPConfigTo(configPath, mcpURL string) (changed bool, oldURL string, err error) {
	entry := map[string]any{"url": mcpURL}
	return mergeCodexTOMLEntry(configPath, entry)
}

// shouldPromptMCPConfig reports whether promptMCPClientConfig should run at
// all: only when --json was NOT passed AND stdin/stderr are both a TTY
// (D-02). Non-interactive callers (scripts/CI) get no behavior change.
func shouldPromptMCPConfig(jsonFlag, tty bool) bool {
	return !jsonFlag && tty
}

// isAffirmative reports whether a promptWithDefault-style answer means
// "yes", mirroring the existing "n"/"N" == no convention used throughout
// init.go (any other non-empty, non-"n" answer, or the "Y" default itself,
// means yes).
func isAffirmative(answer string) bool {
	return answer != "n" && answer != "N"
}

// promptMCPClientConfig is the D-01/D-07 orchestrator: for each of the
// three supported clients (Claude Code, OpenCode, Codex CLI) it asks a Y/N
// prompt via the injected scanner, and on "yes" reads-modifies-writes that
// client's config bound to workspaceName. Before overwriting an existing
// nano-brain entry whose url differs from the one about to be written, it
// peeks the existing config, shows the old vs new url, and asks a second
// Y/N confirmation (D-06) BEFORE calling the write func — a decline never
// touches the file.
//
// The scanner is injectable so tests can feed canned answers against a
// temp projectRoot; production callers pass bufio.NewScanner(os.Stdin).
func promptMCPClientConfig(scanner *bufio.Scanner, projectRoot, workspaceName string) {
	baseURL := resolveMCPURL("/.dockerenv")

	fmt.Println()
	fmt.Println("── MCP client configuration ──")

	claudeURL := buildWorkspaceURL(baseURL, workspaceName)
	promptAndWrite(scanner, "Claude Code", claudeURL, func() (string, string) {
		return peekExistingJSONURL(detectClaudeCodeConfigPath(projectRoot), "mcpServers"), ""
	}, func() (bool, string, string, error) {
		return writeClaudeCodeMCPConfig(projectRoot, baseURL, workspaceName)
	})

	openCodeURL := buildWorkspaceURL(baseURL, workspaceName)
	promptAndWrite(scanner, "OpenCode", openCodeURL, func() (string, string) {
		return peekExistingJSONURL(detectOpenCodeConfigPath(projectRoot), "mcp"), ""
	}, func() (bool, string, string, error) {
		return writeOpenCodeMCPConfig(projectRoot, baseURL, workspaceName)
	})

	promptAndWriteCodex(scanner, baseURL, workspaceName, projectRoot)

	// Windsurf/Devin mcp_config.json is global (one file for every project),
	// so like global Codex it gets the bare URL with no ?workspace= binding.
	for _, configPath := range detectWindsurfConfigPaths() {
		promptAndWrite(scanner, "Windsurf/Devin ("+configPath+")", baseURL, func() (string, string) {
			return peekExistingJSONURL(configPath, "mcpServers"), ""
		}, func() (bool, string, string, error) {
			changed, oldURL, err := mergeJSONMCPEntry(configPath, "mcpServers", map[string]any{"serverUrl": baseURL})
			return changed, oldURL, configPath, err
		})
	}
}

// promptAndWriteCodex handles Codex CLI specially: unlike Claude Code and
// OpenCode (always project-local), Codex config can be GLOBAL
// (~/.codex/config.toml, shared by every project) or PROJECT-scoped
// (<project>/.codex/config.toml). A single global config can only hold one
// nano-brain entry, so binding it to one project's ?workspace= would make
// every other project's Codex point at the wrong workspace. To avoid that,
// the user picks the scope: global writes a bare URL with NO ?workspace=
// (the agent passes a workspace per call), project writes a workspace-bound
// URL but requires trusting the directory (Codex's trusted-project gate).
func promptAndWriteCodex(scanner *bufio.Scanner, baseURL, workspaceName, projectRoot string) {
	answer, ok := promptConsequential(scanner, "Configure Codex CLI for this workspace?", "Y")
	if !ok || !isAffirmative(answer) {
		return
	}

	scope, ok := promptConsequential(scanner, "Codex config scope — [g]lobal (shared, no workspace binding) or [p]roject (this dir, workspace-bound)?", "g")
	if !ok {
		fmt.Println("  Skipped Codex CLI.")
		return
	}

	project := strings.EqualFold(scope, "p") || strings.EqualFold(scope, "project")
	var configPath, newURL string
	if project {
		configPath = detectCodexProjectConfigPath(projectRoot)
		newURL = buildWorkspaceURL(baseURL, workspaceName)
	} else {
		// Global config is shared across every project, so it carries NO
		// ?workspace= — otherwise each init would override the previous
		// project's binding. The agent supplies the workspace per call.
		configPath = detectCodexConfigPath()
		newURL = baseURL
	}

	if oldURL, nonEmpty := peekExistingCodexURL(configPath); oldURL != "" && oldURL != newURL {
		fmt.Printf("  Codex CLI already has a nano-brain entry:\n    current: %s\n    new:     %s\n", oldURL, newURL)
		if nonEmpty {
			fmt.Println("  Note: rewriting this file may drop any hand-written comments (data keys/values are preserved).")
		}
		confirm, ok := promptConsequential(scanner, "Overwrite existing nano-brain entry?", "Y")
		if !ok || !isAffirmative(confirm) {
			fmt.Println("  Skipped Codex CLI (kept existing entry).")
			return
		}
	}

	changed, _, err := writeCodexMCPConfigTo(configPath, newURL)
	if err != nil {
		fmt.Printf("  Error configuring Codex CLI: %v\n", err)
		return
	}
	if !changed {
		fmt.Println("  Codex CLI already configured (no change).")
		return
	}
	fmt.Printf("  Codex CLI configured: %s\n", configPath)
	if project {
		fmt.Println("  Note: Codex only loads project config from a TRUSTED directory — trust this project in Codex if the entry doesn't take effect.")
	} else {
		fmt.Println("  Note: global Codex config carries no ?workspace= (shared across all projects) — pass a workspace per tool call, or re-run with project scope for a bound default.")
	}
}

// writeFunc performs one client's config write, returning whether the
// nano-brain entry changed, the previous url (if any, for the D-06
// confirm), the config path written, and any error.
type writeFunc func() (changed bool, oldURL string, configPath string, err error)

// peekFunc peeks a client's existing config (without modifying it) and
// returns the prior nano-brain "url" (empty if none) plus an optional
// extra caveat line to print alongside the D-06 confirm (e.g. Codex's
// comment-loss warning). Used so promptAndWrite can ask its overwrite
// confirmation BEFORE any write happens.
type peekFunc func() (oldURL, caveat string)

// peekExistingJSONURL reads configPath (without modifying it) and returns
// the "url" (or Windsurf's "serverUrl") field of any existing raw[sectionKey]["nano-brain"] entry, or
// "" if the file/section/entry doesn't exist. Used to drive the D-06
// overwrite confirmation BEFORE any write happens.
func peekExistingJSONURL(configPath, sectionKey string) string {
	data, err := os.ReadFile(configPath)
	if err != nil || len(data) == 0 {
		return ""
	}
	raw := map[string]any{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return ""
	}
	section, ok := raw[sectionKey].(map[string]any)
	if !ok {
		return ""
	}
	nb, ok := section["nano-brain"].(map[string]any)
	if !ok {
		return ""
	}
	u, _ := nb["url"].(string)
	if u == "" {
		u, _ = nb["serverUrl"].(string) // Windsurf/Devin key
	}
	return u
}

// peekExistingCodexURL is peekExistingJSONURL's TOML equivalent for the
// Codex CLI config, plus a report of whether the file is non-empty (used
// to surface the comment-loss caveat, T-10-07).
func peekExistingCodexURL(configPath string) (oldURL string, nonEmpty bool) {
	data, err := os.ReadFile(configPath)
	if err != nil || len(data) == 0 {
		return "", false
	}
	raw := map[string]any{}
	if _, err := toml.Decode(string(data), &raw); err != nil {
		return "", true
	}
	servers, ok := raw["mcp_servers"].(map[string]any)
	if !ok {
		return "", true
	}
	nb, ok := servers["nano-brain"].(map[string]any)
	if !ok {
		return "", true
	}
	u, _ := nb["url"].(string)
	return u, true
}

// promptConsequential reads one line like promptWithDefault, but — unlike
// that helper — distinguishes "user pressed Enter" (ok=true, defaultVal)
// from "stdin closed" (ok=false). This file's two prompts gate file writes,
// including an overwrite of an existing nano-brain entry (D-06); silently
// treating EOF as an accept (promptWithDefault's normal contract) would let
// a dropped session or closed stdin stream cause unconsented writes/
// overwrites to disk (CR-01). Callers must treat ok=false as a decline.
func promptConsequential(scanner *bufio.Scanner, prompt, defaultVal string) (answer string, ok bool) {
	fmt.Printf("  %s [%s]: ", prompt, defaultVal)
	if !scanner.Scan() {
		return "", false
	}
	input := strings.TrimSpace(scanner.Text())
	if input == "" {
		return defaultVal, true
	}
	return input, true
}

// promptAndWrite asks the per-client Y/N prompt via the injected scanner.
// On "yes", it calls peek to check for an existing nano-brain entry with a
// different url; if found, it shows old vs new (plus peek's optional
// caveat line) and asks the D-06 overwrite confirmation BEFORE calling
// write at all — declining never touches the file. Returns whether the
// client's config was actually written.
func promptAndWrite(scanner *bufio.Scanner, clientLabel, newURL string, peek peekFunc, write writeFunc) bool {
	answer, ok := promptConsequential(scanner, fmt.Sprintf("Configure %s for this workspace?", clientLabel), "Y")
	if !ok || !isAffirmative(answer) {
		return false
	}

	if oldURL, caveat := peek(); oldURL != "" && oldURL != newURL {
		fmt.Printf("  %s already has a nano-brain entry:\n    current: %s\n    new:     %s\n", clientLabel, oldURL, newURL)
		if caveat != "" {
			fmt.Printf("  Note: %s.\n", caveat)
		}
		confirm, ok := promptConsequential(scanner, "Overwrite existing nano-brain entry?", "Y")
		if !ok || !isAffirmative(confirm) {
			fmt.Printf("  Skipped %s (kept existing entry).\n", clientLabel)
			return false
		}
	}

	changed, _, writtenPath, err := write()
	if err != nil {
		fmt.Printf("  Error configuring %s: %v\n", clientLabel, err)
		return false
	}
	if !changed {
		fmt.Printf("  %s already configured for this workspace (no change).\n", clientLabel)
		return false
	}
	fmt.Printf("  %s configured: %s\n", clientLabel, writtenPath)
	return true
}
