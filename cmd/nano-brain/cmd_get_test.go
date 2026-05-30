package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func sampleGetDocResponse() []byte {
	r := getDocResult{
		ID:            "11111111-1111-1111-1111-111111111111",
		Title:         "Decision: use PostgreSQL",
		Content:       "We chose PostgreSQL for pgvector.",
		SourcePath:    "memory://decision.md",
		Collection:    "memory",
		Tags:          []string{"decision", "architecture"},
		WorkspaceHash: "ws-abc",
		CreatedAt:     "2026-05-01T00:00:00Z",
		UpdatedAt:     "2026-05-30T10:00:00Z",
	}
	data, _ := json.Marshal(r)
	return data
}

func startGetTestServer(t *testing.T, statusCode int, respBody []byte) (*httptest.Server, *[]byte) {
	t.Helper()
	captured := &[]byte{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/get" {
			t.Errorf("expected path /api/v1/get, got %s", r.URL.Path)
		}
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r.Body)
		*captured = buf.Bytes()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		_, _ = w.Write(respBody)
	}))
	return ts, captured
}

func TestRunGetCmd_BySourcePath(t *testing.T) {
	ts, captured := startGetTestServer(t, http.StatusOK, sampleGetDocResponse())
	defer ts.Close()
	pointClientAt(t, ts)

	reqBody := map[string]interface{}{
		"workspace": "ws-abc",
		"path":      "memory://decision.md",
	}
	data, _ := json.Marshal(reqBody)
	resp, statusCode, err := doRequest("POST", ts.URL+"/api/v1/get", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("doRequest error: %v", err)
	}
	if statusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", statusCode)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(*captured, &body); err != nil {
		t.Fatalf("could not parse captured body: %v", err)
	}
	if body["workspace"] != "ws-abc" {
		t.Errorf("workspace = %v, want ws-abc", body["workspace"])
	}
	if body["path"] != "memory://decision.md" {
		t.Errorf("path = %v, want memory://decision.md", body["path"])
	}

	var result getDocResult
	if err := json.Unmarshal(resp, &result); err != nil {
		t.Fatalf("could not parse response: %v", err)
	}
	if result.ID != "11111111-1111-1111-1111-111111111111" {
		t.Errorf("id = %q, want 11111111-1111-1111-1111-111111111111", result.ID)
	}
	if result.SourcePath != "memory://decision.md" {
		t.Errorf("source_path = %q, want memory://decision.md", result.SourcePath)
	}
}

func TestRunGetCmd_ByID(t *testing.T) {
	ts, captured := startGetTestServer(t, http.StatusOK, sampleGetDocResponse())
	defer ts.Close()
	pointClientAt(t, ts)

	docID := "11111111-1111-1111-1111-111111111111"
	reqBody := map[string]interface{}{
		"workspace": "ws-abc",
		"id":        docID,
	}
	data, _ := json.Marshal(reqBody)
	_, statusCode, err := doRequest("POST", ts.URL+"/api/v1/get", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("doRequest error: %v", err)
	}
	if statusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", statusCode)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(*captured, &body); err != nil {
		t.Fatalf("could not parse captured body: %v", err)
	}
	if body["id"] != docID {
		t.Errorf("id = %v, want %s", body["id"], docID)
	}
}

func TestRunGetCmd_NotFound(t *testing.T) {
	errResp := []byte(`{"message":"document not found"}`)
	ts, _ := startGetTestServer(t, http.StatusNotFound, errResp)
	defer ts.Close()
	pointClientAt(t, ts)

	data, _ := json.Marshal(map[string]interface{}{"workspace": "ws-abc", "path": "missing"})
	_, statusCode, err := doRequest("POST", ts.URL+"/api/v1/get", bytes.NewReader(data))
	if err == nil {
		t.Fatal("expected error for 404 response, got nil")
	}
	if statusCode != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", statusCode)
	}
	if !strings.Contains(err.Error(), "server returned 404") {
		t.Errorf("error = %q, want it to contain 'server returned 404'", err.Error())
	}
}

func TestPrintGetResult(t *testing.T) {
	r := getDocResult{
		ID:         "uuid-1",
		Title:      "My Doc",
		Content:    "Some content here",
		SourcePath: "memory://doc.md",
		Collection: "memory",
		Tags:       []string{"tag1"},
		UpdatedAt:  "2026-05-30T10:00:00Z",
	}
	printGetResult(r)
}

func TestPrintGetResult_NoTitle(t *testing.T) {
	r := getDocResult{
		ID:         "uuid-1",
		Content:    "content",
		SourcePath: "memory://doc.md",
		Collection: "memory",
		Tags:       []string{},
		UpdatedAt:  "2026-05-30T10:00:00Z",
	}
	printGetResult(r)
}
