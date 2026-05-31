package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func sampleMultiGetResponse() []byte {
	r := multiGetResult{
		Results: []getDocResult{
			{
				ID:         "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
				Title:      "Doc A",
				Content:    "Content A",
				SourcePath: "memory://a.md",
				Collection: "memory",
				Tags:       []string{"tag1"},
				UpdatedAt:  "2026-05-30T10:00:00Z",
			},
			{
				ID:         "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
				Title:      "Doc B",
				Content:    "Content B",
				SourcePath: "memory://b.md",
				Collection: "memory",
				Tags:       []string{},
				UpdatedAt:  "2026-05-29T10:00:00Z",
			},
		},
		NotFound: []string{"memory://missing.md"},
	}
	data, _ := json.Marshal(r)
	return data
}

func startMultiGetTestServer(t *testing.T, statusCode int, respBody []byte) (*httptest.Server, *[]byte) {
	t.Helper()
	captured := &[]byte{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/multi-get" {
			t.Errorf("expected path /api/v1/multi-get, got %s", r.URL.Path)
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

func TestRunMultiGetCmd_PathsPassedInBody(t *testing.T) {
	ts, captured := startMultiGetTestServer(t, http.StatusOK, sampleMultiGetResponse())
	defer ts.Close()
	pointClientAt(t, ts)

	reqBody := map[string]interface{}{
		"workspace": "ws-abc",
		"paths":     []string{"memory://a.md", "memory://b.md", "memory://missing.md"},
	}
	data, _ := json.Marshal(reqBody)
	resp, statusCode, err := doRequest("POST", ts.URL+"/api/v1/multi-get", bytes.NewReader(data))
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
	paths, ok := body["paths"].([]interface{})
	if !ok {
		t.Fatalf("paths not a slice: %T", body["paths"])
	}
	if len(paths) != 3 {
		t.Errorf("paths len = %d, want 3", len(paths))
	}

	var result multiGetResult
	if err := json.Unmarshal(resp, &result); err != nil {
		t.Fatalf("could not parse response: %v", err)
	}
	if len(result.Results) != 2 {
		t.Errorf("results len = %d, want 2", len(result.Results))
	}
	if len(result.NotFound) != 1 {
		t.Errorf("not_found len = %d, want 1", len(result.NotFound))
	}
	if result.NotFound[0] != "memory://missing.md" {
		t.Errorf("not_found[0] = %q, want memory://missing.md", result.NotFound[0])
	}
}

func TestRunMultiGetCmd_IDsPassedInBody(t *testing.T) {
	ts, captured := startMultiGetTestServer(t, http.StatusOK, sampleMultiGetResponse())
	defer ts.Close()
	pointClientAt(t, ts)

	ids := []string{"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"}
	reqBody := map[string]interface{}{
		"workspace": "ws-abc",
		"ids":       ids,
	}
	data, _ := json.Marshal(reqBody)
	_, statusCode, err := doRequest("POST", ts.URL+"/api/v1/multi-get", bytes.NewReader(data))
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
	bodyIDs, ok := body["ids"].([]interface{})
	if !ok {
		t.Fatalf("ids not a slice: %T", body["ids"])
	}
	if len(bodyIDs) != 2 {
		t.Errorf("ids len = %d, want 2", len(bodyIDs))
	}
}

func TestRunMultiGetCmd_ServerError(t *testing.T) {
	errResp := []byte(`{"message":"workspace not found"}`)
	ts, _ := startMultiGetTestServer(t, http.StatusBadRequest, errResp)
	defer ts.Close()
	pointClientAt(t, ts)

	data, _ := json.Marshal(map[string]interface{}{"workspace": "bad", "paths": []string{"x"}})
	_, statusCode, err := doRequest("POST", ts.URL+"/api/v1/multi-get", bytes.NewReader(data))
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

func TestSplitCSV(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"a,b,c", []string{"a", "b", "c"}},
		{" a , b ", []string{"a", "b"}},
		{"single", []string{"single"}},
		{"a,,b", []string{"a", "b"}},
		{"", []string{}},
	}
	for _, tt := range tests {
		got := splitCSV(tt.input)
		if len(got) != len(tt.expected) {
			t.Errorf("splitCSV(%q) len = %d, want %d", tt.input, len(got), len(tt.expected))
			continue
		}
		for i, v := range got {
			if v != tt.expected[i] {
				t.Errorf("splitCSV(%q)[%d] = %q, want %q", tt.input, i, v, tt.expected[i])
			}
		}
	}
}

func TestPrintMultiGetResult(t *testing.T) {
	r := multiGetResult{
		Results: []getDocResult{
			{ID: "a", Content: "c1", SourcePath: "p1", Collection: "memory", Tags: []string{}, UpdatedAt: "2026-05-30"},
			{ID: "b", Content: "c2", SourcePath: "p2", Collection: "memory", Tags: []string{}, UpdatedAt: "2026-05-29"},
		},
		NotFound: []string{"p3"},
	}
	printMultiGetResult(r)
}
