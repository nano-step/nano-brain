package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// buildWorkspaceURL appends a ?workspace=<name> query parameter to base,
// URL-escaping name so workspace names containing spaces or other special
// characters cannot inject invalid or unexpected URL structure into the
// generated client config (V5 input-validation mitigation, T-10-06).
func buildWorkspaceURL(base, name string) string {
	return base + "?workspace=" + url.QueryEscape(name)
}

// mergeJSONMCPEntry reads configPath (if it exists), decodes it into a
// generic map[string]any (never a typed struct — see RESEARCH
// Anti-Patterns: typed structs silently drop unknown user keys), sets only
// raw[sectionKey]["nano-brain"] = entry preserving every other key/section
// untouched, and writes the result back with 0600 permissions after
// ensuring the parent directory exists with 0700 (T-10-05).
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

	if mkdirErr := os.MkdirAll(filepath.Dir(configPath), 0700); mkdirErr != nil {
		return false, "", fmt.Errorf("create config dir: %w", mkdirErr)
	}
	if writeErr := os.WriteFile(configPath, out, 0600); writeErr != nil {
		return false, "", fmt.Errorf("write config %s: %w", configPath, writeErr)
	}

	return changed, oldURL, nil
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
// top-level key untouched, and writes the result back with 0600
// permissions after ensuring the parent directory exists with 0700
// (T-10-05). Same (changed, oldURL, err) contract as mergeJSONMCPEntry.
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

	if mkdirErr := os.MkdirAll(filepath.Dir(configPath), 0700); mkdirErr != nil {
		return false, "", fmt.Errorf("create config dir: %w", mkdirErr)
	}
	if writeErr := os.WriteFile(configPath, buf.Bytes(), 0600); writeErr != nil {
		return false, "", fmt.Errorf("write config %s: %w", configPath, writeErr)
	}

	return changed, oldURL, nil
}

// writeCodexMCPConfig writes/updates the global Codex CLI MCP config
// (~/.codex/config.toml, or $CODEX_HOME/config.toml) with a nano-brain
// entry bound to workspaceName. Codex's schema infers HTTP transport from
// the presence of the "url" field alone — no explicit "type" key is used
// (RESEARCH Code Examples).
func writeCodexMCPConfig(baseMCPURL, workspaceName string) (changed bool, oldURL string, configPath string, err error) {
	configPath = detectCodexConfigPath()
	entry := map[string]any{
		"url": buildWorkspaceURL(baseMCPURL, workspaceName),
	}
	changed, oldURL, err = mergeCodexTOMLEntry(configPath, entry)
	return changed, oldURL, configPath, err
}
