package main

import (
	"fmt"
	"os"
	"path/filepath"
)

const openCodeBootstrapCommand = `---
description: Bootstrap nano-brain in this workspace
---

Set up nano-brain for the current workspace.

1. If ~/.nano-brain/config.yml exists and .nanobrain/config.yml does not, seed the workspace-local config from it.
2. Run: nano-brain --config .nanobrain/config.yml init
3. Keep the resulting config local to this workspace.
`

func runInitInstallCmd(target string) {
	workspaceRoot, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: could not determine current workspace: %v\n", err)
		os.Exit(1)
	}

	installPath, err := installInitTarget(target, workspaceRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Installed OpenCode bootstrap at %s\n", installPath)
	fmt.Println("Run /nano-brain . in OpenCode to bootstrap this workspace.")
}

func installInitTarget(target, workspaceRoot string) (string, error) {
	switch target {
	case "opencode":
		return installOpenCodeBootstrap(workspaceRoot)
	default:
		return "", fmt.Errorf("unsupported init target %q", target)
	}
}

func installOpenCodeBootstrap(workspaceRoot string) (string, error) {
	commandsDir := filepath.Join(workspaceRoot, ".opencode", "commands")
	installPath := filepath.Join(commandsDir, "nano-brain.md")

	if err := os.MkdirAll(commandsDir, 0o755); err != nil {
		return "", fmt.Errorf("create OpenCode commands directory: %w", err)
	}

	if err := os.WriteFile(installPath, []byte(openCodeBootstrapCommand), 0o644); err != nil {
		return "", fmt.Errorf("write OpenCode bootstrap: %w", err)
	}

	return installPath, nil
}
