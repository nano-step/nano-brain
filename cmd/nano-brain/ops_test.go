package main

import (
	"encoding/json"
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

func TestResolveHostPort_InvalidPort(t *testing.T) {
	t.Setenv("NANO_BRAIN_HOST", "")
	t.Setenv("NANO_BRAIN_PORT", "notanumber")
	_, port := resolveHostPort()
	if port != 3100 {
		t.Errorf("port = %d, want default %d for invalid input", port, 3100)
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
		json.NewEncoder(w).Encode(statusResp)
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
