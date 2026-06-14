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
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"encoding/json"

	"github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
	gitignore "github.com/sabhiram/go-gitignore"
	"github.com/nano-brain/nano-brain/internal/chunker"
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
	DeleteChunksByIDs(ctx context.Context, ids []uuid.UUID) error
	DeleteDocumentByIDAndWorkspace(ctx context.Context, arg sqlc.DeleteDocumentByIDAndWorkspaceParams) (int64, error)
	UpsertChunk(ctx context.Context, arg sqlc.UpsertChunkParams) (uuid.UUID, error)
	GetDocumentBySourcePath(ctx context.Context, arg sqlc.GetDocumentBySourcePathParams) (sqlc.Document, error)
	InsertChunkEntity(ctx context.Context, arg sqlc.InsertChunkEntityParams) error
	ListChunksByDocumentID(ctx context.Context, arg sqlc.ListChunksByDocumentIDParams) ([]sqlc.ListChunksByDocumentIDRow, error)
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

type fileState struct {
	ModTime time.Time
	Size    int64
	Hash    string
}

type Watcher struct {
	db             *sql.DB
	queries        WatcherQuerier
	graphQuerier   GraphQuerier
	logger         zerolog.Logger
	debounceMs     int
	pollInterval   int
	maxFileSize    int64
	chunkOverlap   int
	symbolRegistry *symbol.Registry
	graphRegistry  *graph.Registry
	embedQueue     *embed.Queue
	dispatcher     *chunker.Dispatcher

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

	summarizeNotify func()
	flowNotify      func(string)

	fileCache   map[string]fileState
	fileCacheMu sync.RWMutex
	hasNewEvents atomic.Bool
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
		chunkOverlap:  cfg.Watcher.ChunkOverlap,
		collections:   make(map[string]watchedCollection),
		dirty:         make(map[string]bool),
		hotRegisterCh: make(chan struct{}, 1),
		fileCache:     make(map[string]fileState),
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

func (w *Watcher) WithDispatcher(d *chunker.Dispatcher) *Watcher {
	w.dispatcher = d
	return w
}

func (w *Watcher) WithSummarizeNotify(fn func()) *Watcher {
	w.summarizeNotify = fn
	return w
}

func (w *Watcher) WithFlowNotify(fn func(string)) *Watcher {
	w.flowNotify = fn
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
			if !w.hasNewEvents.Swap(false) {
				continue
			}
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

	for excluded := range defaultExcludeDirs {
		sep := string(os.PathSeparator)
		if strings.Contains(event.Name, sep+excluded+sep) ||
			strings.HasSuffix(event.Name, sep+excluded) {
			return
		}
	}

	w.hasNewEvents.Store(true)

	dir := filepath.Dir(event.Name)

	w.mu.Lock()
	if _, ok := w.collections[dir]; ok {
		w.dirty[dir] = true
	}
	w.mu.Unlock()

	if event.Op&(fsnotify.Remove|fsnotify.Rename) != 0 {
		w.fileCacheMu.Lock()
		delete(w.fileCache, event.Name)
		w.fileCacheMu.Unlock()
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
	stack := &gitignoreStack{}

	err := filepath.WalkDir(col.dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			w.logger.Warn().Err(err).Str("path", path).Msg("walk error, skipping")
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}

		stack.PopAbove(path)

		if d.IsDir() {
			gitignorePath := filepath.Join(path, ".gitignore")
			if info, err := os.Stat(gitignorePath); err == nil && !info.IsDir() {
				if gi, err := gitignore.CompileIgnoreFile(gitignorePath); err == nil {
					stack.Push(path, gi)
					w.logger.Debug().Str("path", gitignorePath).Msg("loaded nested .gitignore")
				}
			}
		}

		if col.filter != nil && col.filter.shouldSkip(path, d.IsDir()) {
			if d.IsDir() {
				w.cleanupPathPrefix(ctx, col, path)
				return filepath.SkipDir
			}
			w.cleanupIgnoredDocument(ctx, col, path)
			return nil
		}

		if stack.Matches(path) {
			if d.IsDir() {
				w.cleanupPathPrefix(ctx, col, path)
				return filepath.SkipDir
			}
			w.cleanupIgnoredDocument(ctx, col, path)
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

// ShouldSkipPath checks whether the given absolute path matches any of the
// active ignore rules (.nano-brainignore, .gitignore, excludePatterns) for
// the named collection. Returns false if the collection is not found.
func (w *Watcher) ShouldSkipPath(collectionName, workspaceHash, absPath string, isDir bool) bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, col := range w.collections {
		if col.name == collectionName && col.workspaceHash == workspaceHash {
			if col.filter != nil {
				return col.filter.shouldSkip(absPath, isDir)
			}
			return false
		}
	}
	return false
}

// cleanupIgnoredDocument deletes any existing document+chunks for filePath,
// then removes the entry from the file cache. Called when a file newly
// matches .nano-brainignore or .gitignore patterns during a walk.
func (w *Watcher) cleanupIgnoredDocument(ctx context.Context, col watchedCollection, filePath string) {
	doc, err := w.queries.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
		SourcePath:    filePath,
		WorkspaceHash: col.workspaceHash,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return
		}
		w.logger.Warn().Err(err).Str("file", filePath).Msg("cleanupIgnoredDocument: get document failed")
		return
	}

	if err := w.queries.DeleteChunksByDocumentID(ctx, sqlc.DeleteChunksByDocumentIDParams{
		DocumentID:    doc.ID,
		WorkspaceHash: col.workspaceHash,
	}); err != nil {
		w.logger.Warn().Err(err).Str("file", filePath).Msg("cleanupIgnoredDocument: delete chunks failed")
		// fall through — still try to delete the document and cache entry
	}

	if _, err := w.queries.DeleteDocumentByIDAndWorkspace(ctx, sqlc.DeleteDocumentByIDAndWorkspaceParams{
		ID:            doc.ID,
		WorkspaceHash: col.workspaceHash,
	}); err != nil {
		w.logger.Warn().Err(err).Str("file", filePath).Msg("cleanupIgnoredDocument: delete document failed")
	}

	w.fileCacheMu.Lock()
	delete(w.fileCache, filePath)
	w.fileCacheMu.Unlock()

	w.logger.Debug().Str("file", filePath).Str("doc_id", doc.ID.String()).Msg("cleaned up document matching ignore pattern")
}

// cleanupPathPrefix deletes all documents+chunks whose source_path starts
// with the given directory prefix. Called when shouldSkip matches a
// directory — files inside are not walked individually.
func (w *Watcher) cleanupPathPrefix(ctx context.Context, col watchedCollection, dirPath string) {
	prefix := filepath.Clean(dirPath) + "/"

	if _, err := w.db.ExecContext(ctx,
		`DELETE FROM chunks WHERE document_id IN (
			SELECT id FROM documents WHERE source_path LIKE $1 AND workspace_hash = $2
		)`, prefix+"%", col.workspaceHash); err != nil {
		w.logger.Warn().Err(err).Str("prefix", prefix).Msg("cleanupPathPrefix: delete chunks failed")
	}

	res, err := w.db.ExecContext(ctx,
		`DELETE FROM documents WHERE source_path LIKE $1 AND workspace_hash = $2`,
		prefix+"%", col.workspaceHash)
	if err != nil {
		w.logger.Warn().Err(err).Str("prefix", prefix).Msg("cleanupPathPrefix: delete documents failed")
		return
	}
	if n, _ := res.RowsAffected(); n > 0 {
		w.logger.Debug().Str("prefix", prefix).Int64("deleted", n).Msg("cleaned up documents under ignored directory")
	}

	w.fileCacheMu.Lock()
	for cachedPath := range w.fileCache {
		if strings.HasPrefix(cachedPath, prefix) {
			delete(w.fileCache, cachedPath)
		}
	}
	w.fileCacheMu.Unlock()
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

	// Fast-path: skip if mtime+size unchanged (biggest perf win from issue #375)
	w.fileCacheMu.RLock()
	cached, exists := w.fileCache[filePath]
	w.fileCacheMu.RUnlock()

	if exists && cached.ModTime.Equal(info.ModTime()) && cached.Size == info.Size() {
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
		w.fileCacheMu.Lock()
		w.fileCache[filePath] = fileState{ModTime: info.ModTime(), Size: info.Size(), Hash: contentHash}
		w.fileCacheMu.Unlock()
		return
	}

	chunks := w.chunkContent(string(content), filePath)
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

	w.fileCacheMu.Lock()
	w.fileCache[filePath] = fileState{ModTime: info.ModTime(), Size: info.Size(), Hash: contentHash}
	w.fileCacheMu.Unlock()

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

	w.extractAndInsertEntities(ctx, col.workspaceHash, chunks, chunkIDs)

	if w.symbolRegistry != nil {
		w.extractAndUpsertSymbols(ctx, col, filePath, content)
		if w.summarizeNotify != nil {
			w.summarizeNotify()
		}
	}
	if w.graphRegistry != nil {
		w.extractAndUpsertEdges(ctx, col, filePath, content)
		if w.flowNotify != nil {
			w.flowNotify(col.workspaceHash)
		}
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
		docRow, err := w.queries.UpsertDocumentBySourcePath(ctx, params)
		if err != nil {
			w.logger.Warn().Err(err).Str("symbol", s.Name).Msg("symbol upsert failed")
			continue
		}

		chunks := w.chunkContent(s.Signature, sourcePath)
		if len(chunks) == 0 {
			continue
		}
		chunkMeta := pqtype.NullRawMessage{RawMessage: metaBytes, Valid: true}
		chunkIDs, err := w.writeChunks(ctx, w.queries, docRow.ID, col.workspaceHash, chunks, chunkMeta)
		if err != nil {
			w.logger.Warn().Err(err).Str("symbol", s.Name).Msg("symbol chunk write failed")
			continue
		}
		if w.embedQueue != nil {
			for _, id := range chunkIDs {
				w.embedQueue.Enqueue(id)
			}
		}
	}

	w.logger.Info().
		Str("file", filePath).
		Int("symbols", len(syms)).
		Msg("symbols extracted")
}

// buildEdgeMetadata merges e.Metadata (if non-nil) with {"line", "language"},
// giving the caller-supplied fields priority but always including line+language.
func buildEdgeMetadata(e graph.Edge) ([]byte, error) {
	merged := make(map[string]any, len(e.Metadata)+2)
	for k, v := range e.Metadata {
		merged[k] = v
	}
	merged["line"] = e.Line
	merged["language"] = e.Language
	return json.Marshal(merged)
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
		meta, _ := buildEdgeMetadata(e)
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

// ReextractSymbolsForWorkspace re-runs symbol extraction for every file in the
// workspace's collections, bypassing the content-hash early-exit. Returns the
// number of files processed.
func (w *Watcher) ReextractSymbolsForWorkspace(ctx context.Context, workspaceHash string) int {
	if w.symbolRegistry == nil {
		return 0
	}

	w.mu.Lock()
	var cols []watchedCollection
	for _, col := range w.collections {
		if col.workspaceHash == workspaceHash {
			cols = append(cols, col)
		}
	}
	w.mu.Unlock()

	var count int
	for _, col := range cols {
		_ = filepath.WalkDir(col.dirPath, func(path string, d fs.DirEntry, err error) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err != nil || d.IsDir() {
				return nil
			}
			if col.filter != nil && col.filter.shouldSkip(path, false) {
				return nil
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			w.extractAndUpsertSymbols(ctx, col, path, content)
			count++
			return nil
		})
	}
	return count
}

// ReextractEdgesForWorkspace re-runs graph edge extraction for every file in
// the workspace's collections, bypassing the content-hash early-exit. This is
// needed when a new extractor is added after the workspace was already indexed.
// Returns the number of files processed.
func (w *Watcher) ReextractEdgesForWorkspace(ctx context.Context, workspaceHash string) int {
	if w.graphRegistry == nil {
		return 0
	}

	w.mu.Lock()
	var cols []watchedCollection
	for _, col := range w.collections {
		if col.workspaceHash == workspaceHash {
			cols = append(cols, col)
		}
	}
	w.mu.Unlock()

	var count int
	for _, col := range cols {
		_ = filepath.WalkDir(col.dirPath, func(path string, d fs.DirEntry, err error) error {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if err != nil || d.IsDir() {
				return nil
			}
			if col.filter != nil && col.filter.shouldSkip(path, false) {
				return nil
			}
			content, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			w.extractAndUpsertEdges(ctx, col, path, content)
			count++
			return nil
		})
	}
	return count
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

func (w *Watcher) upsertWithTx(ctx context.Context, workspace, filePath string, params sqlc.UpsertDocumentBySourcePathParams, chunks []chunker.Chunk, meta pqtype.NullRawMessage) ([]uuid.UUID, error) {
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

func (w *Watcher) upsertWithoutTx(ctx context.Context, workspace string, params sqlc.UpsertDocumentBySourcePathParams, chunks []chunker.Chunk, meta pqtype.NullRawMessage) ([]uuid.UUID, error) {
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

func (w *Watcher) writeChunks(ctx context.Context, q WatcherQuerier, docID uuid.UUID, workspace string, chunks []chunker.Chunk, meta pqtype.NullRawMessage) ([]uuid.UUID, error) {
	// Step 1: Upsert ALL new chunks (ON CONFLICT handles both new inserts and updates)
	ids := make([]uuid.UUID, 0, len(chunks))
	newHashes := make(map[string]bool, len(chunks))
	
	for _, ch := range chunks {
		id, err := q.UpsertChunk(ctx, sqlc.UpsertChunkParams{
			DocumentID:        docID,
			WorkspaceHash:     workspace,
			ContentHash:       ch.Hash,
			Content:           ch.Content,
			ChunkIndex:        int32(ch.Sequence),
			StartLine:         sql.NullInt32{Int32: int32(ch.StartLine), Valid: true},
			EndLine:           sql.NullInt32{Int32: int32(ch.EndLine), Valid: true},
			Metadata:          meta,
			SymbolName:        nullString(ch.SymbolName),
			SymbolKind:        nullString(ch.SymbolKind),
			Language:          nullString(ch.Language),
			LineStart:         nullInt32(ch.StartLine),
			LineEnd:           nullInt32(ch.EndLine),
			ChunkType:         string(ch.ChunkType),
			EmbeddingStrategy: string(ch.EmbeddingStrategy),
		})
		if err != nil {
			return nil, fmt.Errorf("upsert chunk: %w", err)
		}
		ids = append(ids, id)
		newHashes[ch.Hash] = true
	}

	// Step 2: Delete old chunks that are no longer present
	existing, err := q.ListChunksByDocumentID(ctx, sqlc.ListChunksByDocumentIDParams{
		DocumentID:    docID,
		WorkspaceHash: workspace,
	})
	if err != nil {
		return nil, fmt.Errorf("list existing chunks: %w", err)
	}

	var staleIDs []uuid.UUID
	for _, ch := range existing {
		if !newHashes[ch.ContentHash] {
			staleIDs = append(staleIDs, ch.ID)
		}
	}

	if len(staleIDs) > 0 {
		if err := q.DeleteChunksByIDs(ctx, staleIDs); err != nil {
			return nil, fmt.Errorf("delete stale chunks: %w", err)
		}
	}

	return ids, nil
}

func (w *Watcher) chunkContent(content string, filePath string) []chunker.Chunk {
	if w.dispatcher != nil {
		return w.dispatcher.Chunk(content, filePath)
	}
	fixed := chunker.NewFixedChunkerWithOverlap(w.chunkOverlap)
	return fixed.Chunk(content, filePath)
}

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func nullInt32(v int) sql.NullInt32 {
	if v == 0 {
		return sql.NullInt32{}
	}
	return sql.NullInt32{Int32: int32(v), Valid: true}
}