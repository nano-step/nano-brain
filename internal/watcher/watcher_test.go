package watcher

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/graph"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

type mockQuerier struct {
	upsertDocCalls        atomic.Int64
	upsertChunkCalls      atomic.Int64
	deleteChunkCalls      atomic.Int64
	deleteDocCalls        atomic.Int64
	getDocBySourcePathCalls atomic.Int64
	sourcePathHash        map[string]string
}

func newMockQuerier() *mockQuerier {
	return &mockQuerier{sourcePathHash: make(map[string]string)}
}

func (m *mockQuerier) UpsertDocumentBySourcePath(_ context.Context, arg sqlc.UpsertDocumentBySourcePathParams) (sqlc.UpsertDocumentBySourcePathRow, error) {
	m.upsertDocCalls.Add(1)
	m.sourcePathHash[arg.SourcePath] = arg.ContentHash
	return sqlc.UpsertDocumentBySourcePathRow{
		ID:            uuid.New(),
		ContentHash:   arg.ContentHash,
		Collection:    arg.Collection,
		WorkspaceHash: arg.WorkspaceHash,
	}, nil
}

func (m *mockQuerier) DeleteChunksByDocumentID(_ context.Context, _ sqlc.DeleteChunksByDocumentIDParams) error {
	m.deleteChunkCalls.Add(1)
	return nil
}

func (m *mockQuerier) DeleteChunksByIDs(_ context.Context, _ []uuid.UUID) error {
	m.deleteChunkCalls.Add(1)
	return nil
}

func (m *mockQuerier) UpsertChunk(_ context.Context, _ sqlc.UpsertChunkParams) (uuid.UUID, error) {
	m.upsertChunkCalls.Add(1)
	return uuid.New(), nil
}

func (m *mockQuerier) GetDocumentBySourcePath(_ context.Context, arg sqlc.GetDocumentBySourcePathParams) (sqlc.Document, error) {
	m.getDocBySourcePathCalls.Add(1)
	hash, ok := m.sourcePathHash[arg.SourcePath]
	if !ok {
		return sqlc.Document{}, sql.ErrNoRows
	}
	return sqlc.Document{
		ID:          uuid.New(),
		ContentHash: hash,
	}, nil
}

func (m *mockQuerier) DeleteDocumentByIDAndWorkspace(_ context.Context, _ sqlc.DeleteDocumentByIDAndWorkspaceParams) (int64, error) {
	m.deleteDocCalls.Add(1)
	return 1, nil
}

func (m *mockQuerier) InsertChunkEntity(_ context.Context, _ sqlc.InsertChunkEntityParams) error {
	return nil
}

func (m *mockQuerier) ListChunksByDocumentID(_ context.Context, _ sqlc.ListChunksByDocumentIDParams) ([]sqlc.ListChunksByDocumentIDRow, error) {
	return nil, nil
}

func testConfig(debounceMs, pollSec int) config.Config {
	return config.Config{
		Watcher: config.WatcherConfig{
			DebounceMs:      debounceMs,
			ReindexInterval: pollSec,
		},
		Storage: config.StorageConfig{
			MaxFileSize: 1024,
			MaxSize:     10240,
		},
	}
}

func testLogger() zerolog.Logger {
	return zerolog.New(zerolog.NewTestWriter(nil)).Level(zerolog.Disabled)
}

func newTestWatcher(mq *mockQuerier, debounceMs, pollSec int) *Watcher {
	cfg := testConfig(debounceMs, pollSec)
	return New(nil, mq, testLogger(), cfg)
}

