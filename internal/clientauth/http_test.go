package clientauth

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestRemoteHTTPRequiresExplicitOptIn(t *testing.T) {
	for _, tc := range []struct {
		name, url, optIn string
		allowed          bool
	}{
		{"remote HTTP", "http://example.test/", "", false},
		{"remote HTTP opt-in", "http://example.test/", "1", true},
		{"remote HTTPS", "https://example.test/", "", true},
		{"loopback", "http://127.0.0.1/", "", true},
		{"docker host", "http://host.docker.internal/", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("NANO_BRAIN_ALLOW_INSECURE_REMOTE_TOKEN", tc.optIn)
			called := false
			client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				called = true
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
			})}
			req, err := http.NewRequest(http.MethodGet, tc.url, nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Authorization", "Bearer test-token")
			resp, err := Do(client, req)
			if resp != nil {
				resp.Body.Close()
			}
			if called != tc.allowed || (err == nil) != tc.allowed {
				t.Fatalf("called=%v, err=%v, allowed=%v", called, err, tc.allowed)
			}
		})
	}
}

func TestSameOrigin(t *testing.T) {
	for _, tc := range []struct {
		a, b string
		same bool
	}{
		{"http://localhost:80/api", "http://LOCALHOST/mcp", true},
		{"http://localhost:3100", "http://localhost:3101", false},
		{"http://localhost", "https://localhost", false},
		{"http://localhost", "http://example.test", false},
	} {
		if got := SameOrigin(tc.a, tc.b); got != tc.same {
			t.Errorf("SameOrigin(%q, %q) = %v", tc.a, tc.b, got)
		}
	}
}

func TestAuthenticatedRedirectDoesNotLeakToOtherPort(t *testing.T) {
	var received atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Store(true)
	}))
	defer target.Close()
	source := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer source.Close()

	req, err := http.NewRequest(http.MethodGet, source.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer test-token")
	if _, err := Do(http.DefaultClient, req); err == nil {
		t.Fatal("expected cross-origin redirect to be rejected")
	}
	if received.Load() {
		t.Fatal("redirect reached another origin")
	}
}

func TestAuthenticatedRedirectSameOrigin(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.Redirect(w, r, "/next", http.StatusFound)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
		}
	}))
	defer server.Close()
	req, err := http.NewRequest(http.MethodGet, server.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer test-token")
	resp, err := Do(http.DefaultClient, req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
}
