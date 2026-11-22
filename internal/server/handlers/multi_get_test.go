package handlers_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

type mockMultiGetQuerier struct {
	getByID         func(ctx context.Context, arg sqlc.GetDocumentByIDParams) (sqlc.DocumentView, error)
	getBySourcePath func(ctx context.Context, arg sqlc.GetDocumentBySourcePathParams) (sqlc.DocumentView, error)
}

func (m *mockMultiGetQuerier) GetDocumentByID(ctx context.Context, arg sqlc.GetDocumentByIDParams) (sqlc.DocumentView, error) {
	return m.getByID(ctx, arg)
}

func (m *mockMultiGetQuerier) GetDocumentBySourcePath(ctx context.Context, arg sqlc.GetDocumentBySourcePathParams) (sqlc.DocumentView, error) {
	return m.getBySourcePath(ctx, arg)
}

func TestMultiGet_ByPaths(t *testing.T) {
	docA := uuid.New()
	docB := uuid.New()

	q := &mockMultiGetQuerier{
		getBySourcePath: func(_ context.Context, arg sqlc.GetDocumentBySourcePathParams) (sqlc.DocumentView, error) {
			switch arg.SourcePath {
			case "memory://a.md":
				return sampleDoc(docA, "memory://a.md"), nil
			case "memory://b.md":
				return sampleDoc(docB, "memory://b.md"), nil
			case "memory://missing.md":
				return sqlc.DocumentView{}, sql.ErrNoRows
			default:
				return sqlc.DocumentView{}, sql.ErrNoRows
			}
		},
	}

	e := echo.New()
	body := `{"paths":["memory://a.md","memory://b.md","memory://missing.md"]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/multi-get", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", "ws123")

	h := handlers.MultiGet(q, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	results, ok := resp["results"].([]interface{})
	if !ok {
		t.Fatalf("results not a slice: %T", resp["results"])
	}
	if len(results) != 2 {
		t.Errorf("results len = %d, want 2", len(results))
	}
	notFound, ok := resp["not_found"].([]interface{})
	if !ok {
		t.Fatalf("not_found not a slice: %T", resp["not_found"])
	}
	if len(notFound) != 1 {
		t.Errorf("not_found len = %d, want 1", len(notFound))
	}
	if notFound[0] != "memory://missing.md" {
		t.Errorf("not_found[0] = %v, want memory://missing.md", notFound[0])
	}
}

func TestMultiGet_ByIDs(t *testing.T) {
	docA := uuid.New()
	badID := "not-a-uuid"

	q := &mockMultiGetQuerier{
		getByID: func(_ context.Context, arg sqlc.GetDocumentByIDParams) (sqlc.DocumentView, error) {
			if arg.ID == docA {
				return sampleDoc(docA, "memory://a.md"), nil
			}
			return sqlc.DocumentView{}, sql.ErrNoRows
		},
	}

	e := echo.New()
	body := fmt.Sprintf(`{"ids":["%s","%s"]}`, docA.String(), badID)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/multi-get", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", "ws123")

	h := handlers.MultiGet(q, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	results := resp["results"].([]interface{})
	if len(results) != 1 {
		t.Errorf("results len = %d, want 1", len(results))
	}
	notFound := resp["not_found"].([]interface{})
	if len(notFound) != 1 {
		t.Errorf("not_found len = %d, want 1", len(notFound))
	}
	if notFound[0] != badID {
		t.Errorf("not_found[0] = %v, want %s", notFound[0], badID)
	}
}

func TestMultiGet_NeitherPathsNorIDs(t *testing.T) {
	q := &mockMultiGetQuerier{}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/multi-get", strings.NewReader(`{}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", "ws123")

	h := handlers.MultiGet(q, zerolog.Nop())
	err := h(c)
	if err == nil {
		t.Fatal("expected error for missing paths/ids")
	}
	he, ok := err.(*echo.HTTPError)
	if !ok {
		t.Fatalf("expected HTTPError, got %T", err)
	}
	if he.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", he.Code)
	}
}

func TestMultiGet_EmptyPaths(t *testing.T) {
	q := &mockMultiGetQuerier{
		getBySourcePath: func(_ context.Context, _ sqlc.GetDocumentBySourcePathParams) (sqlc.DocumentView, error) {
			return sqlc.DocumentView{}, sql.ErrNoRows
		},
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/multi-get", strings.NewReader(`{"paths":["gone"]}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", "ws123")

	h := handlers.MultiGet(q, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	results := resp["results"].([]interface{})
	if len(results) != 0 {
		t.Errorf("results len = %d, want 0", len(results))
	}
	notFound := resp["not_found"].([]interface{})
	if len(notFound) != 1 {
		t.Errorf("not_found len = %d, want 1", len(notFound))
	}
}

// Each entry is one query, so the total of paths and ids is capped (#673).
func TestMultiGet_EntryCap(t *testing.T) {
	entries := func(n int) []string {
		out := make([]string, n)
		for i := range out {
			out[i] = fmt.Sprintf("memory://%d.md", i)
		}
		return out
	}

	tests := []struct {
		name       string
		paths, ids int
		want       int
	}{
		{"paths at the cap", 200, 0, http.StatusOK},
		{"paths and ids at the cap", 120, 80, http.StatusOK},
		{"paths over the cap", 201, 0, http.StatusBadRequest},
		{"ids over the cap", 0, 201, http.StatusBadRequest},
		{"paths and ids together over the cap", 120, 81, http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queries := 0
			q := &mockMultiGetQuerier{
				getBySourcePath: func(context.Context, sqlc.GetDocumentBySourcePathParams) (sqlc.DocumentView, error) {
					queries++
					return sqlc.DocumentView{}, sql.ErrNoRows
				},
			}
			body, err := json.Marshal(map[string][]string{"paths": entries(tt.paths), "ids": entries(tt.ids)})
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodPost, "/api/v1/multi-get", strings.NewReader(string(body)))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			c := echo.New().NewContext(req, rec)
			c.Set("workspace", "ws123")

			got := rec.Code
			var msg string
			if err := handlers.MultiGet(q, zerolog.Nop())(c); err != nil {
				he, ok := err.(*echo.HTTPError)
				if !ok {
					t.Fatalf("unexpected error type %T: %v", err, err)
				}
				got, msg = he.Code, fmt.Sprint(he.Message)
			}
			if got != tt.want {
				t.Fatalf("status %d, want %d", got, tt.want)
			}
			if tt.want == http.StatusBadRequest {
				if queries != 0 {
					t.Errorf("ran %d queries before rejecting", queries)
				}
				if want := "too many entries: paths and ids together must not exceed 200"; msg != want {
					t.Errorf("message %q, want %q", msg, want)
				}
			}
		})
	}
}