func TestDebounce_RapidEventsFireOnce(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping timing-sensitive test in short mode")
	}

	dir := t.TempDir()
	mq := newMockQuerier()
	w := newTestWatcher(mq, 100, 3600)

	fp := filepath.Join(dir, "test.md")
	if err := os.WriteFile(fp, []byte("# Hello\nsome content here"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := w.Watch("testcol", dir, "ws123", "*.md"); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()

	for i := 0; i < 10; i++ {
		if err := os.WriteFile(fp, []byte(fmt.Sprintf("# Hello %d\ncontent", i)), 0o644); err != nil {
			t.Fatal(err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	time.Sleep(300 * time.Millisecond)

	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}

	calls := mq.upsertDocCalls.Load()
	if calls < 1 {
		t.Fatalf("expected at least 1 upsert call, got %d", calls)
	}
	if calls > 3 {
		t.Fatalf("expected debounce to coalesce events, got %d upsert calls", calls)
	}
}

func TestPollInterval_TriggersFullScan(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping timing-sensitive test in short mode")
	}

	dir := t.TempDir()
	fp := filepath.Join(dir, "poll.md")
	if err := os.WriteFile(fp, []byte("# Poll test"), 0o644); err != nil {
		t.Fatal(err)
	}

	mq := newMockQuerier()
	w := newTestWatcher(mq, 50000, 1)

	if err := w.Watch("testcol", dir, "ws123", "*.md"); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()

	time.Sleep(1500 * time.Millisecond)

	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}

	if mq.upsertDocCalls.Load() < 1 {
		t.Fatal("expected poll to trigger at least 1 upsert")
	}
}

func TestProcessFile_SkipsLargeFile(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "big.md")
	if err := os.WriteFile(fp, make([]byte, 2048), 0o644); err != nil {
		t.Fatal(err)
	}

	mq := newMockQuerier()
	w := newTestWatcher(mq, 2000, 300)

	col := watchedCollection{
		name:          "testcol",
		dirPath:       dir,
		workspaceHash: "ws123",
		globPattern:   "*.md",
	}

	w.processFile(context.Background(), col, fp)

	if mq.upsertDocCalls.Load() != 0 {
		t.Fatal("expected large file to be skipped")
	}
}

func TestProcessFile_SkipsSameHash(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "same.md")
	content := []byte("# Same content\nno change")
	if err := os.WriteFile(fp, content, 0o644); err != nil {
		t.Fatal(err)
	}

	mq := newMockQuerier()
	w := newTestWatcher(mq, 2000, 300)

	col := watchedCollection{
		name:          "testcol",
		dirPath:       dir,
		workspaceHash: "ws123",
		globPattern:   "*.md",
	}

	w.processFile(context.Background(), col, fp)
	if mq.upsertDocCalls.Load() != 1 {
		t.Fatalf("expected 1 upsert on first call, got %d", mq.upsertDocCalls.Load())
	}

	w.processFile(context.Background(), col, fp)
	if mq.upsertDocCalls.Load() != 1 {
		t.Fatalf("expected hash skip on second call, got %d total upserts", mq.upsertDocCalls.Load())
	}
}

func TestCleanShutdown(t *testing.T) {
	mq := newMockQuerier()
	w := newTestWatcher(mq, 2000, 3600)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected nil error on clean shutdown, got: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("watcher did not shut down within 2 seconds")
	}
}

func TestWatchUnwatch(t *testing.T) {
	dir := t.TempDir()
	mq := newMockQuerier()
	w := newTestWatcher(mq, 2000, 300)

	if err := w.Watch("col1", dir, "ws1", "*.md"); err != nil {
		t.Fatal(err)
	}

	w.mu.Lock()
	absDir, _ := filepath.Abs(dir)
	if _, ok := w.collections[absDir]; !ok {
		w.mu.Unlock()
		t.Fatal("expected collection to be registered")
	}
	w.mu.Unlock()

	if err := w.Unwatch(dir); err != nil {
		t.Fatal(err)
	}

	w.mu.Lock()
	if _, ok := w.collections[absDir]; ok {
		w.mu.Unlock()
		t.Fatal("expected collection to be removed after Unwatch")
	}
	w.mu.Unlock()
}

