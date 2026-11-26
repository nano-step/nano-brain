package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestSendRequestBearerToken(t *testing.T) {
	const token = "test-token-716"
	t.Setenv("NANO_BRAIN_TOKEN", token)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	_, status, err := sendRequest(http.MethodGet, server.URL, nil)
	if err != nil || status != http.StatusNoContent {
		t.Fatalf("authenticated request: status=%d, err=%v", status, err)
	}
}

func TestClientTokenPrecedence(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte("server:\n  auth:\n    enabled: true\n    tokens:\n      - config-token\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("NANO_BRAIN_CONFIG", path)
	t.Setenv("NANO_BRAIN_TOKEN", "")
	if got := clientToken(); got != "config-token" {
		t.Fatal("expected configured token")
	}
	t.Setenv("NANO_BRAIN_TOKEN", "env-token")
	if got := clientToken(); got != "env-token" {
		t.Fatal("expected environment token to take precedence")
	}
}

func TestSendRequestRejectsRemoteHTTPToken(t *testing.T) {
	t.Setenv("NANO_BRAIN_TOKEN", "test-token-716")
	t.Setenv("NANO_BRAIN_ALLOW_INSECURE_REMOTE_TOKEN", "")
	_, _, err := sendRequest(http.MethodGet, "http://example.test/", nil)
	if err == nil || !strings.Contains(err.Error(), "refusing to send credentials") {
		t.Fatalf("expected remote HTTP refusal, got %v", err)
	}
}

func TestSendRequestRedirectDoesNotLeakToken(t *testing.T) {
	const token = "test-token-716"
	t.Setenv("NANO_BRAIN_TOKEN", token)
	var reached atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached.Store(true)
	}))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer source.Close()

	_, _, err := sendRequest(http.MethodGet, source.URL, nil)
	if err == nil {
		t.Fatal("expected cross-origin redirect to be rejected")
	}
	if reached.Load() {
		t.Fatal("redirect reached another origin")
	}
}
