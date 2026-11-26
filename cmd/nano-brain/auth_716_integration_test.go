//go:build integration

package main

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/server/middleware"
	"github.com/rs/zerolog"
)

func TestCLIAuthEndToEnd716(t *testing.T) {
	const configToken = "test-config-token-716"
	const envToken = "test-env-token-716"
	e := echo.New()
	e.Use(middleware.Auth(middleware.AuthSnapshot{
		Enabled: true,
		Tokens:  []string{configToken, envToken},
	}, zerolog.Nop()))
	e.Any("/*", func(c echo.Context) error {
		switch c.Request().URL.Path {
		case "/api/status":
			return c.JSON(http.StatusOK, map[string]any{"version": "dev", "status": "ok"})
		case "/mcp":
			return c.JSON(http.StatusOK, map[string]any{"jsonrpc": "2.0", "id": 1, "result": map[string]any{}})
		case "/api/v1/workspaces":
			return c.JSON(http.StatusOK, map[string]any{"workspaces": []any{}})
		default:
			return c.JSON(http.StatusOK, map[string]any{"id": "test-id", "workspace_hash": "test-workspace"})
		}
	})
	server := httptest.NewServer(e)
	defer server.Close()
	unauthenticated, err := http.Get(server.URL + "/api/status")
	if err != nil {
		t.Fatal(err)
	}
	unauthenticated.Body.Close()
	if unauthenticated.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d", unauthenticated.StatusCode)
	}
	t.Logf("%s %s", unauthenticated.Proto, unauthenticated.Status)
	host, port, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	binary := filepath.Join(dir, "nano-brain")
	build := exec.Command("go", "build", "-buildvcs=false", "-o", binary, ".")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build CLI: %v\n%s", err, output)
	}
	configPath := filepath.Join(dir, "config.yml")
	config := "server:\n  auth:\n    enabled: true\n    tokens:\n      - " + configToken +
		"\ndatabase:\n  url: postgres://localhost:1/test\nembedding:\n  provider: \"\"\n"
	if err := os.WriteFile(configPath, []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	var baseEnv []string
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "NANO_BRAIN_") && !strings.HasPrefix(entry, "DATABASE_URL=") && !strings.HasPrefix(entry, "HOME=") {
			baseEnv = append(baseEnv, entry)
		}
	}
	baseEnv = append(baseEnv, "HOME="+dir, "NANO_BRAIN_CONFIG="+configPath,
		"NANO_BRAIN_HOST="+host, "NANO_BRAIN_PORT="+port, "NANO_BRAIN_NO_AUTO_START=1")

	mcpOverride := ""
	run := func(token string, args ...string) ([]byte, error) {
		t.Helper()
		cmd := exec.Command(binary, args...)
		cmd.Env = append(append([]string{}, baseEnv...), "NANO_BRAIN_TOKEN="+token, "NANO_BRAIN_MCP_URL="+mcpOverride)
		output, err := cmd.CombinedOutput()
		if strings.Contains(string(output), configToken) || strings.Contains(string(output), envToken) ||
			(token != "" && strings.Contains(string(output), token)) {
			t.Fatal("CLI output exposed a credential")
		}
		return output, err
	}

	for _, args := range [][]string{
		{"status", "--json"},
		{"workspaces", "list", "--json"},
		{"init", "--root", dir, "--json"},
		{"reindex", "--root", dir, "--workspace", "test-workspace", "--json"},
		{"write", "test content", "--workspace", "test-workspace", "--json"},
		{"collection", "list", "--workspace", "test-workspace"},
		{"collection", "add", "--name", "test", "--path", dir, "--workspace", "test-workspace"},
		{"collection", "rename", "--old", "test", "--new", "test2", "--workspace", "test-workspace"},
		{"collection", "remove", "--name", "test2", "--workspace", "test-workspace"},
	} {
		if output, err := run(envToken, args...); err != nil {
			t.Fatalf("%v: %v\n%s", args, err, output)
		}
	}
	if output, err := run("", "status", "--json"); err != nil {
		t.Fatalf("config token: %v\n%s", err, output)
	}
	if output, err := run("wrong-token", "status", "--json"); err == nil || !strings.Contains(string(output), "401") {
		t.Fatal("wrong environment token should override configured token and return HTTP 401")
	}

	output, _ := run(envToken, "doctor", "--online", "--json")
	var report struct {
		Checks []struct {
			Name   string `json:"name"`
			Status string `json:"status"`
		} `json:"checks"`
	}
	if err := json.Unmarshal(output, &report); err != nil {
		t.Fatalf("parse doctor output: %v\n%s", err, output)
	}
	online := map[string]bool{"Server running": false, "MCP reachable": false}
	for _, check := range report.Checks {
		if _, ok := online[check.Name]; ok {
			online[check.Name] = check.Status == "ok"
		}
	}
	if !online["Server running"] || !online["MCP reachable"] {
		t.Fatalf("online checks failed: %v", online)
	}

	received := make(chan string, 1)
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
	}))
	defer other.Close()
	mcpOverride = other.URL + "/mcp"
	_, _ = run(envToken, "doctor", "--online", "--json")
	select {
	case auth := <-received:
		if auth != "" {
			t.Fatal("server credential forwarded to a different MCP origin")
		}
	default:
		t.Fatal("MCP override was not probed")
	}
}