func TestTriggerRescanByName(t *testing.T) {
	dir := t.TempDir()
	mq := newMockQuerier()
	w := newTestWatcher(mq, 2000, 300)

	if err := w.Watch("mycol", dir, "ws42", "*.md"); err != nil {
		t.Fatal(err)
	}

	absDir, _ := filepath.Abs(dir)

	found := w.TriggerRescanByName("mycol", "ws42")
	if !found {
		t.Fatal("expected TriggerRescanByName to return true for registered collection")
	}

	w.mu.Lock()
	if !w.dirty[absDir] {
		w.mu.Unlock()
		t.Fatal("expected directory to be marked dirty")
	}
	w.mu.Unlock()

	notFound := w.TriggerRescanByName("other", "ws42")
	if notFound {
		t.Fatal("expected TriggerRescanByName to return false for unregistered collection")
	}

	notFoundWS := w.TriggerRescanByName("mycol", "wrongws")
	if notFoundWS {
		t.Fatal("expected TriggerRescanByName to return false for wrong workspace")
	}
}

func TestProcessFile_SkipsBinaryExtension(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "image.png")
	pngBytes := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d}
	if err := os.WriteFile(fp, pngBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	mq := newMockQuerier()
	w := newTestWatcher(mq, 2000, 300)

	col := watchedCollection{
		name:          "testcol",
		dirPath:       dir,
		workspaceHash: "ws123",
		globPattern:   "*",
	}

	w.processFile(context.Background(), col, fp)

	if mq.upsertDocCalls.Load() != 0 {
		t.Fatalf("expected binary file to be skipped by extension, got %d upserts", mq.upsertDocCalls.Load())
	}
}

func TestProcessFile_SkipsBinaryContentDespiteExtension(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "trap.txt")
	trapBytes := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	if err := os.WriteFile(fp, trapBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	mq := newMockQuerier()
	w := newTestWatcher(mq, 2000, 300)

	col := watchedCollection{
		name:          "testcol",
		dirPath:       dir,
		workspaceHash: "ws123",
		globPattern:   "*.txt",
	}

	w.processFile(context.Background(), col, fp)

	if mq.upsertDocCalls.Load() != 0 {
		t.Fatalf("expected non-UTF8 content to be skipped by safety net, got %d upserts", mq.upsertDocCalls.Load())
	}
}

func TestProcessFile_SkipsNullByteContent(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "data.log")
	if err := os.WriteFile(fp, []byte("entry one\x00entry two"), 0o644); err != nil {
		t.Fatal(err)
	}

	mq := newMockQuerier()
	w := newTestWatcher(mq, 2000, 300)

	col := watchedCollection{
		name:          "testcol",
		dirPath:       dir,
		workspaceHash: "ws123",
		globPattern:   "*.log",
	}

	w.processFile(context.Background(), col, fp)

	if mq.upsertDocCalls.Load() != 0 {
		t.Fatalf("expected null-byte content to be skipped (PG TEXT rejects 0x00), got %d upserts", mq.upsertDocCalls.Load())
	}
}

func TestProcessFile_AcceptsValidUTF8(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "notes.md")
	if err := os.WriteFile(fp, []byte("# Notes\n\nValid UTF-8 content with emoji: Chào thế giới 👋"), 0o644); err != nil {
		t.Fatal(err)
	}

	mq := newMockQuerier()
	w := newTestWatcher(mq, 2000, 300)

	col := watchedCollection{
		name:          "testcol",
		dirPath:       dir,
		workspaceHash: "ws123",
		globPattern:   "*.md",
	}

	w.processFile(context.Background(), col, fp)

	if mq.upsertDocCalls.Load() != 1 {
		t.Fatalf("expected valid UTF-8 markdown to be indexed, got %d upserts", mq.upsertDocCalls.Load())
	}
}

