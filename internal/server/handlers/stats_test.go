package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
)

type mockStatsQuerier struct{}

func (m *mockStatsQuerier) CountDocsByCollectionGrouped(_ context.Context, _ string) ([]sqlc.CountDocsByCollectionGroupedRow, error) {
	return []sqlc.CountDocsByCollectionGroupedRow{{Collection: "memory", DocCount: 5}}, nil
}
func (m *mockStatsQuerier) CountChunksByEmbedStatus(_ context.Context, _ string) ([]sqlc.CountChunksByEmbedStatusRow, error) {
	return []sqlc.CountChunksByEmbedStatusRow{{EmbedStatus: "embedded", ChunkCount: 10}}, nil
}
func (m *mockStatsQuerier) CountGraphEdgesByType(_ context.Context, _ string) ([]sqlc.CountGraphEdgesByTypeRow, error) {
	return []sqlc.CountGraphEdgesByTypeRow{{EdgeType: "calls", EdgeCount: 3}}, nil
}
func (m *mockStatsQuerier) ListTopTags(_ context.Context, _ string) ([]sqlc.ListTopTagsRow, error) {
	return []sqlc.ListTopTagsRow{{Tag: "decision", DocCount: 2}}, nil
}
func (m *mockStatsQuerier) ListRecentDocuments(_ context.Context, _ string) ([]sqlc.ListRecentDocumentsRow, error) {
	return []sqlc.ListRecentDocumentsRow{{
		ID: uuid.New(), Title: "Test Doc", Collection: "memory",
		UpdatedAt: time.Now(), Tags: []string{"test"},
	}}, nil
}
func (m *mockStatsQuerier) ListRecentQueries(_ context.Context, _ string) ([]sqlc.ListRecentQueriesRow, error) {
	return []sqlc.ListRecentQueriesRow{{QueryText: "database", CreatedAt: time.Now()}}, nil
}

func TestStats_PopulatedWorkspace(t *testing.T) {
	e := echo.New()
	h := handlers.Stats(&mockStatsQuerier{}, nopLogger())

	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats?workspace=abc123", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", "abc123")

	if err := h(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}

	if cols, ok := resp["collections"].([]interface{}); !ok || len(cols) == 0 {
		t.Error("expected non-empty collections")
	}
	if docs, ok := resp["recent_docs"].([]interface{}); !ok || len(docs) == 0 {
		t.Error("expected non-empty recent_docs")
	}
}
