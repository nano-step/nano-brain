package handlers_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/server/handlers"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/watcher"
	"github.com/rs/zerolog"
)

type mockReindexQuerier struct {
	collections              []sqlc.Collection
	indexedDocs              map[string][]sqlc.ListDocumentSourcePathsAndHashesRow
	deleteChunksCalled       int
	deleteDocCalled          int
	forceWipeIDs             []uuid.UUID
	forceWipeCalled          bool
	forceWipeLastParams      sqlc.ResetAndReturnChunkIDsByCollectionParams
}

func (m *mockReindexQuerier) ListCollections(_ context.Context, _ string) ([]sqlc.Collection, error) {
	if m.collections != nil {
		return m.collections, nil
	}
	return []sqlc.Collection{{Name: "code", Path: "/tmp/code"}}, nil
}

func (m *mockReindexQuerier) ListDocumentSourcePathsAndHashes(_ context.Context, arg sqlc.ListDocumentSourcePathsAndHashesParams) ([]sqlc.ListDocumentSourcePathsAndHashesRow, error) {
	if m.indexedDocs != nil {
		return m.indexedDocs[arg.Collection], nil
	}
	return nil, nil
}

func (m *mockReindexQuerier) DeleteChunksByDocumentID(_ context.Context, _ sqlc.DeleteChunksByDocumentIDParams) error {
	m.deleteChunksCalled++
	return nil
}

func (m *mockReindexQuerier) DeleteDocumentByIDAndWorkspace(_ context.Context, _ sqlc.DeleteDocumentByIDAndWorkspaceParams) (int64, error) {
	m.deleteDocCalled++
	return 1, nil
}

func (m *mockReindexQuerier) DeleteSymbolDocumentsByCollection(_ context.Context, _ sqlc.DeleteSymbolDocumentsByCollectionParams) error {
	return nil
}

func (m *mockReindexQuerier) ResetAndReturnChunkIDsByCollection(_ context.Context, arg sqlc.ResetAndReturnChunkIDsByCollectionParams) ([]uuid.UUID, error) {
	m.forceWipeCalled = true
	m.forceWipeLastParams = arg
	if m.forceWipeIDs != nil {
		return m.forceWipeIDs, nil
	}
	return []uuid.UUID{uuid.New()}, nil
}

func newTestWatcherForHandler() *watcher.Watcher {
	cfg := config.Config{
		Watcher: config.WatcherConfig{DebounceMs: 2000, ReindexInterval: 300},
		Storage: config.StorageConfig{MaxFileSize: 1024, MaxSize: 10240},
	}
	return watcher.New(nil, nil, zerolog.Nop(), cfg)
}