func TestHotRegister_WatchAfterRunScansExistingFiles(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping timing-sensitive test in short mode")
	}

	// Start the watcher with no collections, then "hot-register" a new
	// workspace by calling Watch after Run is already going. The pre-existing
	// files in that directory MUST be picked up via the dirty-mark mechanism
	// (issue #308 regression guard).

	mq := newMockQuerier()
	w := newTestWatcher(mq, 100, 3600)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()

	time.Sleep(50 * time.Millisecond)

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.md"), []byte("# A\nbody"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.md"), []byte("# B\nbody"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := w.Watch("hotreg", dir, "ws-hotreg", "*.md"); err != nil {
		t.Fatal(err)
	}

	time.Sleep(500 * time.Millisecond)

	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}

	calls := mq.upsertDocCalls.Load()
	if calls < 2 {
		t.Fatalf("expected at least 2 upserts for pre-existing files after hot-register, got %d", calls)
	}
}

func TestProcessFile_SkipsUnchangedMtime(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "unchanged.md")
	content := []byte("# Unchanged\ncontent here")
	if err := os.WriteFile(fp, content, 0o644); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(fp)
	if err != nil {
		t.Fatal(err)
	}

	mq := newMockQuerier()
	w := newTestWatcher(mq, 2000, 300)

	sum := sha256.Sum256(content)
	contentHash := hex.EncodeToString(sum[:])

	w.fileCacheMu.Lock()
	w.fileCache[fp] = fileState{
		ModTime: info.ModTime(),
		Size:    info.Size(),
		Hash:    contentHash,
	}
	w.fileCacheMu.Unlock()

	col := watchedCollection{
		name:          "testcol",
		dirPath:       dir,
		workspaceHash: "ws123",
		globPattern:   "*.md",
	}

	w.processFile(context.Background(), col, fp)

	if mq.upsertDocCalls.Load() != 0 {
		t.Fatalf("expected mtime cache to skip processing, got %d upserts", mq.upsertDocCalls.Load())
	}
}

func TestHandleFSEvent_SkipsGitEvents(t *testing.T) {
	mq := newMockQuerier()
	w := newTestWatcher(mq, 2000, 300)

	dir := t.TempDir()
	if err := w.Watch("testcol", dir, "ws123", "*.md"); err != nil {
		t.Fatal(err)
	}

	debounce := time.NewTimer(time.Duration(w.debounceMs) * time.Millisecond)
	debounce.Stop()

	gitEvent := fsnotify.Event{
		Name: filepath.Join(dir, ".git"),
		Op:   fsnotify.Write,
	}

	w.handleFSEvent(gitEvent, debounce)

	w.mu.Lock()
	isDirty := w.dirty[dir]
	w.mu.Unlock()

	if isDirty {
		t.Fatal("expected .git/ event to be filtered, but directory was marked dirty")
	}
}

func TestPollTicker_SkipsWhenNoEvents(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping timing-sensitive test in short mode")
	}

	dir := t.TempDir()
	fp := filepath.Join(dir, "test.md")
	if err := os.WriteFile(fp, []byte("# Test"), 0o644); err != nil {
		t.Fatal(err)
	}

	mq := newMockQuerier()
	w := newTestWatcher(mq, 50000, 1)

	if err := w.Watch("testcol", dir, "ws123", "*.md"); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()

	time.Sleep(200 * time.Millisecond)

	callsAfterInitialScan := mq.upsertDocCalls.Load()

	w.hasNewEvents.Store(false)

	time.Sleep(1500 * time.Millisecond)

	cancel()
	<-done

	callsAfterPoll := mq.upsertDocCalls.Load()

	if callsAfterPoll > callsAfterInitialScan {
		t.Fatalf("expected poll to skip when hasNewEvents=false, but got %d new upserts after initial scan", callsAfterPoll-callsAfterInitialScan)
	}
}

