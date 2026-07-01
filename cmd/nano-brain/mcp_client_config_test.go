package main

import (
	"bufio"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

func TestBuildWorkspaceURL(t *testing.T) {
	got := buildWorkspaceURL("http://localhost:3100/mcp", "nano-brain")
	want := "http://localhost:3100/mcp?workspace=nano-brain"
	if got != want {
		t.Errorf("buildWorkspaceURL() = %q, want %q", got, want)
	}
}

func TestBuildWorkspaceURL_EscapesSpecialChars(t *testing.T) {
	got := buildWorkspaceURL("http://localhost:3100/mcp", "my project")
	want := "http://localhost:3100/mcp?workspace=my+project"
	if got != want {
		t.Errorf("buildWorkspaceURL() = %q, want %q (space must be escaped)", got, want)
	}
	if parsed, err := url.Parse(got); err != nil || parsed.Query().Get("workspace") != "my project" {
		t.Errorf("buildWorkspaceURL() produced an unparseable/incorrect URL: %q (err=%v)", got, err)
	}
}

func TestMergeJSONMCPEntry_CreatesNewFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".mcp.json")
	entry := map[string]any{"type": "http", "url": "http://localhost:3100/mcp?workspace=foo"}

	changed, oldURL, err := mergeJSONMCPEntry(configPath, "mcpServers", entry)
	if err != nil {
		t.Fatalf("mergeJSONMCPEntry() error = %v", err)
	}
	if !changed {
		t.Error("mergeJSONMCPEntry() changed = false, want true (new file)")
	}
	if oldURL != "" {
		t.Errorf("mergeJSONMCPEntry() oldURL = %q, want \"\" (no prior entry)", oldURL)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read written config: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal written config: %v", err)
	}
	servers, ok := raw["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("mcpServers section missing or wrong type: %#v", raw["mcpServers"])
	}
	nb, ok := servers["nano-brain"].(map[string]any)
	if !ok {
		t.Fatalf("nano-brain entry missing or wrong type: %#v", servers["nano-brain"])
	}
	if nb["type"] != "http" {
		t.Errorf("nano-brain.type = %v, want http", nb["type"])
	}
}

func TestMergeJSONMCPEntry_PreservesUnrelatedKeys(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".mcp.json")
	initial := `{
  "mcpServers": {
    "other-server": {"type": "stdio", "command": "some-cmd"}
  },
  "unrelatedTopLevelKey": "keep-me"
}`
	if err := os.WriteFile(configPath, []byte(initial), 0o644); err != nil {
		t.Fatalf("write initial config: %v", err)
	}

	entry := map[string]any{"type": "http", "url": "http://localhost:3100/mcp?workspace=foo"}
	if _, _, err := mergeJSONMCPEntry(configPath, "mcpServers", entry); err != nil {
		t.Fatalf("mergeJSONMCPEntry() error = %v", err)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read written config: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal written config: %v", err)
	}
	if raw["unrelatedTopLevelKey"] != "keep-me" {
		t.Errorf("unrelatedTopLevelKey = %v, want keep-me", raw["unrelatedTopLevelKey"])
	}
	servers, ok := raw["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("mcpServers section missing: %#v", raw)
	}
	other, ok := servers["other-server"].(map[string]any)
	if !ok {
		t.Fatalf("other-server entry dropped: %#v", servers)
	}
	if other["command"] != "some-cmd" {
		t.Errorf("other-server.command = %v, want some-cmd", other["command"])
	}
	if _, ok := servers["nano-brain"]; !ok {
		t.Error("nano-brain entry was not added")
	}
}

func TestMergeJSONMCPEntry_Idempotent(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".mcp.json")
	entry := map[string]any{"type": "http", "url": "http://localhost:3100/mcp?workspace=foo"}

	if _, _, err := mergeJSONMCPEntry(configPath, "mcpServers", entry); err != nil {
		t.Fatalf("first merge error = %v", err)
	}
	firstBytes, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read after first merge: %v", err)
	}

	changed, _, err := mergeJSONMCPEntry(configPath, "mcpServers", entry)
	if err != nil {
		t.Fatalf("second merge error = %v", err)
	}
	if changed {
		t.Error("second mergeJSONMCPEntry() changed = true, want false (idempotent)")
	}
	secondBytes, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read after second merge: %v", err)
	}
	if string(firstBytes) != string(secondBytes) {
		t.Errorf("file bytes differ after idempotent re-run:\nfirst:  %s\nsecond: %s", firstBytes, secondBytes)
	}
}

