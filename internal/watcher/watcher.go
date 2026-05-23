package watcher

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
	"github.com/nano-brain/nano-brain/internal/chunk"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
	"github.com/sqlc-dev/pqtype"
)

type WatcherQuerier interface {
	UpsertDocument(ctx context.Context, arg sqlc.UpsertDocumentParams) (sqlc.UpsertDocumentRow, error)
	DeleteChunksByDocumentID(ctx context.Context, arg sqlc.DeleteChunksByDocumentIDParams) error
	UpsertChunk(ctx context.Context, arg sqlc.UpsertChunkParams) (uuid.UUID, error)
	GetDocumentBySourcePath(ctx context.Context, arg sqlc.GetDocumentBySourcePathParams) (sqlc.GetDocumentBySourcePathRow, error)
}

type watchedCollection struct {
	name          string
	dirPath       string
	workspaceHash string
	globPattern   string
}

type Watcher struct {
	db           *sql.DB
	queries      WatcherQuerier
	logger       zerolog.Logger
	debounceMs   int
	pollInterval int
	maxFileSize  int64

	fsw         *fsnotify.Watcher
	mu          sync.Mutex
	collections map[string]watchedCollection
	dirty       map[string]bool
}

func New(db *sql.DB, queries WatcherQuerier, logger zerolog.Logger, cfg config.Config) *Watcher {
	return &Watcher{
		db:           db,
		queries:      queries,
		logger:       logger.With().Str("component", "watcher").Logger(),
		debounceMs:   cfg.Watcher.DebounceMs,
		pollInterval: cfg.Watcher.ReindexInterval,
		maxFileSize:  cfg.Storage.MaxFileSize,
		collections:  make(map[string]watchedCollection),
		dirty:        make(map[string]bool),
	}
}

func (w *Watcher) Watch(collectionName, dirPath, workspaceHash, globPattern string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		return fmt.Errorf("resolve path %s: %w", dirPath, err)
	}

	w.collections[absPath] = watchedCollection{
		name:          collectionName,
		dirPath:       absPath,
		workspaceHash: workspaceHash,
		globPattern:   globPattern,
	}

	if w.fsw != nil {
		if err := w.fsw.Add(absPath); err != nil {
			return fmt.Errorf("watch dir %s: %w", absPath, err)
		}
		w.logger.Info().Str("dir", absPath).Str("collection", collectionName).Msg("watching directory")
	}
	return nil
}

func (w *Watcher) Unwatch(dirPath string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		return fmt.Errorf("resolve path %s: %w", dirPath, err)
	}

	delete(w.collections, absPath)
	delete(w.dirty, absPath)

	if w.fsw != nil {
		_ = w.fsw.Remove(absPath)
	}
	return nil
}

func (w *Watcher) Run(ctx context.Context) error {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create fsnotify watcher: %w", err)
	}
	defer fsw.Close()

	w.mu.Lock()
	w.fsw = fsw
	for absPath, col := range w.collections {
		if err := fsw.Add(absPath); err != nil {
			w.logger.Warn().Err(err).Str("dir", absPath).Msg("failed to add watch")
			continue
		}
		w.logger.Info().Str("dir", absPath).Str("collection", col.name).Msg("watching directory")
	}
	w.mu.Unlock()

	debounce := time.NewTimer(time.Duration(w.debounceMs) * time.Millisecond)
	debounce.Stop()

	pollTicker := time.NewTicker(time.Duration(w.pollInterval) * time.Second)
	defer pollTicker.Stop()

	w.logger.Info().
		Int("debounce_ms", w.debounceMs).
		Int("poll_interval_s", w.pollInterval).
		Msg("file watcher started")

	for {
		select {
		case <-ctx.Done():
			debounce.Stop()
			w.logger.Info().Msg("file watcher stopping")
			return nil

		case event, ok := <-fsw.Events:
			if !ok {
				return nil
			}
			w.handleFSEvent(event, debounce)

		case err, ok := <-fsw.Errors:
			if !ok {
				return nil
			}
			w.logger.Error().Err(err).Msg("fsnotify error")

		case <-debounce.C:
			w.processDirty(ctx)

		case <-pollTicker.C:
			w.processAll(ctx)
		}
	}
}

func (w *Watcher) handleFSEvent(event fsnotify.Event, debounce *time.Timer) {
	if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove) == 0 {
		return
	}

	dir := filepath.Dir(event.Name)

	w.mu.Lock()
	if _, ok := w.collections[dir]; ok {
		w.dirty[dir] = true
	}
	w.mu.Unlock()

	if event.Op&fsnotify.Remove != 0 {
		w.logger.Info().Str("file", event.Name).Msg("file removed (deletion not handled, skipping)")
	}

	debounce.Reset(time.Duration(w.debounceMs) * time.Millisecond)
}

func (w *Watcher) processDirty(ctx context.Context) {
	w.mu.Lock()
	dirs := make([]string, 0, len(w.dirty))
	for d := range w.dirty {
		dirs = append(dirs, d)
	}
	w.dirty = make(map[string]bool)
	w.mu.Unlock()

	for _, d := range dirs {
		w.mu.Lock()
		col, ok := w.collections[d]
		w.mu.Unlock()
		if !ok {
			continue
		}
		w.scanCollection(ctx, col)
	}
}