func TestBuildEdgeMetadata_WithMetadata(t *testing.T) {
	e := graph.Edge{
		SourceNode: "handler",
		TargetNode: "/api/v1/items",
		Kind:       graph.EdgeHTTP,
		SourceFile: "api.go",
		Line:       42,
		Language:   "go",
		Metadata:   map[string]any{"method": "POST", "path": "/x"},
	}

	data, err := buildEdgeMetadata(e)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if got["method"] != "POST" {
		t.Errorf("expected method=POST, got %v", got["method"])
	}
	if got["path"] != "/x" {
		t.Errorf("expected path=/x, got %v", got["path"])
	}
	if got["line"] == nil {
		t.Error("expected line field to be present")
	}
	if got["language"] != "go" {
		t.Errorf("expected language=go, got %v", got["language"])
	}
}

func TestBuildEdgeMetadata_NilMetadata(t *testing.T) {
	e := graph.Edge{
		SourceNode: "A",
		TargetNode: "B",
		Kind:       graph.EdgeCalls,
		SourceFile: "main.go",
		Line:       10,
		Language:   "go",
		Metadata:   nil,
	}

	data, err := buildEdgeMetadata(e)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(got) != 2 {
		t.Errorf("expected exactly 2 keys (line, language), got %d: %v", len(got), got)
	}
	if got["line"] == nil {
		t.Error("expected line field")
	}
	if got["language"] != "go" {
		t.Errorf("expected language=go, got %v", got["language"])
	}
}

func TestBuildEdgeMetadata_DoesNotMutateInput(t *testing.T) {
	original := map[string]any{"method": "GET"}
	e := graph.Edge{
		Line:     5,
		Language: "go",
		Metadata: original,
	}

	_, err := buildEdgeMetadata(e)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(original) != 1 {
		t.Errorf("buildEdgeMetadata must not mutate e.Metadata, but it grew to %d keys", len(original))
	}
	if _, ok := original["line"]; ok {
		t.Error("buildEdgeMetadata must not inject 'line' into e.Metadata")
	}
}

func TestCleanupDeletedDocument_DeletesDocAndChunks(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	mq := newMockQuerier()
	w := newTestWatcher(mq, 50, 60)

	dir := t.TempDir()
	workspaceHash := "test-ws-hash"
	filePath := filepath.Join(dir, "main.go")

	mq.sourcePathHash[filePath] = "abc123"

	err := w.Watch("code", dir, workspaceHash, "**/*.go")
	if err != nil {
		t.Fatalf("watch: %v", err)
	}

	w.cleanupDeletedDocument(filePath)

	if mq.getDocBySourcePathCalls.Load() != 1 {
		t.Errorf("expected 1 GetDocumentBySourcePath call, got %d", mq.getDocBySourcePathCalls.Load())
	}
	if mq.deleteChunkCalls.Load() != 1 {
		t.Errorf("expected 1 DeleteChunksByDocumentID call, got %d", mq.deleteChunkCalls.Load())
	}
	if mq.deleteDocCalls.Load() != 1 {
		t.Errorf("expected 1 DeleteDocumentByIDAndWorkspace call, got %d", mq.deleteDocCalls.Load())
	}
}

func TestCleanupDeletedDocument_NoOpWhenDocNotFound(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	mq := newMockQuerier()
	w := newTestWatcher(mq, 50, 60)

	dir := t.TempDir()
	workspaceHash := "test-ws-hash"
	filePath := filepath.Join(dir, "nonexistent.go")

	err := w.Watch("code", dir, workspaceHash, "**/*.go")
	if err != nil {
		t.Fatalf("watch: %v", err)
	}

	w.cleanupDeletedDocument(filePath)

	if mq.getDocBySourcePathCalls.Load() != 1 {
		t.Errorf("expected 1 GetDocumentBySourcePath call, got %d", mq.getDocBySourcePathCalls.Load())
	}
	if mq.deleteChunkCalls.Load() != 0 {
		t.Errorf("expected 0 DeleteChunksByDocumentID calls, got %d", mq.deleteChunkCalls.Load())
	}
	if mq.deleteDocCalls.Load() != 0 {
		t.Errorf("expected 0 DeleteDocumentByIDAndWorkspace calls, got %d", mq.deleteDocCalls.Load())
	}
}

