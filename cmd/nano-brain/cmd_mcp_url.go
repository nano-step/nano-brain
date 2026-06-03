package main

import (
	"fmt"
	"os"
	"strings"
)

func resolveMCPURL(dockerenvPath string) string {
	if v := strings.TrimSpace(os.Getenv("NANO_BRAIN_MCP_URL")); v != "" {
		return v
	}
	if _, err := os.Stat(dockerenvPath); err == nil {
		return "http://host.docker.internal:3100/mcp"
	}
	return "http://localhost:3100/mcp"
}

func runMCPURLCmd(args []string) {
	if len(args) > 0 {
		fmt.Fprintln(os.Stderr, "Usage: nano-brain mcp-url")
		os.Exit(1)
	}
	fmt.Println(resolveMCPURL("/.dockerenv"))
}
