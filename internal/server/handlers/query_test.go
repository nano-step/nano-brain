package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/search"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
	"github.com/rs/zerolog"
)

type mockSearcher struct {
	results      []search.Result
	err          error
	defaultLimit int
	capturedTags []string
}

func (m *mockSearcher) HybridSearch(_ context.Context, _ string, _ string, _ int, tags []string, _ *search.TimeRangeFilter, _ string, _ string) ([]search.Result, error) {
	m.capturedTags = tags
	return m.results, m.err
}

func (m *mockSearcher) DefaultLimit() int {
	if m.defaultLimit > 0 {
		return m.defaultLimit
	}
	return 20
}

func queryRequest(body string, workspace string) (*httptest.ResponseRecorder, echo.Context) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/query", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if workspace != "" {
		c.Set("workspace", workspace)
	}
	return rec, c
}

func TestQuery_Success(t *testing.T) {
	ms := &mockSearcher{results: []search.Result{
		{ID: "r1", Title: "Result One", Content: "snippet", Score: 0.9},
	}}
	h := handlers.Query(ms, zerolog.Nop())

	rec, c := queryRequest(`{"query":"test"}`, "ws1")
	if err := h(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp handlers.SearchResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Total != 1 {
		t.Errorf("expected total=1, got %d", resp.Total)
	}
	if resp.Results[0].ID != "r1" {
		t.Errorf("expected ID=r1, got %s", resp.Results[0].ID)
	}
}

func TestQuery_EmptyQuery(t *testing.T) {
	h := handlers.Query(&mockSearcher{}, zerolog.Nop())
	_, c := queryRequest(`{"query":""}`, "ws1")
	err := h(c)
	if err == nil {
		t.Fatal("expected error for empty query")
	}
	he, ok := err.(*echo.HTTPError)
	if !ok || he.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %v", err)
	}
}

func TestQuery_MissingWorkspace(t *testing.T) {
	h := handlers.Query(&mockSearcher{}, zerolog.Nop())
	_, c := queryRequest(`{"query":"test"}`, "")
	err := h(c)
	if err == nil {
		t.Fatal("expected error for missing workspace")
	}
	he, ok := err.(*echo.HTTPError)
	if !ok || he.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %v", err)
	}
}

func TestQuery_DefaultMaxResults(t *testing.T) {
	h := handlers.Query(&mockSearcher{}, zerolog.Nop())
	rec, c := queryRequest(`{"query":"test"}`, "ws1")
	if err := h(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestQuery_MaxResultsCapping(t *testing.T) {
	h := handlers.Query(&mockSearcher{}, zerolog.Nop())
	rec, c := queryRequest(`{"query":"test","max_results":500}`, "ws1")
	if err := h(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 even with oversized max_results, got %d", rec.Code)
	}
}

func TestQuery_WithTags(t *testing.T) {
	ms := &mockSearcher{results: []search.Result{
		{ID: "r1", Title: "Tagged Result", Content: "snippet", Score: 0.8},
	}}
	h := handlers.Query(ms, zerolog.Nop())

	rec, c := queryRequest(`{"query":"test","tags":["decision","auth"]}`, "ws1")
	if err := h(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	if len(ms.capturedTags) != 2 || ms.capturedTags[0] != "decision" || ms.capturedTags[1] != "auth" {
		t.Errorf("expected tags=[decision,auth], got %v", ms.capturedTags)
	}

	var resp handlers.SearchResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.Total != 1 {
		t.Errorf("expected total=1, got %d", resp.Total)
	}
}

func TestQuery_WithoutTags(t *testing.T) {
	ms := &mockSearcher{}
	h := handlers.Query(ms, zerolog.Nop())

	_, c := queryRequest(`{"query":"test"}`, "ws1")
	if err := h(c); err != nil {
		t.Fatal(err)
	}

	if ms.capturedTags != nil {
		t.Errorf("expected nil tags when not provided, got %v", ms.capturedTags)
	}
}
