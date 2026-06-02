package watcher

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"time"

	"encoding/json"

	"github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
	gitignore "github.com/sabhiram/go-gitignore"
	"github.com/nano-brain/nano-brain/internal/chunk"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/embed"
	"github.com/nano-brain/nano-brain/internal/eventbus"
	"github.com/nano-brain/nano-brain/internal/graph"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/symbol"
	"github.com/rs/zerolog"
	"github.com/sqlc-dev/pqtype"
	"golang.org/x/time/rate"
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
	embedQueue     *embed.Queue

	fsw         *fsnotify.Watcher
	mu          sync.Mutex
	collections map[string]watchedCollection
	dirty       map[string]bool
	// hotRegisterCh signals Run() that a workspace was registered at runtime so
	// the dirty map should be processed without waiting for fsnotify events.
	// Buffered=1 so WatchWithFilter never blocks. See issue #308.
	hotRegisterCh chan struct{}
	pub           eventbus.Publisher
	rateLimiters  map[string]*rate.Limiter

	globalIgnore *gitignore.GitIgnore
}

// SetGlobalIgnore configures the watcher to apply a global gitignore-style
// matcher to every collection's file filter. Pass nil to disable. Must be
// called BEFORE any WatchWithFilter — existing filters are not updated.
// See issue #263.
func (w *Watcher) SetGlobalIgnore(gi *gitignore.GitIgnore) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.globalIgnore = gi
}

func (w *Watcher) CollectionsWatched() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.collections)
}

func New(db *sql.DB, queries WatcherQuerier, logger zerolog.Logger, cfg config.Config) *Watcher {
	return &Watcher{
		db:            db,
		queries:       queries,
		logger:        logger.With().Str("component", "watcher").Logger(),
		debounceMs:    cfg.Watcher.DebounceMs,
		pollInterval:  cfg.Watcher.ReindexInterval,
		maxFileSize:   cfg.Storage.MaxFileSize,
		collections:   make(map[string]watchedCollection),
		dirty:         make(map[string]bool),
		hotRegisterCh: make(chan struct{}, 1),
	}
}

