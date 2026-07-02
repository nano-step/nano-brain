package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

// initResult mirrors the JSON shape returned by POST /api/v1/init.
type initResult struct {
	WorkspaceHash string `json:"workspace_hash"`
	RootPath      string `json:"root_path"`
	Name          string `json:"name"`
	AgentsSnippet string `json:"agents_snippet"`
}

// registerWorkspace performs the workspace-registration HTTP flow shared by
// `nano-brain init --root` and the interactive init wizard's register step
// (D-15): POST /api/v1/init, parse the result, optionally prompt for MCP
// client auto-configuration (D-16, consent-gated, empty-name-guarded), and
// trigger background reindex/harvest.
//
// When jsonFlag is true, the raw response is printed and a zero initResult
// is returned without prompting for MCP config, matching the existing
// `--json` short-circuit behavior.
func registerWorkspace(root, workspace string, jsonFlag bool) (initResult, error) {
	body := map[string]string{"root_path": root}
	if workspace != "" {
		body["workspace"] = workspace
	}
	data, err := json.Marshal(body)
	if err != nil {
		return initResult{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	resp, _, err := doRequest("POST", getBaseURL()+"/api/v1/init", bytes.NewReader(data))
	if err != nil {
		return initResult{}, err
	}

	if jsonFlag {
		fmt.Println(string(resp))
		return initResult{}, nil
	}

	var result initResult
	if err := json.Unmarshal(resp, &result); err != nil {
		fmt.Println(string(resp))
		return initResult{}, nil
	}
	fmt.Printf("Workspace registered: %s\n", result.WorkspaceHash)
	fmt.Printf("Root path: %s\n", result.RootPath)
	fmt.Println()
	fmt.Println(result.AgentsSnippet)

	if shouldPromptMCPConfig(jsonFlag, isTTYFn()) {
		if result.Name == "" {
			// A server started before this CLI version was upgraded won't
			// have returned a name (field didn't exist yet) — writing a
			// ?workspace= URL with an empty name would silently produce a
			// broken binding in every accepted client's config.
			fmt.Println("Warning: server did not return a workspace name (server may need restarting) — skipping MCP client auto-configuration.")
		} else {
			promptMCPClientConfig(bufio.NewScanner(os.Stdin), result.RootPath, result.Name)
		}
	}

	triggerInitBackground(result.WorkspaceHash, root)

	return result, nil
}
