package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

type mockWakeUpQuerier struct {
	recentDocsFn       func(ctx context.Context, arg sqlc.RecentDocumentsParams) ([]sqlc.RecentDocumentsRow, error)
	docStatsFn         func(ctx context.Context, ws string) (sqlc.WorkspaceDocStatsRow, error)
	chunkCountFn       func(ctx context.Context, ws string) (int64, error)
	collectionsLastFn  func(ctx context.Context, ws string) ([]sqlc.ListCollectionsWithLastUpdatedRow, error)
}

func (m *mockWakeUpQuerier) RecentDocuments(ctx context.Context, arg sqlc.RecentDocumentsParams) ([]sqlc.RecentDocumentsRow, error) {
	return m.recentDocsFn(ctx, arg)
}

func (m *mockWakeUpQuerier) WorkspaceDocStats(ctx context.Context, ws string) (sqlc.WorkspaceDocStatsRow, error) {
	return m.docStatsFn(ctx, ws)
}

func (m *mockWakeUpQuerier) WorkspaceChunkCount(ctx context.Context, ws string) (int64, error) {
	return m.chunkCountFn(ctx, ws)
}

func (m *mockWakeUpQuerier) ListCollectionsWithLastUpdated(ctx context.Context, ws string) ([]sqlc.ListCollectionsWithLastUpdatedRow, error) {
	return m.collectionsLastFn(ctx, ws)
}

func emptyMock() *mockWakeUpQuerier {
	return &mockWakeUpQuerier{
		recentDocsFn: func(_ context.Context, _ sqlc.RecentDocumentsParams) ([]sqlc.RecentDocumentsRow, error) {
			return []sqlc.RecentDocumentsRow{}, nil
		},
		docStatsFn: func(_ context.Context, _ string) (sqlc.WorkspaceDocStatsRow, error) {
			return sqlc.WorkspaceDocStatsRow{TotalDocuments: 0, LastUpdated: nil}, nil
		},
		chunkCountFn: func(_ context.Context, _ string) (int64, error) {
			return 0, nil
		},
		collectionsLastFn: func(_ context.Context, _ string) ([]sqlc.ListCollectionsWithLastUpdatedRow, error) {
			return []sqlc.ListCollectionsWithLastUpdatedRow{}, nil
		},
	}
}

func TestWakeUp_MissingWorkspace_GET(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/wake-up", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := handlers.WakeUpHandler(emptyMock(), zerolog.Nop())
	err := h(c)
	if err == nil {
		t.Fatal("expected error for missing workspace")
	}
	he, ok := err.(*echo.HTTPError)
	if !ok || he.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %v", err)
	}
}

func TestWakeUp_MissingWorkspace_POST(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/wake-up", strings.NewReader(`{}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := handlers.WakeUpHandler(emptyMock(), zerolog.Nop())
	err := h(c)
	if err == nil {
		t.Fatal("expected error for missing workspace")
	}
	he, ok := err.(*echo.HTTPError)
	if !ok || he.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %v", err)
	}
}

func TestWakeUp_EmptyWorkspace(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/wake-up?workspace=ws1", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := handlers.WakeUpHandler(emptyMock(), zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp handlers.WakeUpResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp.RecentMemories == nil {
		t.Error("expected non-nil recent_memories array")
	}
	if len(resp.RecentMemories) != 0 {
		t.Errorf("expected empty recent_memories, got %d", len(resp.RecentMemories))
	}
	if resp.ActiveCollections == nil {
		t.Error("expected non-nil active_collections array")
	}
	if resp.Stats.TotalDocuments != 0 {
		t.Errorf("expected 0 total_documents, got %d", resp.Stats.TotalDocuments)
	}
}

func TestWakeUp_SummaryTemplate(t *testing.T) {
	now := time.Now()
	q := &mockWakeUpQuerier{
		recentDocsFn: func(_ context.Context, _ sqlc.RecentDocumentsParams) ([]sqlc.RecentDocumentsRow, error) {
			return []sqlc.RecentDocumentsRow{}, nil
		},
		docStatsFn: func(_ context.Context, _ string) (sqlc.WorkspaceDocStatsRow, error) {
			return sqlc.WorkspaceDocStatsRow{TotalDocuments: 42, LastUpdated: now.Add(-2 * time.Hour)}, nil
		},
		chunkCountFn: func(_ context.Context, _ string) (int64, error) {
			return 150, nil
		},
		collectionsLastFn: func(_ context.Context, _ string) ([]sqlc.ListCollectionsWithLastUpdatedRow, error) {
			return []sqlc.ListCollectionsWithLastUpdatedRow{
				{Name: "default", DocumentCount: 30, LastUpdated: now},
				{Name: "sessions", DocumentCount: 12, LastUpdated: now},
				{Name: "codebase", DocumentCount: 0, LastUpdated: nil},
			}, nil
		},
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/wake-up?workspace=ws1", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := handlers.WakeUpHandler(q, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatal(err)
	}

	var resp handlers.WakeUpResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}

	expected := "Workspace has 42 documents across 3 collections. Last activity: 2h ago."
	if resp.Summary != expected {
		t.Errorf("summary mismatch:\n  got:  %q\n  want: %q", resp.Summary, expected)
	}
	if resp.Stats.TotalDocuments != 42 {
		t.Errorf("expected 42 total_documents, got %d", resp.Stats.TotalDocuments)
	}
	if resp.Stats.TotalChunks != 150 {
		t.Errorf("expected 150 total_chunks, got %d", resp.Stats.TotalChunks)
	}
	if len(resp.ActiveCollections) != 3 {
		t.Errorf("expected 3 collections, got %d", len(resp.ActiveCollections))
	}
}

