package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestProbeServiceHealth(t *testing.T) {
	t.Run("ready", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok","ready":true,"version":"v2026.1.1"}`))
		}))
		defer srv.Close()
		host, port := splitHostPort(t, srv.URL)
		reachable, ready, version, probeErr := probeServiceHealth(host, port)
		if !reachable || !ready || version != "v2026.1.1" || probeErr != "" {
			t.Errorf("probe = (%v, %v, %q, %q), want (true, true, v2026.1.1, \"\")", reachable, ready, version, probeErr)
		}
	})

	t.Run("degraded still reachable", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"degraded","ready":false,"reason":"database unreachable"}`))
		}))
		defer srv.Close()
		host, port := splitHostPort(t, srv.URL)
		reachable, ready, _, _ := probeServiceHealth(host, port)
		if !reachable || ready {
			t.Errorf("probe = (%v, %v), want (true, false)", reachable, ready)
		}
	})

	t.Run("unauthorized reachable not ready", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		}))
		defer srv.Close()
		host, port := splitHostPort(t, srv.URL)
		reachable, ready, _, probeErr := probeServiceHealth(host, port)
		if !reachable || ready {
			t.Errorf("probe = (%v, %v), want (true, false)", reachable, ready)
		}
		if !strings.Contains(probeErr, "authentication") {
			t.Errorf("probeErr = %q, want authentication hint", probeErr)
		}
	})

	t.Run("unreachable", func(t *testing.T) {
		reachable, ready, _, probeErr := probeServiceHealth("127.0.0.1", 1)
		if reachable || ready {
			t.Errorf("probe = (%v, %v), want (false, false)", reachable, ready)
		}
		if probeErr != "" {
			t.Errorf("probeErr = %q, want empty for connection refused", probeErr)
		}
	})
}

func TestEmitServiceStatusJSON(t *testing.T) {
	st := serviceStatus{
		Platform: "darwin", Registered: true, SupervisorState: "active",
		HealthReachable: true, Ready: true, Endpoint: "http://localhost:3100/health", Version: "dev",
	}
	out := captureStdout(t, func() { emitServiceStatus(st, true) })
	var got serviceStatus
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("JSON output not parseable: %v\n%s", err, out)
	}
	if got.Platform != "darwin" || !got.Registered || got.SupervisorState != "active" {
		t.Errorf("round-trip mismatch: %+v", got)
	}
}

func splitHostPort(t *testing.T, url string) (string, int) {
	t.Helper()
	trimmed := strings.TrimPrefix(url, "http://")
	parts := strings.Split(trimmed, ":")
	if len(parts) != 2 {
		t.Fatalf("cannot split %q", url)
	}
	port, err := strconv.Atoi(parts[1])
	if err != nil {
		t.Fatalf("port %q: %v", parts[1], err)
	}
	return parts[0], port
}
