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

	"encoding/json"

	"github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
	"github.com/nano-brain/nano-brain/internal/chunk"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/graph"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/symbol"
	"github.com/rs/zerolog"
	"github.com/sqlc-dev/pqtype"
)

type GraphQuerier interface {
	UpsertGraphEdge(ctx context.Context, arg sqlc.UpsertGraphEdgeParams) error
	DeleteGraphEdgesByFile(ctx context.Context, arg sqlc.DeleteGraphEdgesByFileParams) error
}

type WatcherQuerier interface {
	UpsertDocumentBySourcePath(ctx context.Context, arg sqlc.UpsertDocumentBySourcePathParams) (sqlc.UpsertDocumentBySourcePathRow, error)
	DeleteChunksByDocumentID(ctx context.Context, arg sqlc.DeleteChunksByDocumentIDParams) error
	UpsertChunk(ctx context.Context, arg sqlc.UpsertChunkParams) (uuid.UUID, error)
	GetDocumentBySourcePath(ctx context.Context, arg sqlc.GetDocumentBySourcePathParams) (sqlc.Document, error)
}

type watchedCollection struct {
	name              string
	dirPath           string
	workspaceHash     string
	globPattern       string
	excludePatterns   []string
	allowedExtensions []string
	filter            *fileFilter
}