func TestWakeUp_RecentMemoriesOrdering(t *testing.T) {
	now := time.Now()
	id1, id2 := uuid.New(), uuid.New()

	q := &mockWakeUpQuerier{
		recentDocsFn: func(_ context.Context, arg sqlc.RecentDocumentsParams) ([]sqlc.RecentDocumentsRow, error) {
			if arg.MaxResults != 10 {
				t.Errorf("expected default limit 10, got %d", arg.MaxResults)
			}
			return []sqlc.RecentDocumentsRow{
				{ID: id1, Title: "Recent", Tags: []string{"tag1"}, UpdatedAt: now, Snippet: "first doc content"},
				{ID: id2, Title: "Older", Tags: nil, UpdatedAt: now.Add(-time.Hour), Snippet: "second doc"},
			}, nil
		},
		docStatsFn: func(_ context.Context, _ string) (sqlc.WorkspaceDocStatsRow, error) {
			return sqlc.WorkspaceDocStatsRow{TotalDocuments: 2, LastUpdated: now}, nil
		},
		chunkCountFn: func(_ context.Context, _ string) (int64, error) {
			return 5, nil
		},
		collectionsLastFn: func(_ context.Context, _ string) ([]sqlc.ListCollectionsWithLastUpdatedRow, error) {
			return []sqlc.ListCollectionsWithLastUpdatedRow{}, nil
		},
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/wake-up?workspace=ws1", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := handlers.WakeUpHandler(q, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatal(err)
	}

	var resp handlers.WakeUpResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}

	if len(resp.RecentMemories) != 2 {
		t.Fatalf("expected 2 memories, got %d", len(resp.RecentMemories))
	}
	if resp.RecentMemories[0].ID != id1.String() {
		t.Errorf("expected first memory ID=%s, got %s", id1, resp.RecentMemories[0].ID)
	}
	if resp.RecentMemories[0].Title != "Recent" {
		t.Errorf("expected title 'Recent', got %q", resp.RecentMemories[0].Title)
	}
	if len(resp.RecentMemories[0].Tags) != 1 || resp.RecentMemories[0].Tags[0] != "tag1" {
		t.Errorf("expected tags=[tag1], got %v", resp.RecentMemories[0].Tags)
	}
	if resp.RecentMemories[1].Tags == nil {
		t.Error("expected non-nil tags for second memory (should be empty array)")
	}

	if _, err := time.Parse(time.RFC3339, resp.RecentMemories[0].Date); err != nil {
		t.Errorf("date not RFC3339: %q", resp.RecentMemories[0].Date)
	}
}

func TestWakeUp_LimitParam_GET(t *testing.T) {
	now := time.Now()
	var capturedLimit int32
	q := &mockWakeUpQuerier{
		recentDocsFn: func(_ context.Context, arg sqlc.RecentDocumentsParams) ([]sqlc.RecentDocumentsRow, error) {
			capturedLimit = arg.MaxResults
			return []sqlc.RecentDocumentsRow{
				{ID: uuid.New(), Title: "Doc1", Tags: []string{}, UpdatedAt: now, Snippet: "a"},
				{ID: uuid.New(), Title: "Doc2", Tags: []string{}, UpdatedAt: now, Snippet: "b"},
				{ID: uuid.New(), Title: "Doc3", Tags: []string{}, UpdatedAt: now, Snippet: "c"},
			}, nil
		},
		docStatsFn: func(_ context.Context, _ string) (sqlc.WorkspaceDocStatsRow, error) {
			return sqlc.WorkspaceDocStatsRow{TotalDocuments: 5, LastUpdated: now}, nil
		},
		chunkCountFn: func(_ context.Context, _ string) (int64, error) {
			return 10, nil
		},
		collectionsLastFn: func(_ context.Context, _ string) ([]sqlc.ListCollectionsWithLastUpdatedRow, error) {
			return []sqlc.ListCollectionsWithLastUpdatedRow{}, nil
		},
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/wake-up?workspace=ws1&limit=3", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := handlers.WakeUpHandler(q, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if capturedLimit != 3 {
		t.Errorf("expected limit=3 passed to query, got %d", capturedLimit)
	}
}

func TestWakeUp_LimitCapped(t *testing.T) {
	var capturedLimit int32
	q := &mockWakeUpQuerier{
		recentDocsFn: func(_ context.Context, arg sqlc.RecentDocumentsParams) ([]sqlc.RecentDocumentsRow, error) {
			capturedLimit = arg.MaxResults
			return []sqlc.RecentDocumentsRow{}, nil
		},
		docStatsFn: func(_ context.Context, _ string) (sqlc.WorkspaceDocStatsRow, error) {
			return sqlc.WorkspaceDocStatsRow{TotalDocuments: 0, LastUpdated: nil}, nil
		},
		chunkCountFn: func(_ context.Context, _ string) (int64, error) {
			return 0, nil
		},
		collectionsLastFn: func(_ context.Context, _ string) ([]sqlc.ListCollectionsWithLastUpdatedRow, error) {
			return []sqlc.ListCollectionsWithLastUpdatedRow{}, nil
		},
	}

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/wake-up?workspace=ws1&limit=999", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	h := handlers.WakeUpHandler(q, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatal(err)
	}
	if capturedLimit != 50 {
		t.Errorf("expected limit capped at 50, got %d", capturedLimit)
	}
}

func TestWakeUp_POST_WithWorkspaceInBody(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/wake-up", strings.NewReader(`{"workspace":"ws1"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", "ws1")

	h := handlers.WakeUpHandler(emptyMock(), zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}