func TestMergeJSONMCPEntry_DetectsURLChange(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".mcp.json")
	original := map[string]any{"type": "http", "url": "http://localhost:3100/mcp?workspace=old-name"}
	if _, _, err := mergeJSONMCPEntry(configPath, "mcpServers", original); err != nil {
		t.Fatalf("initial merge error = %v", err)
	}

	updated := map[string]any{"type": "http", "url": "http://localhost:3100/mcp?workspace=new-name"}
	changed, oldURL, err := mergeJSONMCPEntry(configPath, "mcpServers", updated)
	if err != nil {
		t.Fatalf("update merge error = %v", err)
	}
	if !changed {
		t.Error("mergeJSONMCPEntry() changed = false, want true (url differs)")
	}
	if oldURL != "http://localhost:3100/mcp?workspace=old-name" {
		t.Errorf("oldURL = %q, want the prior url", oldURL)
	}
}

func TestWriteClaudeCodeMCPConfig_Shape(t *testing.T) {
	dir := t.TempDir()
	changed, _, configPath, err := writeClaudeCodeMCPConfig(dir, "http://localhost:3100/mcp", "my-workspace")
	if err != nil {
		t.Fatalf("writeClaudeCodeMCPConfig() error = %v", err)
	}
	if !changed {
		t.Error("writeClaudeCodeMCPConfig() changed = false, want true")
	}
	wantPath := filepath.Join(dir, ".mcp.json")
	if configPath != wantPath {
		t.Errorf("configPath = %q, want %q", configPath, wantPath)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	nb := raw["mcpServers"].(map[string]any)["nano-brain"].(map[string]any)
	if nb["type"] != "http" {
		t.Errorf("Claude Code type = %v, want http", nb["type"])
	}
	if nb["url"] != "http://localhost:3100/mcp?workspace=my-workspace" {
		t.Errorf("Claude Code url = %v, want the workspace-bound url", nb["url"])
	}
}

func TestWriteOpenCodeMCPConfig_Shape(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OPENCODE_CONFIG", "")
	changed, _, configPath, err := writeOpenCodeMCPConfig(dir, "http://localhost:3100/mcp", "my-workspace")
	if err != nil {
		t.Fatalf("writeOpenCodeMCPConfig() error = %v", err)
	}
	if !changed {
		t.Error("writeOpenCodeMCPConfig() changed = false, want true")
	}
	wantPath := filepath.Join(dir, "opencode.json")
	if configPath != wantPath {
		t.Errorf("configPath = %q, want %q", configPath, wantPath)
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	nb := raw["mcp"].(map[string]any)["nano-brain"].(map[string]any)
	if nb["type"] != "remote" {
		t.Errorf("OpenCode type = %v, want remote (NOT http) -- Pitfall 1", nb["type"])
	}
	if nb["enabled"] != true {
		t.Errorf("OpenCode enabled = %v, want true", nb["enabled"])
	}
	if nb["url"] != "http://localhost:3100/mcp?workspace=my-workspace" {
		t.Errorf("OpenCode url = %v, want the workspace-bound url", nb["url"])
	}
}

func TestMergeCodexTOMLEntry_CreatesNewFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	entry := map[string]any{"url": "http://localhost:3100/mcp?workspace=foo"}

	changed, oldURL, err := mergeCodexTOMLEntry(configPath, entry)
	if err != nil {
		t.Fatalf("mergeCodexTOMLEntry() error = %v", err)
	}
	if !changed {
		t.Error("mergeCodexTOMLEntry() changed = false, want true (new file)")
	}
	if oldURL != "" {
		t.Errorf("mergeCodexTOMLEntry() oldURL = %q, want \"\"", oldURL)
	}

	raw := map[string]any{}
	if _, err := toml.DecodeFile(configPath, &raw); err != nil {
		t.Fatalf("decode written config: %v", err)
	}
	servers, ok := raw["mcp_servers"].(map[string]any)
	if !ok {
		t.Fatalf("mcp_servers table missing: %#v", raw)
	}
	nb, ok := servers["nano-brain"].(map[string]any)
	if !ok {
		t.Fatalf("nano-brain table missing: %#v", servers)
	}
	if nb["url"] != "http://localhost:3100/mcp?workspace=foo" {
		t.Errorf("nano-brain.url = %v, want the workspace url", nb["url"])
	}
}

func TestMergeCodexTOMLEntry_RealisticRoundTrip(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	fixture, err := os.ReadFile(filepath.Join("testdata", "codex_config_multi_server.toml"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if err := os.WriteFile(configPath, fixture, 0o644); err != nil {
		t.Fatalf("write fixture copy: %v", err)
	}

	entry := map[string]any{"url": "http://localhost:3100/mcp?workspace=my-workspace"}
	changed, oldURL, err := mergeCodexTOMLEntry(configPath, entry)
	if err != nil {
		t.Fatalf("mergeCodexTOMLEntry() error = %v", err)
	}
	if !changed {
		t.Error("mergeCodexTOMLEntry() changed = false, want true")
	}
	if oldURL != "" {
		t.Errorf("oldURL = %q, want \"\" (no prior nano-brain entry)", oldURL)
	}

	raw := map[string]any{}
	if _, err := toml.DecodeFile(configPath, &raw); err != nil {
		t.Fatalf("decode written config: %v", err)
	}

	if raw["model"] != "o3" {
		t.Errorf("top-level model key dropped: got %v, want o3", raw["model"])
	}

	servers, ok := raw["mcp_servers"].(map[string]any)
	if !ok {
		t.Fatalf("mcp_servers table missing after merge: %#v", raw)
	}

	foo, ok := servers["foo"].(map[string]any)
	if !ok {
		t.Fatalf("pre-existing [mcp_servers.foo] dropped: %#v", servers)
	}
	if foo["command"] != "npx" {
		t.Errorf("foo.command = %v, want npx", foo["command"])
	}
	fooArgs, ok := foo["args"].([]any)
	if !ok || len(fooArgs) != 2 || fooArgs[0] != "-y" || fooArgs[1] != "@some/mcp-server" {
		t.Errorf("foo.args = %#v, want [-y @some/mcp-server]", foo["args"])
	}

	bar, ok := servers["bar"].(map[string]any)
	if !ok {
		t.Fatalf("pre-existing [mcp_servers.bar] dropped: %#v", servers)
	}
	if bar["url"] != "https://example.com/mcp" {
		t.Errorf("bar.url = %v, want https://example.com/mcp", bar["url"])
	}
	if bar["bearer_token_env_var"] != "BAR_TOKEN" {
		t.Errorf("bar.bearer_token_env_var = %v, want BAR_TOKEN", bar["bearer_token_env_var"])
	}

	nb, ok := servers["nano-brain"].(map[string]any)
	if !ok {
		t.Fatalf("nano-brain table not added: %#v", servers)
	}
	if nb["url"] != "http://localhost:3100/mcp?workspace=my-workspace" {
		t.Errorf("nano-brain.url = %v, want the workspace url", nb["url"])
	}
}

func TestMergeCodexTOMLEntry_Idempotent(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	entry := map[string]any{"url": "http://localhost:3100/mcp?workspace=foo"}

	if _, _, err := mergeCodexTOMLEntry(configPath, entry); err != nil {
		t.Fatalf("first merge error = %v", err)
	}
	firstBytes, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read after first merge: %v", err)
	}

	changed, _, err := mergeCodexTOMLEntry(configPath, entry)
	if err != nil {
		t.Fatalf("second merge error = %v", err)
	}
	if changed {
		t.Error("second mergeCodexTOMLEntry() changed = true, want false (idempotent)")
	}
	secondBytes, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("read after second merge: %v", err)
	}
	if string(firstBytes) != string(secondBytes) {
		t.Errorf("file bytes differ after idempotent re-run:\nfirst:  %s\nsecond: %s", firstBytes, secondBytes)
	}
}

func TestMergeCodexTOMLEntry_DetectsURLChange(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	original := map[string]any{"url": "http://localhost:3100/mcp?workspace=old-name"}
	if _, _, err := mergeCodexTOMLEntry(configPath, original); err != nil {
		t.Fatalf("initial merge error = %v", err)
	}

	updated := map[string]any{"url": "http://localhost:3100/mcp?workspace=new-name"}
	changed, oldURL, err := mergeCodexTOMLEntry(configPath, updated)
	if err != nil {
		t.Fatalf("update merge error = %v", err)
	}
	if !changed {
		t.Error("mergeCodexTOMLEntry() changed = false, want true (url differs)")
	}
	if oldURL != "http://localhost:3100/mcp?workspace=old-name" {
		t.Errorf("oldURL = %q, want the prior url", oldURL)
	}
}

func TestWriteCodexMCPConfig_Shape(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.toml")
	t.Setenv("CODEX_HOME", dir)

	changed, _, gotPath, err := writeCodexMCPConfig("http://localhost:3100/mcp", "my-workspace")
	if err != nil {
		t.Fatalf("writeCodexMCPConfig() error = %v", err)
	}
	if !changed {
		t.Error("writeCodexMCPConfig() changed = false, want true")
	}
	if gotPath != configPath {
		t.Errorf("configPath = %q, want %q", gotPath, configPath)
	}

	raw := map[string]any{}
	if _, err := toml.DecodeFile(configPath, &raw); err != nil {
		t.Fatalf("decode written config: %v", err)
	}
	servers := raw["mcp_servers"].(map[string]any)
	nb := servers["nano-brain"].(map[string]any)
	if nb["url"] != "http://localhost:3100/mcp?workspace=my-workspace" {
		t.Errorf("Codex url = %v, want the workspace-bound url", nb["url"])
	}
	if _, hasType := nb["type"]; hasType {
		t.Error("Codex entry should not have an explicit type key (transport inferred from url presence)")
	}
}

func TestShouldPromptMCPConfig(t *testing.T) {
	tests := []struct {
		name     string
		jsonFlag bool
		tty      bool
		want     bool
	}{
		{"tty and not json", false, true, true},
		{"json flag suppresses even on tty", true, true, false},
		{"non-tty suppresses even without json", false, false, false},
		{"json and non-tty both suppress", true, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shouldPromptMCPConfig(tt.jsonFlag, tt.tty)
			if got != tt.want {
				t.Errorf("shouldPromptMCPConfig(%v, %v) = %v, want %v", tt.jsonFlag, tt.tty, got, tt.want)
			}
		})
	}
}

