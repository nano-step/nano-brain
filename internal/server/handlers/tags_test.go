package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

type mockTagQuerier struct {
	listTagsByWorkspaceFn func(ctx context.Context, workspaceHash string) ([]sqlc.ListTagsByWorkspaceRow, error)
}

func (m *mockTagQuerier) ListTagsByWorkspace(ctx context.Context, workspaceHash string) ([]sqlc.ListTagsByWorkspaceRow, error) {
	return m.listTagsByWorkspaceFn(ctx, workspaceHash)
}

func TestListTags(t *testing.T) {
	q := &mockTagQuerier{
		listTagsByWorkspaceFn: func(_ context.Context, ws string) ([]sqlc.ListTagsByWorkspaceRow, error) {
			if ws != "ws123" {
				t.Errorf("expected workspace ws123, got %s", ws)
			}
			return []sqlc.ListTagsByWorkspaceRow{
				{Tag: "decision", Count: 5},
				{Tag: "summary", Count: 3},
			}, nil
		},
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tags?workspace=ws123", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", "ws123")

	h := handlers.ListTags(q, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var items []struct {
		Tag   string `json:"tag"`
		Count int64  `json:"count"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&items); err != nil {
		t.Fatal(err)
	}

	if len(items) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(items))
	}
	if items[0].Tag != "decision" || items[0].Count != 5 {
		t.Errorf("unexpected first tag: %+v", items[0])
	}
	if items[1].Tag != "summary" || items[1].Count != 3 {
		t.Errorf("unexpected second tag: %+v", items[1])
	}
}

func TestListTagsEmpty(t *testing.T) {
	q := &mockTagQuerier{
		listTagsByWorkspaceFn: func(_ context.Context, _ string) ([]sqlc.ListTagsByWorkspaceRow, error) {
			return nil, nil
		},
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tags?workspace=ws123", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", "ws123")

	h := handlers.ListTags(q, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var items []interface{}
	if err := json.NewDecoder(rec.Body).Decode(&items); err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Errorf("expected empty list, got %d items", len(items))
	}
}