func TestCleanupDeletedDocument_NoOpWhenNoMatchingCollection(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	mq := newMockQuerier()
	w := newTestWatcher(mq, 50, 60)

	w.cleanupDeletedDocument("/some/random/path.go")

	if mq.getDocBySourcePathCalls.Load() != 0 {
		t.Errorf("expected 0 GetDocumentBySourcePath calls, got %d", mq.getDocBySourcePathCalls.Load())
	}
}

// TestScanCollection_WatchesSubdirectories verifies that scanning a collection
// registers an fsnotify watch on every (non-skipped) directory in the tree, not
// just the root. fsnotify is non-recursive, so without per-dir watches edits in
// subdirectories never fire events (issue #497).
func TestScanCollection_WatchesSubdirectories(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "internal", "embed")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "x.md"), []byte("# x"), 0o644); err != nil {
		t.Fatal(err)
	}

	mq := newMockQuerier()
	w := newTestWatcher(mq, 2000, 300)

	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	defer fsw.Close()
	w.fsw = fsw

	col := watchedCollection{name: "c", dirPath: root, workspaceHash: "ws", globPattern: "*.md"}
	w.scanCollection(context.Background(), col)

	w.mu.Lock()
	defer w.mu.Unlock()
	for _, want := range []string{root, filepath.Join(root, "internal"), sub} {
		if !w.watchedDirs[want] {
			t.Errorf("expected dir to be watched: %s (watchedDirs=%v)", want, w.watchedDirs)
		}
	}
}

// TestSubdirEdit_IndexedWithoutRootActivity is the end-to-end regression for
// issue #497: a file created in a nested subdirectory must be indexed off its
// own fsnotify event, with no activity at the workspace root.
func TestSubdirEdit_IndexedWithoutRootActivity(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping timing-sensitive test in short mode")
	}

	root := t.TempDir()
	sub := filepath.Join(root, "nested", "deep")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	mq := newMockQuerier()
	w := newTestWatcher(mq, 100, 3600)

	if err := w.Watch("c", root, "ws", "*.md"); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()

	// Let the initial scan run and register the recursive subdir watch.
	time.Sleep(300 * time.Millisecond)
	baseline := mq.upsertDocCalls.Load()

	// Write a new file ONLY in the deep subdir — no root-level activity.
	if err := os.WriteFile(filepath.Join(sub, "new.md"), []byte("# new\nbody text"), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(400 * time.Millisecond) // > debounce (100ms)

	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}

	if got := mq.upsertDocCalls.Load(); got <= baseline {
		t.Fatalf("expected subdir edit to be indexed (upsert calls > %d), got %d", baseline, got)
	}
}

// mockGraphQuerier implements GraphQuerier for tests, counting calls to each method.
type mockGraphQuerier struct {
	upsertEdgeCalls atomic.Int64
	deleteEdgeCalls atomic.Int64
}

func (m *mockGraphQuerier) UpsertGraphEdge(_ context.Context, _ sqlc.UpsertGraphEdgeParams) error {
	m.upsertEdgeCalls.Add(1)
	return nil
}

func (m *mockGraphQuerier) DeleteGraphEdgesByFile(_ context.Context, _ sqlc.DeleteGraphEdgesByFileParams) error {
	m.deleteEdgeCalls.Add(1)
	return nil
}

