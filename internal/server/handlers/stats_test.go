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
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
)

type mockStatsQuerier struct {
	docsTotal       int64
	chunksTotal     int64
	embeddingsTotal int64
}

func (m *mockStatsQuerier) CountDocsByCollectionGrouped(_ context.Context, _ string) ([]sqlc.CountDocsByCollectionGroupedRow, error) {
	return []sqlc.CountDocsByCollectionGroupedRow{{Collection: "memory", DocCount: 5}}, nil
}
func (m *mockStatsQuerier) CountChunksByEmbedStatus(_ context.Context, _ string) ([]sqlc.CountChunksByEmbedStatusRow, error) {
	return []sqlc.CountChunksByEmbedStatusRow{
		{EmbedStatus: "embedded", ChunkCount: 10},
		{EmbedStatus: "pending", ChunkCount: 2},
		{EmbedStatus: "embed_failed", ChunkCount: 1},
	}, nil
}
func (m *mockStatsQuerier) CountGraphEdgesByType(_ context.Context, _ string) ([]sqlc.CountGraphEdgesByTypeRow, error) {
	return []sqlc.CountGraphEdgesByTypeRow{
		{EdgeType: "calls", EdgeCount: 3},
		{EdgeType: "contains", EdgeCount: 7},
	}, nil
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
func (m *mockStatsQuerier) CountDocumentsByWorkspace(_ context.Context, _ string) (int64, error) {
	return m.docsTotal, nil
}
func (m *mockStatsQuerier) CountChunksByWorkspace(_ context.Context, _ string) (int64, error) {
	return m.chunksTotal, nil
}
func (m *mockStatsQuerier) CountEmbeddingsByWorkspace(_ context.Context, _ string) (int64, error) {
	return m.embeddingsTotal, nil
}

type mockWatcherInfo struct{ count int }

func (m *mockWatcherInfo) CollectionsWatched() int { return m.count }

func newTestStatsHandler(q handlers.StatsQuerier) *handlers.StatsHandler {
	getCfg := func() (config.HarvesterConfig, config.IntervalsConfig) {
		return config.HarvesterConfig{}, config.IntervalsConfig{SessionPoll: 120}
	}
	return handlers.NewStatsHandler(
		q, nopLogger(),
		"v1.2.3",
		time.Now().Add(-300*time.Second),
		config.EmbeddingConfig{Provider: "ollama", Model: "nomic-embed-text", Dimension: 768},
		12,
		getCfg,
		config.WatcherConfig{DebounceMs: 2000},
		&mockWatcherInfo{count: 4},
	)
}

func TestStats_ResponseShape(t *testing.T) {
	q := &mockStatsQuerier{docsTotal: 5, chunksTotal: 13, embeddingsTotal: 11}
	h := newTestStatsHandler(q)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats?workspace=abc123", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", "abc123")

	if err := h.Handle(c); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}

	expected := []string{
		"server_version", "uptime_sec", "embedding", "migration_version",
		"docs_total", "chunks_total", "chunks_by_embed_status", "embeddings_total",
		"graph_edges_by_type", "collections", "tags_top_20", "harvest", "watcher",
		"recent_docs",
	}
	for _, k := range expected {
		if _, ok := resp[k]; !ok {
			t.Errorf("missing required field %q", k)
		}
	}

	for _, legacy := range []string{"chunks", "graph_edges", "top_tags", "recent_queries"} {
		if _, ok := resp[legacy]; ok {
			t.Errorf("legacy field %q must not be present", legacy)
		}
	}

	if v := resp["server_version"]; v != "v1.2.3" {
		t.Errorf("server_version = %v, want v1.2.3", v)
	}

	cbes, ok := resp["chunks_by_embed_status"].(map[string]interface{})
	if !ok {
		t.Fatalf("chunks_by_embed_status is %T, want map[string]interface{}", resp["chunks_by_embed_status"])
	}
	for _, k := range []string{"pending", "embedded", "embed_failed"} {
		if _, ok := cbes[k]; !ok {
			t.Errorf("chunks_by_embed_status missing key %q", k)
		}
	}
	if v := cbes["embedded"]; v != float64(10) {
		t.Errorf("chunks_by_embed_status.embedded = %v, want 10", v)
	}

	geb, ok := resp["graph_edges_by_type"].(map[string]interface{})
	if !ok {
		t.Fatalf("graph_edges_by_type is %T, want map[string]interface{}", resp["graph_edges_by_type"])
	}
	if v := geb["calls"]; v != float64(3) {
		t.Errorf("graph_edges_by_type.calls = %v, want 3", v)
	}

	tags, ok := resp["tags_top_20"].([]interface{})
	if !ok {
		t.Fatalf("tags_top_20 is %T, want array", resp["tags_top_20"])
	}
	if len(tags) > 0 {
		tag0 := tags[0].(map[string]interface{})
		if _, ok := tag0["count"]; !ok {
			t.Error("tag entry missing 'count' field (must NOT be 'doc_count')")
		}
		if _, ok := tag0["doc_count"]; ok {
			t.Error("tag entry has legacy 'doc_count' field; must be 'count'")
		}
	}

	emb, ok := resp["embedding"].(map[string]interface{})
	if !ok {
		t.Fatalf("embedding is %T, want object", resp["embedding"])
	}
	for _, k := range []string{"provider", "model", "dim"} {
		if _, ok := emb[k]; !ok {
			t.Errorf("embedding missing key %q", k)
		}
	}

	if v := resp["docs_total"]; v != float64(5) {
		t.Errorf("docs_total = %v, want 5", v)
	}
	if v := resp["chunks_total"]; v != float64(13) {
		t.Errorf("chunks_total = %v, want 13", v)
	}
	if v := resp["embeddings_total"]; v != float64(11) {
		t.Errorf("embeddings_total = %v, want 11", v)
	}

	w, ok := resp["watcher"].(map[string]interface{})
	if !ok {
		t.Fatalf("watcher is %T, want object", resp["watcher"])
	}
	if v := w["collections_watched"]; v != float64(4) {
		t.Errorf("watcher.collections_watched = %v, want 4", v)
	}
	if v := w["debounce_ms"]; v != float64(2000) {
		t.Errorf("watcher.debounce_ms = %v, want 2000", v)
	}
}

