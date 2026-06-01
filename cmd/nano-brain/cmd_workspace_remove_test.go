package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newRemoveServer(t *testing.T, hash string, respPayload interface{}, status int) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/workspaces":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"workspaces": []map[string]interface{}{
					{"hash": hash, "doc_count": 5},
				},
			})
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/workspaces/"+hash:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(status)
			_ = json.NewEncoder(w).Encode(respPayload)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(ts.Close)
	return ts
}

func TestParseWorkspaceRemoveFlags_Basic(t *testing.T) {
	f, errMsg := parseWorkspaceRemoveFlags([]string{"--workspace=abc123", "--force"})
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if f.workspace != "abc123" {
		t.Errorf("workspace = %q, want abc123", f.workspace)
	}
	if !f.force {
		t.Error("expected force=true")
	}
}

func TestParseWorkspaceRemoveFlags_DryRun(t *testing.T) {
	f, errMsg := parseWorkspaceRemoveFlags([]string{"--workspace=abc123", "--dry-run"})
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if !f.dryRun {
		t.Error("expected dryRun=true")
	}
	if f.force {
		t.Error("expected force=false")
	}
}

func TestParseWorkspaceRemoveFlags_WorkspaceSplit(t *testing.T) {
	f, errMsg := parseWorkspaceRemoveFlags([]string{"--workspace", "abc123"})
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if f.workspace != "abc123" {
		t.Errorf("workspace = %q, want abc123", f.workspace)
	}
}

func TestParseWorkspaceRemoveFlags_UnknownFlag(t *testing.T) {
	_, errMsg := parseWorkspaceRemoveFlags([]string{"--workspace=abc", "--bad-flag"})
	if errMsg == "" {
		t.Fatal("expected error for unknown flag")
	}
}

func TestParseWorkspaceRemoveFlags_Help(t *testing.T) {
	_, errMsg := parseWorkspaceRemoveFlags([]string{"-h"})
	if errMsg != "help" {
		t.Errorf("expected errMsg=help, got %q", errMsg)
	}
}

func TestWorkspaceRemoveCLI_MissingWorkspace(t *testing.T) {
	var out, errOut bytes.Buffer
	code := runWorkspacesRemoveWithIO(nil, &out, &errOut)
	if code == 0 {
		t.Fatal("expected non-zero exit for missing workspace")
	}
	if !strings.Contains(errOut.String(), "--workspace") {
		t.Errorf("expected usage hint in stderr; got %q", errOut.String())
	}
}

func TestWorkspaceRemoveCLI_RefusesWithoutForce(t *testing.T) {
	ts := newRemoveServer(t, "abc123", nil, http.StatusOK)
	setHostPort(t, ts)

	var out, errOut bytes.Buffer
	code := runWorkspacesRemoveWithIO([]string{"--workspace=abc123"}, &out, &errOut)
	if code == 0 {
		t.Fatal("expected non-zero exit without --force")
	}
	if !strings.Contains(errOut.String(), "--force") {
		t.Errorf("expected --force hint in stderr; got %q", errOut.String())
	}
}

func TestWorkspaceRemoveCLI_DryRun(t *testing.T) {
	ts := newRemoveServer(t, "abc123", nil, http.StatusOK)
	setHostPort(t, ts)

	var out, errOut bytes.Buffer
	code := runWorkspacesRemoveWithIO([]string{"--workspace=abc123", "--dry-run"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit=%d, want 0; stderr=%q", code, errOut.String())
	}
	if !strings.Contains(out.String(), "Dry run") {
		t.Errorf("expected 'Dry run' in stdout; got %q", out.String())
	}
	if !strings.Contains(out.String(), "No changes written") {
		t.Errorf("expected 'No changes written' in stdout; got %q", out.String())
	}
}

func TestWorkspaceRemoveCLI_ForceSuccess(t *testing.T) {
	payload := map[string]interface{}{
		"workspace":         "abc123",
		"deleted_docs":      float64(5),
		"workspace_removed": true,
	}
	ts := newRemoveServer(t, "abc123", payload, http.StatusOK)
	setHostPort(t, ts)

	var out, errOut bytes.Buffer
	code := runWorkspacesRemoveWithIO([]string{"--workspace=abc123", "--force"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit=%d, want 0; stderr=%q stderr=%q", code, errOut.String(), errOut.String())
	}
	got := out.String()
	if !strings.Contains(got, "abc123") {
		t.Errorf("expected workspace hash in output; got %q", got)
	}
	if !strings.Contains(got, "removed") {
		t.Errorf("expected 'removed' in output; got %q", got)
	}
}

func TestWorkspaceRemoveCLI_ForceJSON(t *testing.T) {
	payload := map[string]interface{}{
		"workspace":         "abc123",
		"deleted_docs":      float64(5),
		"workspace_removed": true,
	}
	ts := newRemoveServer(t, "abc123", payload, http.StatusOK)
	setHostPort(t, ts)

	var out, errOut bytes.Buffer
	code := runWorkspacesRemoveWithIO([]string{"--workspace=abc123", "--force", "--json"}, &out, &errOut)
	if code != 0 {
		t.Fatalf("exit=%d, want 0; stderr=%q", code, errOut.String())
	}
	var got map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimRight(out.String(), "\n")), &got); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, out.String())
	}
	if got["workspace"] != "abc123" {
		t.Errorf("workspace = %v, want abc123", got["workspace"])
	}
}

func TestWorkspaceRemoveCLI_NotFound(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/workspaces":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{})
		case r.Method == http.MethodDelete:
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"message":"workspace not found"}`))
		}
	}))
	t.Cleanup(ts.Close)
	setHostPort(t, ts)

	var out, errOut bytes.Buffer
	code := runWorkspacesRemoveWithIO([]string{"--workspace=nonexistent", "--force"}, &out, &errOut)
	if code == 0 {
		t.Fatal("expected non-zero exit for 404")
	}
}
