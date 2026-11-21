package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveHostPort_Defaults(t *testing.T) {
	t.Setenv("NANO_BRAIN_HOST", "")
	t.Setenv("NANO_BRAIN_PORT", "")
	orig := isContainerFn
	isContainerFn = func() bool { return false }
	t.Cleanup(func() { isContainerFn = orig })
	host, port := resolveHostPort()
	if host != "localhost" {
		t.Errorf("host = %q, want %q", host, "localhost")
	}
	if port != 3100 {
		t.Errorf("port = %d, want %d", port, 3100)
	}
}

func TestResolveHostPort_EnvOverride(t *testing.T) {
	t.Setenv("NANO_BRAIN_HOST", "myhost")
	t.Setenv("NANO_BRAIN_PORT", "9090")
	host, port := resolveHostPort()
	if host != "myhost" {
		t.Errorf("host = %q, want %q", host, "myhost")
	}
	if port != 9090 {
		t.Errorf("port = %d, want %d", port, 9090)
	}
}

// A wildcard server.host is an address to listen on, not one to dial: the
// client must connect to localhost so its Host header is in the server's
// default allowlist. Explicit NANO_BRAIN_HOST values and non-wildcard hosts
// are used as given.
func TestResolveHostPort_WildcardConfigHostDialsLocalhost(t *testing.T) {
	tests := []struct {
		name       string
		configHost string
		envHost    string
		want       string
	}{
		{"ipv4 wildcard", "0.0.0.0", "", "localhost"},
		{"ipv6 wildcard", "::", "", "localhost"},
		{"bracketed ipv6 wildcard", "[::]", "", "localhost"},
		{"loopback name unchanged", "localhost", "", "localhost"},
		{"loopback address unchanged", "127.0.0.1", "", "127.0.0.1"},
		{"lan address unchanged", "192.168.1.50", "", "192.168.1.50"},
		{"hostname unchanged", "brain.example.com", "", "brain.example.com"},
		{"explicit env wildcard is not rewritten", "localhost", "0.0.0.0", "0.0.0.0"},
		{"explicit env host wins over wildcard config", "0.0.0.0", "myhost", "myhost"},
	}

	orig := isContainerFn
	isContainerFn = func() bool { return false }
	t.Cleanup(func() { isContainerFn = orig })

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfgPath := filepath.Join(t.TempDir(), "config.yml")
			if err := os.WriteFile(cfgPath, []byte(fmt.Sprintf("server:\n  host: %q\n  port: 3199\n", tt.configHost)), 0o600); err != nil {
				t.Fatal(err)
			}
			t.Setenv("NANO_BRAIN_CONFIG", cfgPath)
			// An empty NANO_BRAIN_SERVER_HOST still overrides the file, so
			// it has to be absent; t.Setenv first so it is restored after.
			t.Setenv("NANO_BRAIN_SERVER_HOST", "")
			os.Unsetenv("NANO_BRAIN_SERVER_HOST")
			t.Setenv("NANO_BRAIN_HOST", tt.envHost)
			t.Setenv("NANO_BRAIN_PORT", "")

			host, port := resolveHostPort()
			if host != tt.want {
				t.Errorf("server.host %q NANO_BRAIN_HOST %q: host = %q, want %q", tt.configHost, tt.envHost, host, tt.want)
			}
			if port != 3199 {
				t.Errorf("port = %d, want 3199 (config not loaded?)", port)
			}
			if got, want := getBaseURL(), fmt.Sprintf("http://%s:3199", tt.want); got != want {
				t.Errorf("getBaseURL() = %q, want %q", got, want)
			}
		})
	}
}

func TestResolveHostPort_InvalidPort(t *testing.T) {
	t.Setenv("NANO_BRAIN_HOST", "")
	t.Setenv("NANO_BRAIN_PORT", "notanumber")
	_, port := resolveHostPort()
	if port != 3100 {
		t.Errorf("port = %d, want default %d for invalid input", port, 3100)
	}
}

func TestResolveHostPort_OutOfRangePort(t *testing.T) {
	t.Setenv("NANO_BRAIN_HOST", "")
	t.Setenv("NANO_BRAIN_PORT", "99999")
	_, port := resolveHostPort()
	if port != 3100 {
		t.Errorf("port = %d, want default %d for out-of-range port", port, 3100)
	}
}

func TestTailLines_Basic(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")
	content := "line1\nline2\nline3\nline4\nline5\n"
	if err := os.WriteFile(logFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	lines, err := tailLines(logFile, 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 3 {
		t.Errorf("got %d lines, want 3", len(lines))
	}
	if lines[0] != "line3" || lines[1] != "line4" || lines[2] != "line5" {
		t.Errorf("got %v, want [line3 line4 line5]", lines)
	}
}

func TestTailLines_FewerThanRequested(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")
	content := "line1\nline2\n"
	if err := os.WriteFile(logFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	lines, err := tailLines(logFile, 50)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 2 {
		t.Errorf("got %d lines, want 2", len(lines))
	}
}

func TestTailLines_FileNotFound(t *testing.T) {
	_, err := tailLines("/nonexistent/file.log", 10)
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestTailLines_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "empty.log")
	if err := os.WriteFile(logFile, []byte{}, 0o644); err != nil {
		t.Fatal(err)
	}

	lines, err := tailLines(logFile, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 0 {
		t.Errorf("got %d lines, want 0 for empty file", len(lines))
	}
}

func TestTailLines_MultiChunk(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "big.log")

	var b strings.Builder
	totalLines := 2000
	for i := 1; i <= totalLines; i++ {
		fmt.Fprintf(&b, "line-%04d\n", i)
	}
	if err := os.WriteFile(logFile, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	lines, err := tailLines(logFile, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 5 {
		t.Fatalf("got %d lines, want 5", len(lines))
	}
	if lines[0] != "line-1996" {
		t.Errorf("lines[0] = %q, want %q", lines[0], "line-1996")
	}
	if lines[4] != "line-2000" {
		t.Errorf("lines[4] = %q, want %q", lines[4], "line-2000")
	}
}

func TestRunStatusCmd_JSON(t *testing.T) {
	statusResp := map[string]interface{}{
		"status":  "ok",
		"version": "test",
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/status" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(statusResp)
	}))
	defer ts.Close()

	parts := strings.Split(strings.TrimPrefix(ts.URL, "http://"), ":")
	t.Setenv("NANO_BRAIN_HOST", parts[0])
	if len(parts) > 1 {
		t.Setenv("NANO_BRAIN_PORT", parts[1])
	}

	resp, _, err := doRequest("GET", ts.URL+"/api/status", nil)
	if err != nil {
		t.Fatalf("doRequest failed: %v", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(resp, &result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}
	if result["status"] != "ok" {
		t.Errorf("status = %v, want ok", result["status"])
	}
}

func TestDefaultLogPath(t *testing.T) {
	p := defaultLogPath()
	if !strings.Contains(p, ".nano-brain") || !strings.HasSuffix(p, "nano-brain.log") {
		t.Errorf("defaultLogPath() = %q, want path containing .nano-brain/logs/nano-brain.log", p)
	}
}