// TestProcessFile_ContentHashSkipsEdgeExtraction verifies D-06a: when processFile
// is called with byte-identical content already stored in the DB (mtime cache
// cleared so the fast-path does not fire), the content-hash dedup check must
// short-circuit BEFORE extractAndUpsertEdges is reached.
//
// Setup: pre-seed the mock querier with the file's hash (simulating a file that
// was already fully indexed on a prior run). The mtime cache is intentionally
// empty. With db==nil, reaching extractAndUpsertEdges would cause a nil-pointer
// panic at w.db.BeginTx — so a panic is the RED signal; a clean return with
// upsertDocCalls==0 (no re-index) is GREEN.
func TestProcessFile_ContentHashSkipsEdgeExtraction(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "edge_skip.go")
	content := []byte("package main\n\nfunc Foo() {}\n")
	if err := os.WriteFile(fp, content, 0o644); err != nil {
		t.Fatal(err)
	}

	sum := sha256.Sum256(content)
	wantHash := hex.EncodeToString(sum[:])

	mq := newMockQuerier()
	// Pre-seed: file was already indexed in a prior run; hash is stored in DB.
	mq.sourcePathHash[fp] = wantHash

	w := newTestWatcher(mq, 2000, 300)

	mgq := &mockGraphQuerier{}
	w.WithGraphRegistry(graph.NewRegistry(), mgq)

	col := watchedCollection{
		name:          "testcol",
		dirPath:       dir,
		workspaceHash: "ws-edge-skip",
		globPattern:   "*.go",
	}

	// mtime cache is empty (simulates a daemon restart where in-memory cache was lost).
	// processFile will read+hash the file, find the hash matches the DB record, and
	// must return WITHOUT calling extractAndUpsertEdges.
	// GREEN: dedup fires first -> no panic, upsertDocCalls stays 0.
	// RED (pre-reorder): extractAndUpsertEdges reached first -> w.db.BeginTx panics (db==nil).
	w.processFile(context.Background(), col, fp)

	if got := mq.upsertDocCalls.Load(); got != 0 {
		t.Fatalf("expected upsertDocCalls=0 (hash already stored), got %d", got)
	}

	// Cache entry must be repopulated so the next scan hits the mtime fast-path.
	w.fileCacheMu.RLock()
	entry, ok := w.fileCache[fp]
	w.fileCacheMu.RUnlock()
	if !ok {
		t.Fatal("expected fileCache entry to be repopulated after content-hash dedup hit")
	}
	if entry.Hash != wantHash {
		t.Fatalf("expected cached hash %s, got %s", wantHash, entry.Hash)
	}
}

// TestUnwatchTree_ClearsNestedAndAllowsRewatch covers the #497 review finding:
// removing a directory must clear its nested watch entries (not just the exact
// path), otherwise watchDir would skip re-watching a recreated subtree.
func TestUnwatchTree_ClearsNestedAndAllowsRewatch(t *testing.T) {
	root := t.TempDir()
	a := filepath.Join(root, "a")
	b := filepath.Join(a, "b")
	if err := os.MkdirAll(b, 0o755); err != nil {
		t.Fatal(err)
	}

	mq := newMockQuerier()
	w := newTestWatcher(mq, 2000, 300)
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	defer fsw.Close()
	w.fsw = fsw

	col := watchedCollection{name: "c", dirPath: root, workspaceHash: "ws", globPattern: "*.md"}
	w.scanCollection(context.Background(), col)

	w.mu.Lock()
	for _, p := range []string{root, a, b} {
		if !w.watchedDirs[p] {
			w.mu.Unlock()
			t.Fatalf("precondition: %s should be watched (watchedDirs=%v)", p, w.watchedDirs)
		}
	}
	// Remove the subtree rooted at a/.
	w.unwatchTreeLocked(a)
	if w.watchedDirs[a] || w.watchedDirs[b] {
		w.mu.Unlock()
		t.Fatalf("nested watch entries not cleared on remove: %v", w.watchedDirs)
	}
	if !w.watchedDirs[root] {
		w.mu.Unlock()
		t.Fatal("root watch should survive removal of a nested subtree")
	}
	w.mu.Unlock()

	// A recreated subdirectory must be watchable again (no stale entry blocking it).
	w.watchDir(b)
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.watchedDirs[b] {
		t.Fatal("recreated subdirectory was not re-watched (stale watchedDirs entry)")
	}
}
