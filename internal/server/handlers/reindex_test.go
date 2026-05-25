package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/watcher"
	"github.com/rs/zerolog"
)

type mockReindexQuerier struct {
	called     bool
	returnN    int64
	returnErr  error
	lastParams sqlc.ResetEmbedStatusByCollectionParams
}

func (m *mockReindexQuerier) ResetEmbedStatusByCollection(_ context.Context, arg sqlc.ResetEmbedStatusByCollectionParams) (int64, error) {
	m.called = true
	m.lastParams = arg
	return m.returnN, m.returnErr
}

func newTestWatcherForHandler() *watcher.Watcher {
	cfg := config.Config{
		Watcher: config.WatcherConfig{DebounceMs: 2000, ReindexInterval: 300},
		Storage: config.StorageConfig{MaxFileSize: 1024, MaxSize: 10240},
	}
	return watcher.New(nil, nil, zerolog.Nop(), cfg)
}

func TestTriggerReindex(t *testing.T) {
	e := echo.New()
	body := `{"workspace":"ws123","root":"code"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/reindex", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", "ws123")

	mq := &mockReindexQuerier{returnN: 5}
	w := newTestWatcherForHandler()

	h := handlers.TriggerReindex(mq, w, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", rec.Code, rec.Body.String())
	}

	if !mq.called {
		t.Fatal("expected ResetEmbedStatusByCollection to be called")
	}
	if mq.lastParams.WorkspaceHash != "ws123" || mq.lastParams.Collection != "code" {
		t.Errorf("unexpected params: %+v", mq.lastParams)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["status"] != "queued" {
		t.Errorf("expected status=queued, got %v", resp["status"])
	}
	if resp["chunks_enqueued"] != float64(5) {
		t.Errorf("expected chunks_enqueued=5, got %v", resp["chunks_enqueued"])
	}
	msg, _ := resp["message"].(string)
	if !strings.Contains(msg, "code") || !strings.Contains(msg, "ws123") {
		t.Errorf("unexpected message: %s", msg)
	}
}

func TestTriggerReindexMissingRoot(t *testing.T) {
	e := echo.New()
	body := `{"workspace":"ws123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/reindex", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", "ws123")

	mq := &mockReindexQuerier{}
	w := newTestWatcherForHandler()

	h := handlers.TriggerReindex(mq, w, zerolog.Nop())
	err := h(c)
	if err == nil {
		t.Fatal("expected error for missing root")
	}
	he, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected echo.HTTPError, got %T", err)
	}
	if he.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", he.Code)
	}
	if mq.called {
		t.Error("expected ResetEmbedStatusByCollection not to be called on missing root")
	}
}

func TestTriggerUpdate(t *testing.T) {
	e := echo.New()
	body := `{"workspace":"ws123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/update", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", "ws123")

	h := handlers.TriggerUpdate(zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["status"] != "queued" {
		t.Errorf("expected status=queued, got %v", resp["status"])
	}
	msg, _ := resp["message"].(string)
	if !strings.Contains(msg, "ws123") {
		t.Errorf("unexpected message: %s", msg)
	}
}
