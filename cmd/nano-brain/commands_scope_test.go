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

func TestParseStubFlags_TagsSpaceForm(t *testing.T) {
	f, errMsg := parseStubFlags([]string{"q", "--workspace", "ws1", "--tags", "decision,auth"})
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if len(f.tags) != 2 || f.tags[0] != "decision" || f.tags[1] != "auth" {
		t.Errorf("tags = %v, want [decision auth]", f.tags)
	}
}

func TestParseStubFlags_TagsEqualsForm(t *testing.T) {
	f, errMsg := parseStubFlags([]string{"q", "--workspace", "ws1", "--tags=bug,fix"})
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if len(f.tags) != 2 || f.tags[0] != "bug" || f.tags[1] != "fix" {
		t.Errorf("tags = %v, want [bug fix]", f.tags)
	}
}

func TestParseStubFlags_TagsSingleTag(t *testing.T) {
	f, errMsg := parseStubFlags([]string{"q", "--workspace", "ws1", "--tags=decision"})
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if len(f.tags) != 1 || f.tags[0] != "decision" {
		t.Errorf("tags = %v, want [decision]", f.tags)
	}
}

func TestParseStubFlags_TagsMissingValue(t *testing.T) {
	_, errMsg := parseStubFlags([]string{"q", "--tags"})
	if errMsg == "" {
		t.Fatal("expected error for missing --tags value")
	}
}

func TestParseStubFlags_NoTags(t *testing.T) {
	f, errMsg := parseStubFlags([]string{"q", "--workspace", "ws1"})
	if errMsg != "" {
		t.Fatalf("unexpected error: %s", errMsg)
	}
	if f.tags != nil {
		t.Errorf("expected nil tags when not provided, got %v", f.tags)
	}
}

func TestStubCmd_TagsSentInBody(t *testing.T) {
	var capturedBody map[string]interface{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewDecoder(r.Body).Decode(&capturedBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer ts.Close()
	pointHTTPClientAt(t, ts)

	f, errMsg := parseStubFlags([]string{"test query", "--workspace", "ws1", "--tags=decision,auth"})
	if errMsg != "" {
		t.Fatalf("parse error: %s", errMsg)
	}

	bodyMap := map[string]interface{}{"query": f.query, "workspace": f.workspace}
	if len(f.tags) > 0 {
		bodyMap["tags"] = f.tags
	}
	body, _ := json.Marshal(bodyMap)
	_, _, err := doRequest("POST", ts.URL+"/api/v1/query", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}

	tagsRaw, ok := capturedBody["tags"]
	if !ok {
		t.Fatal("expected 'tags' key in request body")
	}
	tagsSlice, ok := tagsRaw.([]interface{})
	if !ok {
		t.Fatalf("expected tags to be array, got %T", tagsRaw)
	}
	if len(tagsSlice) != 2 {
		t.Errorf("expected 2 tags, got %d", len(tagsSlice))
	}
	if tagsSlice[0] != "decision" || tagsSlice[1] != "auth" {
		t.Errorf("expected [decision auth], got %v", tagsSlice)
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
