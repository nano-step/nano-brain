package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func sampleWakeUpResponse() []byte {
	resp := wakeUpResponse{
		Summary: "Workspace has 42 documents across 3 collections. Last activity: 5m ago.",
		RecentMemories: []wakeUpMemory{
			{
				ID:      "uuid-1",
				Title:   "Decision: use PostgreSQL",
				Snippet: "We decided to use PostgreSQL for its pgvector support.",
				Tags:    []string{"decision", "architecture"},
				Date:    "2026-05-30T10:00:00Z",
			},
		},
		ActiveCollections: []wakeUpCollection{
			{Name: "memory", DocumentCount: 10, LastUpdated: "2026-05-30T09:00:00Z"},
			{Name: "codebase", DocumentCount: 32, LastUpdated: "2026-05-29T15:00:00Z"},
		},
		Stats: wakeUpStats{
			TotalDocuments: 42,
			TotalChunks:    180,
			LastActivity:   "2026-05-30T10:00:00Z",
		},
	}
	data, _ := json.Marshal(resp)
	return data
}

func startWakeUpTestServer(t *testing.T, statusCode int, respBody []byte) (*httptest.Server, *[]byte) {
	t.Helper()
	captured := &[]byte{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/wake-up" {
			t.Errorf("expected path /api/v1/wake-up, got %s", r.URL.Path)
		}
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r.Body)
		body := buf.Bytes()
		*captured = body
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		_, _ = w.Write(respBody)
	}))
	return ts, captured
}

func pointClientAt(t *testing.T, ts *httptest.Server) {
	t.Helper()
	addr := strings.TrimPrefix(ts.URL, "http://")
	parts := strings.SplitN(addr, ":", 2)
	t.Setenv("NANO_BRAIN_HOST", parts[0])
	if len(parts) == 2 {
		t.Setenv("NANO_BRAIN_PORT", parts[1])
	}
}

func TestRunWakeUpCmd_RequestBodyContainsWorkspace(t *testing.T) {
	ts, captured := startWakeUpTestServer(t, http.StatusOK, sampleWakeUpResponse())
	defer ts.Close()
	pointClientAt(t, ts)

	reqBody := map[string]interface{}{"workspace": "abc123", "limit": 5}
	data, _ := json.Marshal(reqBody)
	resp, statusCode, err := doRequest("POST", ts.URL+"/api/v1/wake-up", bytes.NewReader(data))
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
	if body["workspace"] != "abc123" {
		t.Errorf("workspace = %v, want abc123", body["workspace"])
	}
	if body["limit"] != float64(5) {
		t.Errorf("limit = %v, want 5", body["limit"])
	}

	var result wakeUpResponse
	if err := json.Unmarshal(resp, &result); err != nil {
		t.Fatalf("could not parse response: %v", err)
	}
	if result.Stats.TotalDocuments != 42 {
		t.Errorf("total_documents = %d, want 42", result.Stats.TotalDocuments)
	}
	if len(result.RecentMemories) != 1 {
		t.Errorf("recent_memories len = %d, want 1", len(result.RecentMemories))
	}
}

func TestRunWakeUpCmd_DefaultLimitOmitted(t *testing.T) {
	ts, captured := startWakeUpTestServer(t, http.StatusOK, sampleWakeUpResponse())
	defer ts.Close()
	pointClientAt(t, ts)

	reqBody := map[string]interface{}{"workspace": "ws-hash"}
	data, _ := json.Marshal(reqBody)
	_, _, err := doRequest("POST", ts.URL+"/api/v1/wake-up", bytes.NewReader(data))
	if err != nil {
		t.Fatalf("doRequest error: %v", err)
	}

	var body map[string]interface{}
	if err := json.Unmarshal(*captured, &body); err != nil {
		t.Fatalf("could not parse captured body: %v", err)
	}
	if _, hasLimit := body["limit"]; hasLimit {
		t.Errorf("expected no 'limit' key when using default, got body: %v", body)
	}
}

func TestRunWakeUpCmd_ServerError(t *testing.T) {
	errResp := []byte(`{"message":"workspace not found"}`)
	ts, _ := startWakeUpTestServer(t, http.StatusBadRequest, errResp)
	defer ts.Close()
	pointClientAt(t, ts)

	data, _ := json.Marshal(map[string]interface{}{"workspace": "invalid"})
	_, statusCode, err := doRequest("POST", ts.URL+"/api/v1/wake-up", bytes.NewReader(data))
	if err == nil {
		t.Fatal("expected error for 400 response, got nil")
	}
	if statusCode != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", statusCode)
	}
	if !strings.Contains(err.Error(), "server returned 400") {
		t.Errorf("error = %q, want it to contain 'server returned 400'", err.Error())
	}
}

func TestPrintWakeUpResponse_NoCollections(t *testing.T) {
	r := wakeUpResponse{
		Summary:           "No data yet.",
		RecentMemories:    []wakeUpMemory{},
		ActiveCollections: []wakeUpCollection{},
		Stats:             wakeUpStats{TotalDocuments: 0, TotalChunks: 0},
	}
	printWakeUpResponse(r)
}

func TestPrintWakeUpResponse_SnippetTruncation(t *testing.T) {
	longSnippet := strings.Repeat("x", 200)
	r := wakeUpResponse{
		Summary: "ok",
		RecentMemories: []wakeUpMemory{
			{Title: "test", Snippet: longSnippet, Tags: []string{}, Date: "2026-01-01T00:00:00Z"},
		},
		ActiveCollections: []wakeUpCollection{},
		Stats:             wakeUpStats{},
	}
	printWakeUpResponse(r)
}
