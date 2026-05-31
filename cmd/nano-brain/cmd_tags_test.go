package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func sampleTagsResponse() []byte {
	items := []tagItem{
		{Tag: "decision", Count: 5},
		{Tag: "architecture", Count: 3},
		{Tag: "bug", Count: 1},
	}
	data, _ := json.Marshal(items)
	return data
}

func startTagsTestServer(t *testing.T, statusCode int, respBody []byte) (*httptest.Server, *string) {
	t.Helper()
	capturedWS := new(string)
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/tags" {
			t.Errorf("expected path /api/v1/tags, got %s", r.URL.Path)
		}
		*capturedWS = r.URL.Query().Get("workspace")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		_, _ = w.Write(respBody)
	}))
	return ts, capturedWS
}

func TestRunTagsCmd_WorkspacePassedAsQueryParam(t *testing.T) {
	ts, capturedWS := startTagsTestServer(t, http.StatusOK, sampleTagsResponse())
	defer ts.Close()
	pointClientAt(t, ts)

	resp, statusCode, err := doRequest("GET", ts.URL+"/api/v1/tags?workspace=ws-abc", nil)
	if err != nil {
		t.Fatalf("doRequest error: %v", err)
	}
	if statusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", statusCode)
	}
	if *capturedWS != "ws-abc" {
		t.Errorf("workspace query param = %q, want ws-abc", *capturedWS)
	}

	var items []tagItem
	if err := json.Unmarshal(resp, &items); err != nil {
		t.Fatalf("could not parse response: %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("expected 3 tags, got %d", len(items))
	}
	if items[0].Tag != "decision" || items[0].Count != 5 {
		t.Errorf("unexpected first tag: %+v", items[0])
	}
}

func TestRunTagsCmd_EmptyResponse(t *testing.T) {
	ts, _ := startTagsTestServer(t, http.StatusOK, []byte("[]"))
	defer ts.Close()
	pointClientAt(t, ts)

	resp, statusCode, err := doRequest("GET", ts.URL+"/api/v1/tags?workspace=ws-empty", nil)
	if err != nil {
		t.Fatalf("doRequest error: %v", err)
	}
	if statusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", statusCode)
	}

	var items []tagItem
	if err := json.Unmarshal(resp, &items); err != nil {
		t.Fatalf("could not parse response: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 tags, got %d", len(items))
	}
}

func TestRunTagsCmd_ServerError(t *testing.T) {
	errResp := []byte(`{"message":"workspace not found"}`)
	ts, _ := startTagsTestServer(t, http.StatusBadRequest, errResp)
	defer ts.Close()
	pointClientAt(t, ts)

	_, statusCode, err := doRequest("GET", ts.URL+"/api/v1/tags?workspace=bad", nil)
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