func TestPromptMCPClientConfig_DeclineAllWritesNothing(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OPENCODE_CONFIG", "")
	codexHome := t.TempDir()
	t.Setenv("CODEX_HOME", codexHome)

	scanner := bufio.NewScanner(strings.NewReader("n\nn\nn\n"))
	promptMCPClientConfig(scanner, dir, "my-workspace")

	if _, err := os.Stat(filepath.Join(dir, ".mcp.json")); !os.IsNotExist(err) {
		t.Errorf(".mcp.json should not exist after declining all, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "opencode.json")); !os.IsNotExist(err) {
		t.Errorf("opencode.json should not exist after declining all, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(codexHome, "config.toml")); !os.IsNotExist(err) {
		t.Errorf("config.toml should not exist after declining all, stat err = %v", err)
	}
}

func TestPromptMCPClientConfig_SelectiveYesWritesOnlyThatClient(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("OPENCODE_CONFIG", "")
	codexHome := t.TempDir()
	t.Setenv("CODEX_HOME", codexHome)

	// y for Claude Code, n for OpenCode, n for Codex.
	scanner := bufio.NewScanner(strings.NewReader("y\nn\nn\n"))
	promptMCPClientConfig(scanner, dir, "my-workspace")

	if _, err := os.Stat(filepath.Join(dir, ".mcp.json")); err != nil {
		t.Errorf(".mcp.json should exist after accepting Claude Code, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "opencode.json")); !os.IsNotExist(err) {
		t.Errorf("opencode.json should not exist after declining OpenCode, stat err = %v", err)
	}
	if _, err := os.Stat(filepath.Join(codexHome, "config.toml")); !os.IsNotExist(err) {
		t.Errorf("config.toml should not exist after declining Codex, stat err = %v", err)
	}
}

func TestPromptMCPClientConfig_OverwriteConfirm_Declined(t *testing.T) {
	dir := t.TempDir()
	// Pre-seed .mcp.json with a nano-brain entry bound to a different workspace.
	if _, _, _, err := writeClaudeCodeMCPConfig(dir, "http://localhost:3100/mcp", "old-workspace"); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	before, err := os.ReadFile(filepath.Join(dir, ".mcp.json"))
	if err != nil {
		t.Fatalf("read seeded config: %v", err)
	}

	// y to configure Claude Code, then n to decline the overwrite confirm; n, n for the other two clients.
	scanner := bufio.NewScanner(strings.NewReader("y\nn\nn\nn\n"))
	promptMCPClientConfig(scanner, dir, "new-workspace")

	after, err := os.ReadFile(filepath.Join(dir, ".mcp.json"))
	if err != nil {
		t.Fatalf("read config after decline: %v", err)
	}
	if string(before) != string(after) {
		t.Errorf("file changed after declining overwrite confirm:\nbefore: %s\nafter:  %s", before, after)
	}
}

func TestPromptMCPClientConfig_OverwriteConfirm_Accepted(t *testing.T) {
	dir := t.TempDir()
	if _, _, _, err := writeClaudeCodeMCPConfig(dir, "http://localhost:3100/mcp", "old-workspace"); err != nil {
		t.Fatalf("seed config: %v", err)
	}

	// y to configure Claude Code, y to confirm overwrite; n, n for the other two clients.
	scanner := bufio.NewScanner(strings.NewReader("y\ny\nn\nn\n"))
	promptMCPClientConfig(scanner, dir, "new-workspace")

	data, err := os.ReadFile(filepath.Join(dir, ".mcp.json"))
	if err != nil {
		t.Fatalf("read config after accept: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	nb := raw["mcpServers"].(map[string]any)["nano-brain"].(map[string]any)
	if nb["url"] != "http://localhost:3100/mcp?workspace=new-workspace" {
		t.Errorf("nano-brain.url = %v, want the new workspace url after confirmed overwrite", nb["url"])
	}
}
