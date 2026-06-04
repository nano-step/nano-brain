package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
	"github.com/rs/zerolog"
)

type mockResetQuerier struct {
	countFn      func(ctx context.Context, hash string) (int64, error)
	delDocsFn    func(ctx context.Context, hash string) error
	delWsFn      func(ctx context.Context, hash string) error
	docDelCalled bool
	wsDelCalled  bool
}

func (m *mockResetQuerier) CountDocumentsByWorkspace(ctx context.Context, hash string) (int64, error) {
	if m.countFn != nil {
		return m.countFn(ctx, hash)
	}
	return 5, nil
}

func (m *mockResetQuerier) DeleteDocumentsByWorkspace(ctx context.Context, hash string) error {
	m.docDelCalled = true
	if m.delDocsFn != nil {
		return m.delDocsFn(ctx, hash)
	}
	return nil
}

func (m *mockResetQuerier) DeleteWorkspace(ctx context.Context, hash string) error {
	m.wsDelCalled = true
	if m.delWsFn != nil {
		return m.delWsFn(ctx, hash)
	}
	return nil
}

func TestResetWorkspace_NonTxPath(t *testing.T) {
	q := &mockResetQuerier{}
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/reset", strings.NewReader(`{"workspace":"ws1"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := handlers.ResetWorkspace(q, nil, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rec.Code)
	}
	if !q.docDelCalled {
		t.Errorf("expected documents deleted: docs=%v", q.docDelCalled)
	}
	if q.wsDelCalled {
		t.Errorf("workspace should NOT be deleted: ws=%v", q.wsDelCalled)
	}
	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["workspace"] != "ws1" {
		t.Errorf("workspace mismatch: %v", resp["workspace"])
	}
}

func TestResetWorkspace_MissingWorkspace(t *testing.T) {
	q := &mockResetQuerier{}
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/reset", strings.NewReader(`{}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := handlers.ResetWorkspace(q, nil, zerolog.Nop())
	err := h(c)
	if err == nil {
		t.Fatal("expected error for missing workspace")
	}
	if he, ok := err.(*echo.HTTPError); !ok || he.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %v", err)
	}
}