func fileHash(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file for hash: %v", err)
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func TestIncrementalReindex_NewFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "new.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	e := echo.New()
	body := `{"workspace":"ws123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/reindex", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", "ws123")

	mq := &mockReindexQuerier{
		collections: []sqlc.Collection{{Name: "code", Path: dir}},
		indexedDocs: map[string][]sqlc.ListDocumentSourcePathsAndHashesRow{
			"code": {},
		},
	}
	w := newTestWatcherForHandler()

	h := handlers.TriggerReindex(mq, w, nil, nil, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["embedded"] != float64(1) {
		t.Errorf("expected embedded=1, got %v", resp["embedded"])
	}
	if resp["skipped"] != float64(0) {
		t.Errorf("expected skipped=0, got %v", resp["skipped"])
	}
	if resp["deleted"] != float64(0) {
		t.Errorf("expected deleted=0, got %v", resp["deleted"])
	}
}

func TestIncrementalReindex_UnchangedFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "same.txt")
	if err := os.WriteFile(filePath, []byte("unchanged"), 0o644); err != nil {
		t.Fatal(err)
	}
	hash := fileHash(t, filePath)
	docID := uuid.New()

	e := echo.New()
	body := `{"workspace":"ws123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/reindex", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", "ws123")

	mq := &mockReindexQuerier{
		collections: []sqlc.Collection{{Name: "code", Path: dir}},
		indexedDocs: map[string][]sqlc.ListDocumentSourcePathsAndHashesRow{
			"code": {{ID: docID, SourcePath: filePath, ContentHash: hash}},
		},
	}
	w := newTestWatcherForHandler()

	h := handlers.TriggerReindex(mq, w, nil, nil, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["skipped"] != float64(1) {
		t.Errorf("expected skipped=1, got %v", resp["skipped"])
	}
	if resp["embedded"] != float64(0) {
		t.Errorf("expected embedded=0, got %v", resp["embedded"])
	}
	if resp["deleted"] != float64(0) {
		t.Errorf("expected deleted=0, got %v", resp["deleted"])
	}
	if mq.deleteChunksCalled != 0 {
		t.Errorf("expected no DeleteChunks calls, got %d", mq.deleteChunksCalled)
	}
}

func TestIncrementalReindex_ChangedFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "changed.txt")
	if err := os.WriteFile(filePath, []byte("new content"), 0o644); err != nil {
		t.Fatal(err)
	}
	docID := uuid.New()

	e := echo.New()
	body := `{"workspace":"ws123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/reindex", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", "ws123")

	mq := &mockReindexQuerier{
		collections: []sqlc.Collection{{Name: "code", Path: dir}},
		indexedDocs: map[string][]sqlc.ListDocumentSourcePathsAndHashesRow{
			"code": {{ID: docID, SourcePath: filePath, ContentHash: "old-hash-different"}},
		},
	}
	w := newTestWatcherForHandler()

	h := handlers.TriggerReindex(mq, w, nil, nil, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["embedded"] != float64(1) {
		t.Errorf("expected embedded=1, got %v", resp["embedded"])
	}
	if mq.deleteChunksCalled != 1 {
		t.Errorf("expected 1 DeleteChunks call, got %d", mq.deleteChunksCalled)
	}
}

func TestIncrementalReindex_DeletedFile(t *testing.T) {
	dir := t.TempDir()
	docID := uuid.New()
	ghostPath := filepath.Join(dir, "ghost.txt")

	// Create a valid file in the directory so diskFiles is non-empty.
	// This ensures the orphan deletion guard (Fix 1) doesn't skip deletion.
	validPath := filepath.Join(dir, "keep.txt")
	if err := os.WriteFile(validPath, []byte("data"), 0644); err != nil {
		t.Fatalf("failed to create valid file: %v", err)
	}

	e := echo.New()
	body := `{"workspace":"ws123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/reindex", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", "ws123")

	mq := &mockReindexQuerier{
		collections: []sqlc.Collection{{Name: "code", Path: dir}},
		indexedDocs: map[string][]sqlc.ListDocumentSourcePathsAndHashesRow{
			"code": {{ID: docID, SourcePath: ghostPath, ContentHash: "some-hash"}},
		},
	}
	w := newTestWatcherForHandler()

	h := handlers.TriggerReindex(mq, w, nil, nil, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["deleted"] != float64(1) {
		t.Errorf("expected deleted=1, got %v", resp["deleted"])
	}
	if mq.deleteChunksCalled != 1 {
		t.Errorf("expected 1 DeleteChunks call, got %d", mq.deleteChunksCalled)
	}
	if mq.deleteDocCalled != 1 {
		t.Errorf("expected 1 DeleteDoc call, got %d", mq.deleteDocCalled)
	}
}

func TestTriggerReindex_ForceWipe(t *testing.T) {
	dir := t.TempDir()
	fakeIDs := []uuid.UUID{uuid.New(), uuid.New(), uuid.New(), uuid.New(), uuid.New()}

	e := echo.New()
	body := `{"workspace":"ws123","force_wipe":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/reindex", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", "ws123")

	mq := &mockReindexQuerier{
		collections:  []sqlc.Collection{{Name: "code", Path: dir}},
		forceWipeIDs: fakeIDs,
	}
	w := newTestWatcherForHandler()

	h := handlers.TriggerReindex(mq, w, nil, nil, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !mq.forceWipeCalled {
		t.Fatal("expected ResetAndReturnChunkIDsByCollection to be called for force-wipe")
	}

	var resp map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if resp["chunks_enqueued"] != float64(5) {
		t.Errorf("expected chunks_enqueued=5, got %v", resp["chunks_enqueued"])
	}
}

func TestTriggerReindexNoRoot(t *testing.T) {
	dir := t.TempDir()

	e := echo.New()
	body := `{"workspace":"ws123"}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/reindex", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set("workspace", "ws123")

	mq := &mockReindexQuerier{
		collections: []sqlc.Collection{
			{Name: "code", Path: dir},
			{Name: "memory", Path: dir},
		},
		indexedDocs: map[string][]sqlc.ListDocumentSourcePathsAndHashesRow{
			"code":   {},
			"memory": {},
		},
	}
	w := newTestWatcherForHandler()

	h := handlers.TriggerReindex(mq, w, nil, nil, zerolog.Nop())
	if err := h(c); err != nil {
		t.Fatalf("handler error: %v", err)
	}
	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rec.Code)
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
