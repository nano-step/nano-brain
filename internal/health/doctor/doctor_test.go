package doctor_test

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/health/doctor"
)

func TestCheckBinaryExists_Present(t *testing.T) {
	tmp, err := os.CreateTemp(t.TempDir(), "nb-*")
	if err != nil {
		t.Fatal(err)
	}
	tmp.Close()
	if err := os.Chmod(tmp.Name(), 0o755); err != nil {
		t.Fatal(err)
	}

	c := doctor.CheckBinaryExists(tmp.Name())
	if c.Status != "ok" {
		t.Errorf("status = %q, want ok", c.Status)
	}
}

func TestCheckBinaryExists_Missing(t *testing.T) {
	c := doctor.CheckBinaryExists("/nonexistent/path/nano-brain")
	if c.Status != "fail" {
		t.Errorf("status = %q, want fail", c.Status)
	}
}

func TestCheckBinaryExists_NotExecutable(t *testing.T) {
	tmp, err := os.CreateTemp(t.TempDir(), "nb-*")
	if err != nil {
		t.Fatal(err)
	}
	tmp.Close()
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		t.Fatal(err)
	}

	c := doctor.CheckBinaryExists(tmp.Name())
	if c.Status != "fail" {
		t.Errorf("status = %q, want fail", c.Status)
	}
}

func TestCheckQueueHealth_Nominal(t *testing.T) {
	c := doctor.CheckQueueHealth(doctor.StatusResponse{QueuePending: 5, QueueCapacity: 10})
	if c.Status != "ok" {
		t.Errorf("status = %q, want ok", c.Status)
	}
}

func TestCheckQueueHealth_Warn80(t *testing.T) {
	c := doctor.CheckQueueHealth(doctor.StatusResponse{QueuePending: 80, QueueCapacity: 100})
	if c.Status != "warn" {
		t.Errorf("status = %q, want warn", c.Status)
	}
}

func TestCheckQueueHealth_Fail95(t *testing.T) {
	c := doctor.CheckQueueHealth(doctor.StatusResponse{QueuePending: 95, QueueCapacity: 100})
	if c.Status != "fail" {
		t.Errorf("status = %q, want fail", c.Status)
	}
}

func TestCheckVersionSkew_Match(t *testing.T) {
	c := doctor.CheckVersionSkew("v1.2.3", "v1.2.3")
	if c.Status != "ok" {
		t.Errorf("status = %q, want ok", c.Status)
	}
}

func TestCheckVersionSkew_Mismatch(t *testing.T) {
	c := doctor.CheckVersionSkew("v1.2.3", "v1.2.4")
	if c.Status != "warn" {
		t.Errorf("status = %q, want warn", c.Status)
	}
}

func TestCheckServerRunning_Unreachable(t *testing.T) {
	c, status := doctor.CheckServerRunning("http://127.0.0.1:19999", "")
	if c.Status != "fail" {
		t.Errorf("status = %q, want fail", c.Status)
	}
	if status != nil {
		t.Errorf("expected nil status on failure")
	}
}

func TestOnlineChecksBearerToken(t *testing.T) {
	const token = "doctor-test-token"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/api/status":
			_, _ = w.Write([]byte(`{"version":"test"}`))
		case "/mcp":
			_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	check, status := doctor.CheckServerRunning(srv.URL, token)
	if check.Status != "ok" || status == nil {
		t.Fatalf("status probe failed: %+v", check)
	}
	if check := doctor.CheckMCPReachable(srv.URL+"/mcp", token); check.Status != "ok" {
		t.Fatalf("MCP probe failed: %+v", check)
	}
}

func TestCheckMCPReachable_404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := doctor.CheckMCPReachable(srv.URL+"/mcp", "")
	if c.Status != "fail" {
		t.Errorf("status = %q, want fail", c.Status)
	}
}

// Regression test for #633: the stateless streamable transport answers
// GET /mcp with 405, so the probe must POST a JSON-RPC initialize and treat
// a well-formed JSON-RPC payload (SSE or plain JSON) as reachable.
func TestCheckMCPReachable_OK_SSE(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":1,\"result\":{\"serverInfo\":{\"name\":\"nano-brain\"}}}\n\n"))
	}))
	defer srv.Close()

	c := doctor.CheckMCPReachable(srv.URL+"/mcp", "")
	if c.Status != "ok" {
		t.Errorf("status = %q, want ok (hint: %s)", c.Status, c.Hint)
	}
}

func TestCheckMCPReachable_OK_JSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"serverInfo":{"name":"nano-brain"}}}`))
	}))
	defer srv.Close()

	c := doctor.CheckMCPReachable(srv.URL+"/mcp", "")
	if c.Status != "ok" {
		t.Errorf("status = %q, want ok (hint: %s)", c.Status, c.Hint)
	}
}

func TestCheckMCPReachable_200_Malformed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<html>not jsonrpc</html>`))
	}))
	defer srv.Close()

	c := doctor.CheckMCPReachable(srv.URL+"/mcp", "")
	if c.Status != "fail" {
		t.Errorf("status = %q, want fail", c.Status)
	}
}

func TestCheckEmbeddingProvider_Disabled(t *testing.T) {
	c, body := doctor.CheckEmbeddingProvider(config.EmbeddingConfig{Provider: ""})
	if c.Status != "skip" {
		t.Errorf("status = %q, want skip", c.Status)
	}
	if c.Detail != "disabled — BM25-only" {
		t.Errorf("detail = %q, want %q", c.Detail, "disabled — BM25-only")
	}
	if body != nil {
		t.Errorf("ollamaBody = %v, want nil", body)
	}
}

func TestCheckEmbeddingModel_Disabled(t *testing.T) {
	c := doctor.CheckEmbeddingModel(config.EmbeddingConfig{Provider: ""}, nil)
	if c.Status != "skip" {
		t.Errorf("status = %q, want skip", c.Status)
	}
	if c.Detail != "disabled — BM25-only" {
		t.Errorf("detail = %q, want %q", c.Detail, "disabled — BM25-only")
	}
}

func TestCheckEmbeddingProvider_VoyageConfigured(t *testing.T) {
	c, _ := doctor.CheckEmbeddingProvider(config.EmbeddingConfig{Provider: "voyage", VoyageAPIKey: "k"})
	if c.Status != "ok" {
		t.Errorf("status = %q, want ok", c.Status)
	}
}

func TestCheckConfig_FileDoesNotExistHint(t *testing.T) {
	c := doctor.CheckConfig("/tmp/missing-explicit.yml", os.ErrNotExist)
	if c.Status != "fail" {
		t.Errorf("status = %q, want fail", c.Status)
	}
	if c.Hint == "" || c.Detail != "/tmp/missing-explicit.yml" {
		t.Errorf("unexpected check: %+v", c)
	}
}
