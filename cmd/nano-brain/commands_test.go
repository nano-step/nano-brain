package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestGetBaseURL_Defaults(t *testing.T) {
	t.Setenv("NANO_BRAIN_HOST", "")
	t.Setenv("NANO_BRAIN_PORT", "")
	got := getBaseURL()
	if got != "http://localhost:3100" {
		t.Errorf("getBaseURL() = %q, want %q", got, "http://localhost:3100")
	}
}

func TestGetBaseURL_EnvOverride(t *testing.T) {
	t.Setenv("NANO_BRAIN_HOST", "myhost")
	t.Setenv("NANO_BRAIN_PORT", "9090")
	got := getBaseURL()
	if got != "http://myhost:9090" {
		t.Errorf("getBaseURL() = %q, want %q", got, "http://myhost:9090")
	}
}

func TestGetBaseURL_PartialOverride(t *testing.T) {
	t.Setenv("NANO_BRAIN_HOST", "custom")
	t.Setenv("NANO_BRAIN_PORT", "")
	got := getBaseURL()
	if got != "http://custom:3100" {
		t.Errorf("getBaseURL() = %q, want %q", got, "http://custom:3100")
	}
}

func TestDoRequest_Success(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer ts.Close()

	data, err := doRequest("GET", ts.URL+"/test", nil)
	if err != nil {
		t.Fatalf("doRequest() error = %v", err)
	}
	if !strings.Contains(string(data), "ok") {
		t.Errorf("expected response to contain 'ok', got %q", string(data))
	}
}

func TestDoRequest_ServerError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"fail"}`))
	}))
	defer ts.Close()

	data, err := doRequest("GET", ts.URL+"/test", nil)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if !strings.Contains(err.Error(), "server returned 500") {
		t.Errorf("error = %q, want it to contain 'server returned 500'", err.Error())
	}
	if data == nil {
		t.Error("expected body data even on error")
	}
}

func TestDoRequest_ConnectionRefused(t *testing.T) {
	t.Setenv("NANO_BRAIN_HOST", "localhost")
	t.Setenv("NANO_BRAIN_PORT", "19999")
	_, err := doRequest("GET", "http://localhost:19999/test", nil)
	if err == nil {
		t.Fatal("expected error for connection refused")
	}
	if !strings.Contains(err.Error(), "cannot connect to nano-brain server") &&
		!strings.Contains(err.Error(), "request failed") {
		t.Errorf("error = %q, expected connection error message", err.Error())
	}
}

func TestDoRequest_NotFound_ReturnsBody(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"not_found"}`))
	}))
	defer ts.Close()

	data, err := doRequest("POST", ts.URL+"/api/v1/query", strings.NewReader(`{}`))
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if !strings.Contains(err.Error(), "server returned 404") {
		t.Errorf("error = %q, want 'server returned 404'", err.Error())
	}
	if !strings.Contains(string(data), "not_found") {
		t.Errorf("body = %q, want 'not_found'", string(data))
	}
}

func TestDoRequest_SetsContentType(t *testing.T) {
	var gotCT string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	doRequest("POST", ts.URL+"/test", strings.NewReader(`{"foo":"bar"}`))
	if gotCT != "application/json" {
		t.Errorf("Content-Type = %q, want %q", gotCT, "application/json")
	}
}

func TestDoRequest_NoContentTypeWithoutBody(t *testing.T) {
	var gotCT string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	doRequest("GET", ts.URL+"/test", nil)
	if gotCT != "" {
		t.Errorf("Content-Type = %q, want empty for nil body", gotCT)
	}
}

func TestGetBaseURL_EnvVarsFromOS(t *testing.T) {
	os.Setenv("NANO_BRAIN_HOST", "check")
	defer os.Unsetenv("NANO_BRAIN_HOST")
	os.Setenv("NANO_BRAIN_PORT", "4444")
	defer os.Unsetenv("NANO_BRAIN_PORT")

	got := getBaseURL()
	if got != "http://check:4444" {
		t.Errorf("getBaseURL() = %q, want %q", got, "http://check:4444")
	}
}
