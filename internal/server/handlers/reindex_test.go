package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/google/uuid"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/watcher"
	"github.com/rs/zerolog"
)

type mockReindexQuerier struct {
	called      bool
	returnIDs   []uuid.UUID
	returnErr   error
	lastParams  sqlc.ResetAndReturnChunkIDsByCollectionParams
	collections []sqlc.Collection
}

func (m *mockReindexQuerier) ResetAndReturnChunkIDsByCollection(_ context.Context, arg sqlc.ResetAndReturnChunkIDsByCollectionParams) ([]uuid.UUID, error) {
	m.called = true
	m.lastParams = arg
	if m.returnIDs != nil {
		return m.returnIDs, m.returnErr
	}
	return []uuid.UUID{uuid.New()}, m.returnErr
}

func (m *mockReindexQuerier) ListCollections(_ context.Context, _ string) ([]sqlc.Collection, error) {
	if m.collections != nil {
		return m.collections, nil
	}
	return []sqlc.Collection{{Name: "code", Path: "/tmp/code"}}, nil
}

func (m *mockReindexQuerier) DeleteSymbolDocumentsByCollection(_ context.Context, _ sqlc.DeleteSymbolDocumentsByCollectionParams) error {
	return nil
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

	fakeIDs := []uuid.UUID{uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()}
	mq := &mockReindexQuerier{
		returnIDs:   fakeIDs,
		collections: []sqlc.Collection{{Name: "code", Path: "/tmp/code"}},
	}
	w := newTestWatcherForHandler()

	h := handlers.TriggerReindex(mq, w, nil, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !mq.called {
		t.Fatal("expected ResetAndReturnChunkIDsByCollection to be called")
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
	if !strings.Contains(msg, "ws123") {
		t.Errorf("unexpected message: %s", msg)
	}
}

func TestTriggerReindexByPath(t *testing.T) {
	e := echo.New()
	body := `{"workspace":"ws123","root":"/my/project"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/reindex", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", "ws123")

	mq := &mockReindexQuerier{
		returnIDs:   []uuid.UUID{uuid.New(), uuid.New(), uuid.New()},
		collections: []sqlc.Collection{{Name: "code", Path: "/my/project"}},
	}
	w := newTestWatcherForHandler()

	h := handlers.TriggerReindex(mq, w, nil, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rec.Code)
	}
	if !mq.called {
		t.Fatal("expected ResetAndReturnChunkIDsByCollection to be called for path-matched collection")
	}
	if mq.lastParams.Collection != "code" {
		t.Errorf("expected collection name 'code', got %q", mq.lastParams.Collection)
	}
}

func TestTriggerReindexNoRoot(t *testing.T) {
	e := echo.New()
	body := `{"workspace":"ws123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/reindex", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", "ws123")

	mq := &mockReindexQuerier{
		returnIDs:   []uuid.UUID{uuid.New(), uuid.New()},
		collections: []sqlc.Collection{{Name: "code", Path: "/tmp/code"}, {Name: "memory", Path: "/tmp/mem"}},
	}
	w := newTestWatcherForHandler()

	h := handlers.TriggerReindex(mq, w, nil, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rec.Code)
	}
	if !mq.called {
		t.Fatal("expected ResetAndReturnChunkIDsByCollection to be called for all collections")
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
