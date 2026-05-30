package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseStubFlags_ScopeAll(t *testing.T) {
	f, errMsg := parseStubFlags([]string{"my query", "--scope", "all"})
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if f.scope != "all" {
		t.Errorf("scope = %q, want %q", f.scope, "all")
	}
	if f.query != "my query" {
		t.Errorf("query = %q, want %q", f.query, "my query")
	}
}

func TestParseStubFlags_ScopeAllEquals(t *testing.T) {
	f, errMsg := parseStubFlags([]string{"q", "--scope=all"})
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if f.scope != "all" {
		t.Errorf("scope = %q, want %q", f.scope, "all")
	}
}

func TestParseStubFlags_ScopeWorkspaceDefault(t *testing.T) {
	f, errMsg := parseStubFlags([]string{"q", "--workspace", "abc"})
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if f.scope != "workspace" {
		t.Errorf("scope = %q, want %q", f.scope, "workspace")
	}
	if f.workspace != "abc" {
		t.Errorf("workspace = %q, want %q", f.workspace, "abc")
	}
}

func TestParseStubFlags_ScopeWorkspaceExplicit(t *testing.T) {
	f, errMsg := parseStubFlags([]string{"q", "--scope", "workspace", "--workspace", "abc"})
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if f.scope != "workspace" {
		t.Errorf("scope = %q, want %q", f.scope, "workspace")
	}
}

func TestParseStubFlags_ScopeInvalid(t *testing.T) {
	_, errMsg := parseStubFlags([]string{"q", "--scope", "foo"})
	if errMsg == "" {
		t.Fatal("expected error for invalid scope, got none")
	}
}

func TestParseStubFlags_ScopeMissingValue(t *testing.T) {
	_, errMsg := parseStubFlags([]string{"q", "--scope"})
	if errMsg == "" {
		t.Fatal("expected error for missing --scope value")
	}
}

func TestParseStubFlags_ScopeAllOverridesWorkspace(t *testing.T) {
	f, errMsg := parseStubFlags([]string{"q", "--scope", "all", "--workspace", "abc"})
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if f.scope != "all" {
		t.Errorf("scope = %q, want %q", f.scope, "all")
	}
	if f.workspace != "abc" {
		t.Errorf("workspace = %q, want %q", f.workspace, "abc")
	}
}

func TestStubCmd_ScopeAll(t *testing.T) {
	var capturedBody map[string]string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer ts.Close()
	pointHTTPClientAt(t, ts)

	f, errMsg := parseStubFlags([]string{"test query", "--scope", "all"})
	if errMsg != "" {
		t.Fatalf("parse error: %s", errMsg)
	}

	workspaceVal := f.workspace
	if f.scope == "all" {
		workspaceVal = "all"
	}

	body, _ := json.Marshal(map[string]string{"query": f.query, "workspace": workspaceVal})
	_, _, err := doRequest("POST", ts.URL+"/api/v1/query", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	if capturedBody["workspace"] != "all" {
		t.Errorf("workspace in body = %q, want %q", capturedBody["workspace"], "all")
	}
	if capturedBody["query"] != "test query" {
		t.Errorf("query in body = %q, want %q", capturedBody["query"], "test query")
	}
}

func TestStubCmd_ScopeWorkspace(t *testing.T) {
	var capturedBody map[string]string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer ts.Close()
	pointHTTPClientAt(t, ts)

	f, errMsg := parseStubFlags([]string{"test query", "--scope", "workspace", "--workspace", "abc"})
	if errMsg != "" {
		t.Fatalf("parse error: %s", errMsg)
	}

	body, _ := json.Marshal(map[string]string{"query": f.query, "workspace": f.workspace})
	_, _, err := doRequest("POST", ts.URL+"/api/v1/query", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	if capturedBody["workspace"] != "abc" {
		t.Errorf("workspace in body = %q, want %q", capturedBody["workspace"], "abc")
	}
}

func TestStubCmd_ScopeAllOverridesWorkspace(t *testing.T) {
	var capturedBody map[string]string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer ts.Close()
	pointHTTPClientAt(t, ts)

	f, errMsg := parseStubFlags([]string{"test query", "--scope", "all", "--workspace", "abc"})
	if errMsg != "" {
		t.Fatalf("parse error: %s", errMsg)
	}

	workspaceVal := f.workspace
	if f.scope == "all" {
		workspaceVal = "all"
	}

	body, _ := json.Marshal(map[string]string{"query": f.query, "workspace": workspaceVal})
	_, _, err := doRequest("POST", ts.URL+"/api/v1/query", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	if capturedBody["workspace"] != "all" {
		t.Errorf("workspace in body = %q, want %q (--scope=all should override --workspace)", capturedBody["workspace"], "all")
	}
}
