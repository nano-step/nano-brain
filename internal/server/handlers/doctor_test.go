package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
)

func TestDoctor_ReturnsArray(t *testing.T) {
	e := echo.New()
	deps := handlers.DoctorDeps{
		ConfigPath: "/tmp/nonexistent.yml",
		LoadConfig: func() (*config.Config, error) {
			return &config.Config{}, nil
		},
	}
	h := handlers.Doctor(deps, nopLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/doctor", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	if err := h(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}

	var resp struct {
		Checks    []json.RawMessage `json:"checks"`
		AllPassed bool              `json:"all_passed"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if len(resp.Checks) == 0 {
		t.Error("expected at least one check")
	}

	var first struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(resp.Checks[0], &first); err != nil {
		t.Fatal(err)
	}
	validStatuses := map[string]bool{"ok": true, "warn": true, "fail": true, "skip": true}
	if !validStatuses[first.Status] {
		t.Errorf("unexpected status %q", first.Status)
	}
}
