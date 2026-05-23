package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
)

type mockHarvestRunner struct {
	harvested int
	skipped   int
	errors    int
}

func (m *mockHarvestRunner) RunOnce(_ context.Context) (int, int, int) {
	return m.harvested, m.skipped, m.errors
}

func TestTriggerHarvest_Success(t *testing.T) {
	runner := handlers.HarvestRunner(&mockHarvestRunner{harvested: 5, skipped: 10, errors: 1})

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/harvest", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := handlers.TriggerHarvest(func() handlers.HarvestRunner { return runner })
	if err := h(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp handlers.HarvestResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Harvested != 5 {
		t.Errorf("harvested = %d, want 5", resp.Harvested)
	}
	if resp.Skipped != 10 {
		t.Errorf("skipped = %d, want 10", resp.Skipped)
	}
	if resp.Errors != 1 {
		t.Errorf("errors = %d, want 1", resp.Errors)
	}
}

func TestTriggerHarvest_NilRunner(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/harvest", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := handlers.TriggerHarvest(func() handlers.HarvestRunner { return nil })
	err := h(c)
	if err == nil {
		t.Fatal("expected error for nil runner")
	}
	he, ok := err.(*echo.HTTPError)
	if !ok || he.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %v", err)
	}
}

func TestTriggerHarvest_NilGetter(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/harvest", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := handlers.TriggerHarvest(func() handlers.HarvestRunner { return nil })
	err := h(c)
	if err == nil {
		t.Fatal("expected error for nil runner")
	}
	he, ok := err.(*echo.HTTPError)
	if !ok || he.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %v", err)
	}
}

func TestTriggerHarvest_ZeroCounts(t *testing.T) {
	runner := handlers.HarvestRunner(&mockHarvestRunner{})

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/harvest", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := handlers.TriggerHarvest(func() handlers.HarvestRunner { return runner })
	if err := h(c); err != nil {
		t.Fatal(err)
	}

	var resp handlers.HarvestResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Harvested != 0 || resp.Skipped != 0 || resp.Errors != 0 {
		t.Errorf("expected all zeros, got %+v", resp)
	}
}

func TestTriggerHarvest_ConcurrentCalls(t *testing.T) {
	runner := handlers.HarvestRunner(&mockHarvestRunner{harvested: 3, skipped: 7, errors: 0})
	getRunner := func() handlers.HarvestRunner { return runner }

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e := echo.New()
			req := httptest.NewRequest(http.MethodPost, "/api/harvest", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			h := handlers.TriggerHarvest(getRunner)
			if err := h(c); err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if rec.Code != http.StatusOK {
				t.Errorf("expected 200, got %d", rec.Code)
				return
			}

			var resp handlers.HarvestResponse
			if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
				t.Errorf("decode error: %v", err)
				return
			}
			if resp.Harvested != 3 || resp.Skipped != 7 || resp.Errors != 0 {
				t.Errorf("unexpected response: %+v", resp)
			}
		}()
	}
	wg.Wait()
}
