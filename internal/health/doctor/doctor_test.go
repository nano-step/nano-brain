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
	c, status := doctor.CheckServerRunning("http://127.0.0.1:19999")
	if c.Status != "fail" {
		t.Errorf("status = %q, want fail", c.Status)
	}
	if status != nil {
		t.Errorf("expected nil status on failure")
	}
}

func TestCheckMCPReachable_404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := doctor.CheckMCPReachable(srv.URL + "/mcp")
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