// WithPublisher sets the event bus publisher for watcher file-change events.
func (w *Watcher) WithPublisher(pub eventbus.Publisher) *Watcher {
	w.pub = pub
	w.rateLimiters = make(map[string]*rate.Limiter)
	return w
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

func (w *Watcher) WithEmbedQueue(eq *embed.Queue) *Watcher {
	w.embedQueue = eq
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

	filter, ferr := newFileFilter(absPath, excludePatterns, allowedExtensions, w.globalIgnore)
	if ferr != nil {
		w.logger.Warn().Err(ferr).Str("dir", absPath).Str("collection", collectionName).Msg("workspace .nano-brainignore failed to load, continuing without local matcher")
	} else if filter.localIgnore != nil {
		w.logger.Debug().Str("dir", absPath).Str("collection", collectionName).Msg("loaded workspace .nano-brainignore")
	}
	w.collections[absPath] = watchedCollection{
		name:              collectionName,
		dirPath:           absPath,
		workspaceHash:     workspaceHash,
		globPattern:       globPattern,
		excludePatterns:   excludePatterns,
		allowedExtensions: allowedExtensions,
		filter:            filter,
	}

	if w.fsw != nil {
		if _, err := os.Stat(absPath); err != nil {
			w.logger.Info().Str("dir", absPath).Str("collection", collectionName).Msg("collection path not found, skipping watch")
			return nil
		}
		if err := w.fsw.Add(absPath); err != nil {
			return fmt.Errorf("watch dir %s: %w", absPath, err)
		}
		// Mark dirty + nudge Run() so processDirty runs an immediate scan.
		// Without this, fsnotify only fires on FUTURE changes — existing files in a
		// freshly-registered workspace never get queued for embedding until server
		// restart (issue #308).
		w.dirty[absPath] = true
		select {
		case w.hotRegisterCh <- struct{}{}:
		default:
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

		case <-w.hotRegisterCh:
			w.processDirty(ctx)
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
	err := filepath.WalkDir(col.dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			w.logger.Warn().Err(err).Str("path", path).Msg("walk error, skipping")
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if col.filter != nil && col.filter.shouldSkip(path, d.IsDir()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.IsDir() {
			w.processFile(ctx, col, path)
		}
		return nil
	})
	if err != nil && !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		w.logger.Error().Err(err).Str("dir", col.dirPath).Msg("walk failed")
	}
}

func (w *Watcher) processFile(ctx context.Context, col watchedCollection, filePath string) {
	if isBinaryExtension(filePath) {
		w.logger.Debug().Str("file", filePath).Msg("skipping binary file (extension)")
		return
	}

	info, err := os.Stat(filePath)
	if err != nil {
		w.logger.Warn().Err(err).Str("file", filePath).Msg("stat failed, skipping")
		return
	}

	if info.IsDir() {
		return
	}

	w.logger.Info().
		Str("path", filePath).
		Str("collection", col.name).
		Msg("processing file")

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

	if isBinaryContent(content) {
		w.logger.Warn().Str("file", filePath).Msg("skipping binary file (non-UTF8 content)")
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

	var chunkIDs []uuid.UUID
	if w.db != nil {
		ids, err := w.upsertWithTx(ctx, col.workspaceHash, filePath, params, chunks, meta)
		if err != nil {
			w.logger.Error().Err(err).Str("file", filePath).Msg("index failed")
			return
		}
		chunkIDs = ids
	} else {
		ids, err := w.upsertWithoutTx(ctx, col.workspaceHash, params, chunks, meta)
		if err != nil {
			w.logger.Error().Err(err).Str("file", filePath).Msg("index failed")
			return
		}
		chunkIDs = ids
	}

	w.publishFileEvent(col.workspaceHash, filePath, "modified")

	w.logger.Info().
		Str("file", filePath).
		Str("collection", col.name).
		Int("chunks", len(chunkIDs)).
		Msg("indexed file")

	if w.embedQueue != nil {
		for _, id := range chunkIDs {
			w.embedQueue.Enqueue(id)
		}
	}

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
			w.logger.Warn().Err(err).Str("edge", e.SourceNode+"->"+e.TargetNode).Msg("graph edge upsert failed, rolling back")
			return
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

func (w *Watcher) publishFileEvent(workspace, filePath, action string) {
	if w.pub == nil {
		return
	}
	w.mu.Lock()
	lim, ok := w.rateLimiters[workspace]
	if !ok {
		lim = rate.NewLimiter(rate.Limit(10), 10)
		w.rateLimiters[workspace] = lim
	}
	w.mu.Unlock()

	if !lim.Allow() {
		return
	}

	payload, _ := json.Marshal(map[string]string{
		"path":   filePath,
		"action": action,
	})
	w.pub.Publish(eventbus.Event{
		Type:      "watcher",
		Workspace: workspace,
		Payload:   payload,
		TS:        time.Now(),
	})
}

func (w *Watcher) upsertWithTx(ctx context.Context, workspace, filePath string, params sqlc.UpsertDocumentBySourcePathParams, chunks []chunk.Chunk, meta pqtype.NullRawMessage) ([]uuid.UUID, error) {
	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // Rollback after Commit is a no-op in database/sql.

	tq := sqlc.New(tx)
	docRow, err := tq.UpsertDocumentBySourcePath(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("upsert document: %w", err)
	}

	ids, err := w.writeChunks(ctx, tq, docRow.ID, workspace, chunks, meta)
	if err != nil {
		return nil, fmt.Errorf("write chunks: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return ids, nil
}

func (w *Watcher) upsertWithoutTx(ctx context.Context, workspace string, params sqlc.UpsertDocumentBySourcePathParams, chunks []chunk.Chunk, meta pqtype.NullRawMessage) ([]uuid.UUID, error) {
	docRow, err := w.queries.UpsertDocumentBySourcePath(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("upsert document: %w", err)
	}

	ids, err := w.writeChunks(ctx, w.queries, docRow.ID, workspace, chunks, meta)
	if err != nil {
		return nil, fmt.Errorf("write chunks: %w", err)
	}
	return ids, nil
}

func (w *Watcher) writeChunks(ctx context.Context, q WatcherQuerier, docID uuid.UUID, workspace string, chunks []chunk.Chunk, meta pqtype.NullRawMessage) ([]uuid.UUID, error) {
	if err := q.DeleteChunksByDocumentID(ctx, sqlc.DeleteChunksByDocumentIDParams{
		DocumentID:    docID,
		WorkspaceHash: workspace,
	}); err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, 0, len(chunks))
	for _, ch := range chunks {
		id, err := q.UpsertChunk(ctx, sqlc.UpsertChunkParams{
			DocumentID:    docID,
			WorkspaceHash: workspace,
			ContentHash:   ch.Hash,
			Content:       ch.Content,
			ChunkIndex:    int32(ch.Sequence),
			StartLine:     sql.NullInt32{Int32: int32(ch.StartLine), Valid: true},
			EndLine:       sql.NullInt32{Int32: int32(ch.EndLine), Valid: true},
			Metadata:      meta,
		})
		if err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}