func TestStats_EmptyWorkspace(t *testing.T) {
	q := &mockStatsQuerier{docsTotal: 0, chunksTotal: 0, embeddingsTotal: 0}
	q.docsTotal = 0
	h := handlers.NewStatsHandler(
		&emptyStatsQuerier{}, nopLogger(),
		"v0.0.0",
		time.Now(),
		config.EmbeddingConfig{Provider: "ollama", Model: "nomic-embed-text", Dimension: 768},
		0,
		nil,
		config.WatcherConfig{},
		nil,
	)

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/stats?workspace=empty", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", "empty")

	if err := h.Handle(c); err != nil {
		t.Fatal(err)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if v := resp["docs_total"]; v != float64(0) {
		t.Errorf("docs_total = %v, want 0", v)
	}
	cbes := resp["chunks_by_embed_status"].(map[string]interface{})
	if v := cbes["pending"]; v != float64(0) {
		t.Errorf("chunks_by_embed_status.pending = %v, want 0", v)
	}
}

type emptyStatsQuerier struct{}

func (e *emptyStatsQuerier) CountDocsByCollectionGrouped(_ context.Context, _ string) ([]sqlc.CountDocsByCollectionGroupedRow, error) {
	return nil, nil
}
func (e *emptyStatsQuerier) CountChunksByEmbedStatus(_ context.Context, _ string) ([]sqlc.CountChunksByEmbedStatusRow, error) {
	return nil, nil
}
func (e *emptyStatsQuerier) CountGraphEdgesByType(_ context.Context, _ string) ([]sqlc.CountGraphEdgesByTypeRow, error) {
	return nil, nil
}
func (e *emptyStatsQuerier) ListTopTags(_ context.Context, _ string) ([]sqlc.ListTopTagsRow, error) {
	return nil, nil
}
func (e *emptyStatsQuerier) ListRecentDocuments(_ context.Context, _ string) ([]sqlc.ListRecentDocumentsRow, error) {
	return nil, nil
}
func (e *emptyStatsQuerier) CountDocumentsByWorkspace(_ context.Context, _ string) (int64, error) {
	return 0, nil
}
func (e *emptyStatsQuerier) CountChunksByWorkspace(_ context.Context, _ string) (int64, error) {
	return 0, nil
}
func (e *emptyStatsQuerier) CountEmbeddingsByWorkspace(_ context.Context, _ string) (int64, error) {
	return 0, nil
}
