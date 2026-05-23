package watcher

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

type mockQuerier struct {
	upsertDocCalls   atomic.Int64
	upsertChunkCalls atomic.Int64
	deleteChunkCalls atomic.Int64
	// sourcePathHash is not synchronized — safe only in sequential test paths.
	// Add sync.Mutex if tests evolve to exercise concurrent watcher+mock.
	sourcePathHash map[string]string
}

func newMockQuerier() *mockQuerier {
	return &mockQuerier{sourcePathHash: make(map[string]string)}
}

func (m *mockQuerier) UpsertDocument(_ context.Context, arg sqlc.UpsertDocumentParams) (sqlc.UpsertDocumentRow, error) {
	m.upsertDocCalls.Add(1)
	m.sourcePathHash[arg.SourcePath] = arg.ContentHash
	return sqlc.UpsertDocumentRow{
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

func (m *mockQuerier) UpsertChunk(_ context.Context, _ sqlc.UpsertChunkParams) (uuid.UUID, error) {
	m.upsertChunkCalls.Add(1)
	return uuid.New(), nil
}

func (m *mockQuerier) GetDocumentBySourcePath(_ context.Context, arg sqlc.GetDocumentBySourcePathParams) (sqlc.GetDocumentBySourcePathRow, error) {
	hash, ok := m.sourcePathHash[arg.SourcePath]
	if !ok {
		return sqlc.GetDocumentBySourcePathRow{}, sql.ErrNoRows
	}
	return sqlc.GetDocumentBySourcePathRow{
		ID:          uuid.New(),
		ContentHash: hash,
	}, nil
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
