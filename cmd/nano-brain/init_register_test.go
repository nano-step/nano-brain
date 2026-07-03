package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRegisterWorkspace_BuildsCorrectBodyAndParsesResult(t *testing.T) {
	var capturedBody map[string]interface{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/init" {
			w.WriteHeader(http.StatusOK)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"workspace_hash": "hash123",
			"root_path":      "/test/path",
			"name":           "test-workspace",
			"agents_snippet": "snippet-body",
		})
	}))
	defer ts.Close()
	pointHTTPClientAt(t, ts)

	origIsTTYFn := isTTYFn
	isTTYFn = func() bool { return false }
	t.Cleanup(func() { isTTYFn = origIsTTYFn })

	root := t.TempDir()
	result, err := registerWorkspace(root, "", false)
	if err != nil {
		t.Fatalf("registerWorkspace() error = %v", err)
	}

	if capturedBody["root_path"] != root {
		t.Errorf("captured body root_path = %v, want %v", capturedBody["root_path"], root)
	}
	if _, ok := capturedBody["workspace"]; ok {
		t.Errorf("captured body should not contain workspace key when workspace arg is empty, got %v", capturedBody)
	}
	if len(capturedBody) != 1 {
		t.Errorf("captured body = %v, want only root_path key", capturedBody)
	}

	if result.Name != "test-workspace" {
		t.Errorf("result.Name = %q, want %q", result.Name, "test-workspace")
	}
	if result.WorkspaceHash != "hash123" {
		t.Errorf("result.WorkspaceHash = %q, want %q", result.WorkspaceHash, "hash123")
	}
	if result.RootPath != "/test/path" {
		t.Errorf("result.RootPath = %q, want %q", result.RootPath, "/test/path")
	}
	if result.AgentsSnippet != "snippet-body" {
		t.Errorf("result.AgentsSnippet = %q, want %q", result.AgentsSnippet, "snippet-body")
	}
}

func TestRegisterWorkspace_IncludesWorkspaceWhenProvided(t *testing.T) {
	var capturedBody map[string]interface{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/init" {
			w.WriteHeader(http.StatusOK)
			return
		}
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"workspace_hash": "hash456",
			"root_path":      "/test/path2",
			"name":           "",
			"agents_snippet": "",
		})
	}))
	defer ts.Close()
	pointHTTPClientAt(t, ts)

	origIsTTYFn := isTTYFn
	isTTYFn = func() bool { return false }
	t.Cleanup(func() { isTTYFn = origIsTTYFn })

	root := t.TempDir()
	result, err := registerWorkspace(root, "myworkspace", false)
	if err != nil {
		t.Fatalf("registerWorkspace() error = %v", err)
	}

	if capturedBody["workspace"] != "myworkspace" {
		t.Errorf("captured body workspace = %v, want %q", capturedBody["workspace"], "myworkspace")
	}
	if result.WorkspaceHash != "hash456" {
		t.Errorf("result.WorkspaceHash = %q, want %q", result.WorkspaceHash, "hash456")
	}
}

func TestRegisterWorkspace_JSONFlagShortCircuits(t *testing.T) {
	called := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"workspace_hash": "hash789",
			"root_path":      "/test/path3",
			"name":           "should-not-be-used",
			"agents_snippet": "",
		})
	}))
	defer ts.Close()
	pointHTTPClientAt(t, ts)

	origIsTTYFn := isTTYFn
	isTTYFn = func() bool { t.Fatal("isTTYFn should not be consulted when jsonFlag is true"); return true }
	t.Cleanup(func() { isTTYFn = origIsTTYFn })

	root := t.TempDir()
	result, err := registerWorkspace(root, "", true)
	if err != nil {
		t.Fatalf("registerWorkspace() error = %v", err)
	}
	if !called {
		t.Fatal("expected /api/v1/init to be called")
	}
	if result != (initResult{}) {
		t.Errorf("result = %+v, want zero value when jsonFlag is true", result)
	}
}

func TestRegisterWorkspace_EmptyNameSkipsMCPPrompt(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"workspace_hash": "hashABC",
			"root_path":      "/test/path4",
			"name":           "",
			"agents_snippet": "",
		})
	}))
	defer ts.Close()
	pointHTTPClientAt(t, ts)

	origIsTTYFn := isTTYFn
	isTTYFn = func() bool { return true }
	t.Cleanup(func() { isTTYFn = origIsTTYFn })

	root := t.TempDir()
	result, err := registerWorkspace(root, "", false)
	if err != nil {
		t.Fatalf("registerWorkspace() error = %v", err)
	}
	if result.WorkspaceHash != "hashABC" {
		t.Errorf("result.WorkspaceHash = %q, want %q", result.WorkspaceHash, "hashABC")
	}
}