func (w *Watcher) processAll(ctx context.Context) {
	w.mu.Lock()
	cols := make([]watchedCollection, 0, len(w.collections))
	for _, col := range w.collections {
		cols = append(cols, col)
	}
	w.mu.Unlock()

	for _, col := range cols {
		w.scanCollection(ctx, col)
	}
}

func (w *Watcher) scanCollection(ctx context.Context, col watchedCollection) {
	pattern := filepath.Join(col.dirPath, col.globPattern)
	matches, err := filepath.Glob(pattern)
	if err != nil {
		w.logger.Error().Err(err).Str("pattern", pattern).Msg("glob failed")
		return
	}

	for _, filePath := range matches {
		if ctx.Err() != nil {
			return
		}
		w.processFile(ctx, col, filePath)
	}
}

func (w *Watcher) processFile(ctx context.Context, col watchedCollection, filePath string) {
	info, err := os.Stat(filePath)
	if err != nil {
		w.logger.Warn().Err(err).Str("file", filePath).Msg("stat failed, skipping")
		return
	}

	if info.Size() > w.maxFileSize {
		w.logger.Warn().
			Str("file", filePath).
			Int64("size", info.Size()).
			Int64("max", w.maxFileSize).
			Msg("file exceeds max size, skipping")
		return
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		w.logger.Warn().Err(err).Str("file", filePath).Msg("read failed, skipping")
		return
	}

	sum := sha256.Sum256(content)
	contentHash := hex.EncodeToString(sum[:])

	existing, err := w.queries.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
		SourcePath:    filePath,
		WorkspaceHash: col.workspaceHash,
	})
	if err == nil && existing.ContentHash == contentHash {
		return
	}

	chunks := chunk.Split(string(content), chunk.DefaultConfig())
	title := filepath.Base(filePath)
	meta := pqtype.NullRawMessage{RawMessage: []byte(`{}`), Valid: true}

	params := sqlc.UpsertDocumentParams{
		WorkspaceHash: col.workspaceHash,
		ContentHash:   contentHash,
		Title:         title,
		Content:       string(content),
		SourcePath:    filePath,
		Collection:    col.name,
		Tags:          []string{},
		Metadata:      meta,
	}

	if w.db != nil {
		if err := w.upsertWithTx(ctx, col.workspaceHash, filePath, params, chunks, meta); err != nil {
			w.logger.Error().Err(err).Str("file", filePath).Msg("index failed")
			return
		}
	} else {
		if err := w.upsertWithoutTx(ctx, col.workspaceHash, params, chunks, meta); err != nil {
			w.logger.Error().Err(err).Str("file", filePath).Msg("index failed")
			return
		}
	}

	w.logger.Info().
		Str("file", filePath).
		Str("collection", col.name).
		Int("chunks", len(chunks)).
		Msg("indexed file")
}

func (w *Watcher) upsertWithTx(ctx context.Context, workspace, filePath string, params sqlc.UpsertDocumentParams, chunks []chunk.Chunk, meta pqtype.NullRawMessage) error {
	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	tq := sqlc.New(tx)
	docRow, err := tq.UpsertDocument(ctx, params)
	if err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("upsert document: %w", err)
	}

	if err := w.writeChunks(ctx, tq, docRow.ID, workspace, chunks, meta); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("write chunks: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func (w *Watcher) upsertWithoutTx(ctx context.Context, workspace string, params sqlc.UpsertDocumentParams, chunks []chunk.Chunk, meta pqtype.NullRawMessage) error {
	docRow, err := w.queries.UpsertDocument(ctx, params)
	if err != nil {
		return fmt.Errorf("upsert document: %w", err)
	}

	if err := w.writeChunks(ctx, w.queries, docRow.ID, workspace, chunks, meta); err != nil {
		return fmt.Errorf("write chunks: %w", err)
	}
	return nil
}

func (w *Watcher) writeChunks(ctx context.Context, q WatcherQuerier, docID uuid.UUID, workspace string, chunks []chunk.Chunk, meta pqtype.NullRawMessage) error {
	if err := q.DeleteChunksByDocumentID(ctx, sqlc.DeleteChunksByDocumentIDParams{
		DocumentID:    docID,
		WorkspaceHash: workspace,
	}); err != nil {
		return err
	}
	for _, ch := range chunks {
		if _, err := q.UpsertChunk(ctx, sqlc.UpsertChunkParams{
			DocumentID:    docID,
			WorkspaceHash: workspace,
			ContentHash:   ch.Hash,
			Content:       ch.Content,
			ChunkIndex:    int32(ch.Sequence),
			StartLine:     sql.NullInt32{Int32: int32(ch.StartLine), Valid: true},
			EndLine:       sql.NullInt32{Int32: int32(ch.EndLine), Valid: true},
			Metadata:      meta,
		}); err != nil {
			return err
		}
	}
	return nil
}
