package main

import (
	"os"
	"testing"
)

func TestResolveMCPURL_EnvVarWins(t *testing.T) {
	t.Setenv("NANO_BRAIN_MCP_URL", "http://custom-host:9999/mcp")
	got := resolveMCPURL("/.dockerenv")
	want := "http://custom-host:9999/mcp"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveMCPURL_EnvVarWhitespaceTrimmed(t *testing.T) {
	t.Setenv("NANO_BRAIN_MCP_URL", "  http://trimmed:8888/mcp  ")
	got := resolveMCPURL("/.dockerenv")
	want := "http://trimmed:8888/mcp"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveMCPURL_EnvVarEmptyFallsThrough(t *testing.T) {
	t.Setenv("NANO_BRAIN_MCP_URL", "")
	got := resolveMCPURL("/.dockerenv-nonexistent-path-xyz")
	want := "http://localhost:3100/mcp"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveMCPURL_DockerEnvDetected(t *testing.T) {
	t.Setenv("NANO_BRAIN_MCP_URL", "")

	tmp, err := os.CreateTemp(t.TempDir(), ".dockerenv-*")
	if err != nil {
		t.Fatal(err)
	}
	tmp.Close()

	got := resolveMCPURL(tmp.Name())
	want := "http://host.docker.internal:3100/mcp"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestResolveMCPURL_Default(t *testing.T) {
	t.Setenv("NANO_BRAIN_MCP_URL", "")
	got := resolveMCPURL("/this/path/does/not/exist/.dockerenv")
	want := "http://localhost:3100/mcp"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