type Watcher struct {
	db             *sql.DB
	queries        WatcherQuerier
	graphQuerier   GraphQuerier
	logger         zerolog.Logger
	debounceMs     int
	pollInterval   int
	maxFileSize    int64
	symbolRegistry *symbol.Registry
	graphRegistry  *graph.Registry

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

func (w *Watcher) WithSymbolRegistry(r *symbol.Registry) *Watcher {
	w.symbolRegistry = r
	return w
}

func (w *Watcher) WithGraphRegistry(r *graph.Registry, gq GraphQuerier) *Watcher {
	w.graphRegistry = r
	w.graphQuerier = gq
	return w
}

func (w *Watcher) Watch(collectionName, dirPath, workspaceHash, globPattern string) error {
	return w.WatchWithFilter(collectionName, dirPath, workspaceHash, globPattern, nil, nil)
}

func (w *Watcher) WatchWithFilter(collectionName, dirPath, workspaceHash, globPattern string, excludePatterns, allowedExtensions []string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		return fmt.Errorf("resolve path %s: %w", dirPath, err)
	}

	w.collections[absPath] = watchedCollection{
		name:              collectionName,
		dirPath:           absPath,
		workspaceHash:     workspaceHash,
		globPattern:       globPattern,
		excludePatterns:   excludePatterns,
		allowedExtensions: allowedExtensions,
		filter:            newFileFilter(absPath, excludePatterns, allowedExtensions),
	}

	if w.fsw != nil {
		if _, err := os.Stat(absPath); err != nil {
			w.logger.Info().Str("dir", absPath).Str("collection", collectionName).Msg("collection path not found, skipping watch")
			return nil
		}
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

// TriggerRescanByName marks the directory of a named collection dirty so the
// watcher will re-scan it on the next debounce tick. Returns true if the
// collection was found, false if it is not registered with this watcher.
func (w *Watcher) TriggerRescanByName(collectionName, workspaceHash string) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	for path, col := range w.collections {
		if col.name == collectionName && col.workspaceHash == workspaceHash {
			w.dirty[path] = true
			return true
		}
	}
	return false
}

func (w *Watcher) Run(ctx context.Context) error {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create fsnotify watcher: %w", err)
	}
	defer func() {
		fsw.Close()
		w.mu.Lock()
		w.fsw = nil
		w.mu.Unlock()
	}()

	w.mu.Lock()
	w.fsw = fsw
	for absPath, col := range w.collections {
		if _, err := os.Stat(absPath); err != nil {
			w.logger.Info().Str("dir", absPath).Str("collection", col.name).Msg("collection path not found, skipping watch")
			continue
		}
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

	w.processAll(ctx)

	for {
		select {
		case <-ctx.Done():
			debounce.Stop()
			w.logger.Info().Msg("file watcher stopping")
			return nil

		case event, ok := <-fsw.Events:
			if !ok {
				debounce.Stop()
				return nil
			}
			w.handleFSEvent(event, debounce)

		case err, ok := <-fsw.Errors:
			if !ok {
				debounce.Stop()
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
	if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename) == 0 {
		return
	}

	dir := filepath.Dir(event.Name)

	w.mu.Lock()
	if _, ok := w.collections[dir]; ok {
		w.dirty[dir] = true
	}
	w.mu.Unlock()

	if event.Op&fsnotify.Remove != 0 {
		// TODO(story-2.x): Implement stale document cleanup. When a file is deleted,
		// its document+chunks remain in the DB. Consider diffing globbed files against
		// DB documents in scanCollection to purge orphans.
		w.logger.Info().Str("file", event.Name).Msg("file removed (deletion not handled, skipping)")
	}

	if !debounce.Stop() {
		select {
		case <-debounce.C:
		default:
		}
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
		if col.filter != nil && col.filter.shouldSkip(filePath) {
			w.logger.Debug().Str("file", filePath).Msg("skipping filtered file")
			continue
		}
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

	if info.IsDir() {
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

	params := sqlc.UpsertDocumentBySourcePathParams{
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

	if w.symbolRegistry != nil {
		w.extractAndUpsertSymbols(ctx, col, filePath, content)
	}
	if w.graphRegistry != nil {
		w.extractAndUpsertEdges(ctx, col, filePath, content)
	}
}

func (w *Watcher) extractAndUpsertSymbols(ctx context.Context, col watchedCollection, filePath string, content []byte) {
	syms, err := w.symbolRegistry.Extract(filePath, content)
	if err != nil {
		w.logger.Warn().Err(err).Str("file", filePath).Msg("symbol extraction failed")
		return
	}
	if len(syms) == 0 {
		return
	}

	for _, s := range syms {
		metaBytes, _ := json.Marshal(map[string]string{
			"source_type": "symbol",
			"kind":        string(s.Kind),
			"language":    s.Language,
			"signature":   s.Signature,
		})
		sourcePath := filePath + "?symbol=" + s.Name + "&kind=" + string(s.Kind)
		sum := sha256.Sum256([]byte(sourcePath + s.Signature))
		params := sqlc.UpsertDocumentBySourcePathParams{
			WorkspaceHash: col.workspaceHash,
			ContentHash:   hex.EncodeToString(sum[:]),
			Title:         s.Name,
			Content:       s.Signature,
			SourcePath:    sourcePath,
			Collection:    col.name,
			Tags:          []string{"symbol", s.Language, string(s.Kind)},
			Metadata:      pqtype.NullRawMessage{RawMessage: metaBytes, Valid: true},
		}
		if _, err := w.queries.UpsertDocumentBySourcePath(ctx, params); err != nil {
			w.logger.Warn().Err(err).Str("symbol", s.Name).Msg("symbol upsert failed")
		}
	}

	w.logger.Info().
		Str("file", filePath).
		Int("symbols", len(syms)).
		Msg("symbols extracted")
}

func (w *Watcher) extractAndUpsertEdges(ctx context.Context, col watchedCollection, filePath string, content []byte) {
	edges, err := w.graphRegistry.ExtractEdges(filePath, content)
	if err != nil {
		w.logger.Warn().Err(err).Str("file", filePath).Msg("graph edge extraction failed")
		return
	}

	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		w.logger.Warn().Err(err).Str("file", filePath).Msg("graph tx begin failed")
		return
	}
	defer tx.Rollback() //nolint:errcheck

	tq := sqlc.New(tx)
	if err := tq.DeleteGraphEdgesByFile(ctx, sqlc.DeleteGraphEdgesByFileParams{
		WorkspaceHash: col.workspaceHash,
		SourceFile:    filePath,
	}); err != nil {
		w.logger.Warn().Err(err).Str("file", filePath).Msg("graph edge delete failed")
		return
	}

	for _, e := range edges {
		meta, _ := json.Marshal(map[string]any{"line": e.Line, "language": e.Language})
		if err := tq.UpsertGraphEdge(ctx, sqlc.UpsertGraphEdgeParams{
			WorkspaceHash: col.workspaceHash,
			SourceNode:    e.SourceNode,
			TargetNode:    e.TargetNode,
			EdgeType:      string(e.Kind),
			SourceFile:    e.SourceFile,
			Metadata:      meta,
		}); err != nil {
			w.logger.Warn().Err(err).Str("edge", e.SourceNode+"->"+e.TargetNode).Msg("graph edge upsert failed")
		}
	}

	if err := tx.Commit(); err != nil {
		w.logger.Warn().Err(err).Str("file", filePath).Msg("graph tx commit failed")
		return
	}

	w.logger.Info().
		Str("file", filePath).
		Int("edges", len(edges)).
		Msg("graph edges extracted")
}

func (w *Watcher) upsertWithTx(ctx context.Context, workspace, filePath string, params sqlc.UpsertDocumentBySourcePathParams, chunks []chunk.Chunk, meta pqtype.NullRawMessage) error {
	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // Rollback after Commit is a no-op in database/sql.

	tq := sqlc.New(tx)
	docRow, err := tq.UpsertDocumentBySourcePath(ctx, params)
	if err != nil {
		return fmt.Errorf("upsert document: %w", err)
	}

	if err := w.writeChunks(ctx, tq, docRow.ID, workspace, chunks, meta); err != nil {
		return fmt.Errorf("write chunks: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

func (w *Watcher) upsertWithoutTx(ctx context.Context, workspace string, params sqlc.UpsertDocumentBySourcePathParams, chunks []chunk.Chunk, meta pqtype.NullRawMessage) error {
	docRow, err := w.queries.UpsertDocumentBySourcePath(ctx, params)
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
