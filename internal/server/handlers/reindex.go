package handlers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/embed"
	"github.com/nano-brain/nano-brain/internal/eventbus"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/watcher"
	"github.com/rs/zerolog"
)

const maxIncrementalFileSize = 10 * 1024 * 1024

type ReindexQuerier interface {
	ListCollections(ctx context.Context, workspaceHash string) ([]sqlc.Collection, error)
	ListDocumentSourcePathsAndHashes(ctx context.Context, arg sqlc.ListDocumentSourcePathsAndHashesParams) ([]sqlc.ListDocumentSourcePathsAndHashesRow, error)
	DeleteDocumentByIDAndWorkspace(ctx context.Context, arg sqlc.DeleteDocumentByIDAndWorkspaceParams) (int64, error)
	DeleteChunksByDocumentID(ctx context.Context, arg sqlc.DeleteChunksByDocumentIDParams) error
	DeleteSymbolDocumentsByCollection(ctx context.Context, arg sqlc.DeleteSymbolDocumentsByCollectionParams) error
	ResetAndReturnChunkIDsByCollection(ctx context.Context, arg sqlc.ResetAndReturnChunkIDsByCollectionParams) ([]uuid.UUID, error)
}

type reindexRequest struct {
	Workspace string `json:"workspace"`
	Root      string `json:"root"`
	ForceWipe bool   `json:"force_wipe"`
}

type reindexResponse struct {
	Status           string `json:"status"`
	ChunksEnqueued   int64  `json:"chunks_enqueued"`
	WatcherTriggered bool   `json:"watcher_triggered"`
	Scanned          int    `json:"scanned"`
	Skipped          int    `json:"skipped"`
	Embedded         int    `json:"embedded"`
	FilesPending     int    `json:"files_pending_ingest"`
	Deleted          int    `json:"deleted"`
	DurationMs       int64  `json:"duration_ms"`
	Message          string `json:"message"`
}

// TriggerReindex godoc
// @Summary      Trigger a reindex (incremental or force-wipe) for a workspace
// @Description  Requests an asynchronous watcher rescan; files_pending_ingest estimates changed files, while embedded and chunks_enqueued count completed/queued work at response time. force_wipe requeues existing chunks.
// @Tags         reindex
// @Accept       json
// @Produce      json
// @Param        request body reindexRequest true "Reindex options"
// @Success      200 {object} reindexResponse
// @Success      202 {object} reindexResponse
// @Failure      400 {object} map[string]string
// @Security     WorkspaceRegisteredAuth
// @Security     CSRFToken
// @Router       /api/v1/reindex [post]
func TriggerReindex(queries ReindexQuerier, w *watcher.Watcher, eq *embed.Queue, pub eventbus.Publisher, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		start := time.Now()
		var req reindexRequest
		if err := c.Bind(&req); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
		}

		workspace := c.Get("workspace").(string)

		publishReindex(pub, workspace, "started", 0, 0, 0, 0, "")

		collections, err := queries.ListCollections(c.Request().Context(), workspace)
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("list collections: %v", err))
		}

		targets := collectionsToReindex(collections, req.Root)
		if len(targets) == 0 {
			reqLog := LoggerFromCtx(c, logger)
			reqLog.Info().Str("workspace", workspace).Str("root", req.Root).
				Msg("reindex queued: no matching collections")
			return c.JSON(http.StatusAccepted, reindexResponse{
				Status:  "queued",
				Message: fmt.Sprintf("no collections found for workspace %s", workspace),
			})
		}

		if req.ForceWipe {
			return triggerForceWipe(c, queries, w, eq, pub, logger, workspace, req.Root, targets, start)
		}

		return triggerIncremental(c, queries, w, pub, logger, workspace, req.Root, targets, start)
	}
}

func triggerForceWipe(c echo.Context, queries ReindexQuerier, w *watcher.Watcher, eq *embed.Queue, pub eventbus.Publisher, logger zerolog.Logger, workspace, root string, targets []sqlc.Collection, start time.Time) error {
	var totalChunks int64
	var watcherTriggered bool
	for _, col := range targets {
		ids, err := queries.ResetAndReturnChunkIDsByCollection(c.Request().Context(), sqlc.ResetAndReturnChunkIDsByCollectionParams{
			WorkspaceHash: workspace,
			Collection:    col.Name,
		})
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("reset embed status: %v", err))
		}
		if eq != nil {
			for _, id := range ids {
				if eq.ForceEnqueue(id) {
					totalChunks++
				}
			}
		}
		_ = queries.DeleteSymbolDocumentsByCollection(c.Request().Context(), sqlc.DeleteSymbolDocumentsByCollectionParams{
			WorkspaceHash: workspace,
			Collection:    col.Name,
		})
		if w.TriggerRescanByName(col.Name, workspace) {
			watcherTriggered = true
		}
	}
	if watcherTriggered && w != nil {
		go w.ReextractSymbolsForWorkspace(context.Background(), workspace)
	}

	publishReindex(pub, workspace, "queued", int(totalChunks), 0, 0, 0, "")

	reqLog := LoggerFromCtx(c, logger)
	reqLog.Info().
		Str("workspace", workspace).
		Str("root", root).
		Int64("chunks_enqueued", totalChunks).
		Bool("watcher_triggered", watcherTriggered).
		Msg("reindex force-wipe queued")

	return c.JSON(http.StatusAccepted, reindexResponse{
		Status:           "queued",
		ChunksEnqueued:   totalChunks,
		WatcherTriggered: watcherTriggered,
		DurationMs:       time.Since(start).Milliseconds(),
		Message:          fmt.Sprintf("Reindex (force-wipe) queued for workspace %s", workspace),
	})
}

func triggerIncremental(c echo.Context, queries ReindexQuerier, w *watcher.Watcher, pub eventbus.Publisher, logger zerolog.Logger, workspace, root string, targets []sqlc.Collection, start time.Time) error {
	var scanned, skipped, filesPending, deleted int
	var watcherTriggered bool

	for _, col := range targets {
		diskFiles, err := walkCollectionFiles(col)
		if err != nil {
			reqLog := LoggerFromCtx(c, logger)
			reqLog.Warn().Err(err).Str("collection", col.Name).Msg("walk collection failed, skipping")
			continue
		}

		indexedRows, err := queries.ListDocumentSourcePathsAndHashes(c.Request().Context(), sqlc.ListDocumentSourcePathsAndHashesParams{
			WorkspaceHash: workspace,
			Collection:    col.Name,
		})
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("list indexed docs: %v", err))
		}

		type indexedEntry struct {
			id          uuid.UUID
			contentHash string
		}
		indexed := make(map[string]indexedEntry, len(indexedRows))
		for _, row := range indexedRows {
			indexed[row.SourcePath] = indexedEntry{id: row.ID, contentHash: row.ContentHash}
		}

		// Guard: if disk walk returned nothing but DB has indexed docs, skip orphan
		// deletion — files may all exceed maxIncrementalFileSize or be temporarily
		// unreadable. Deleting everything would be catastrophic data loss.
		if len(diskFiles) == 0 && len(indexedRows) > 0 {
			reqLog := LoggerFromCtx(c, logger)
			reqLog.Warn().
				Str("collection", col.Name).
				Int("indexed", len(indexedRows)).
				Msg("disk walk returned empty for non-empty collection — skipping orphan deletion")
			continue
		}

		var colHasChanges bool
		changedFiles := make(map[string]bool)
		ignoredFiles := make(map[string]bool)
		for path, diskHash := range diskFiles {
			// Skip files matching ignore patterns — the watcher's periodic
			// scanCollection handles their cleanup via shouldSkip.
			if w != nil && w.ShouldSkipPath(col.Name, workspace, path, false) {
				// Also purge from indexed so the orphan loop below deletes
				// any previously-indexed document + chunks.
				delete(indexed, path)
				ignoredFiles[path] = true
				skipped++
				continue
			}
			scanned++
			entry, exists := indexed[path]
			if !exists {
				colHasChanges = true
				filesPending++
			} else if entry.contentHash == diskHash {
				skipped++
			} else {
				if err := queries.DeleteChunksByDocumentID(c.Request().Context(), sqlc.DeleteChunksByDocumentIDParams{
					DocumentID:    entry.id,
					WorkspaceHash: workspace,
				}); err != nil {
					reqLog := LoggerFromCtx(c, logger)
					reqLog.Warn().Err(err).Str("path", path).Msg("delete chunks failed")
				}
				changedFiles[path] = true
				colHasChanges = true
				filesPending++
			}
		}
		if colHasChanges {
			if w.TriggerRescanByName(col.Name, workspace) {
				watcherTriggered = true
			}
		}

		for path, entry := range indexed {
			// Never treat synthetic scheme URIs (flow://, summary://, opencode://, memory://)
			// as deleted filesystem files.
			if strings.Contains(path, "://") {
				continue
			}
			// Synthetic child docs (e.g. "/abs/file.js?symbol=foo&kind=function") are not
			// disk paths themselves: keep them while their parent file remains unchanged on
			// disk, and drop them only when the parent file changed, became ignored, or was
			// deleted from disk (#624).
			if qIdx := strings.IndexByte(path, '?'); qIdx > 0 {
				basePath := path[:qIdx]
				refreshingParent := changedFiles[basePath] || ignoredFiles[basePath]
				if !refreshingParent {
					if _, onDisk := diskFiles[basePath]; onDisk {
						continue
					}
					if _, statErr := os.Stat(basePath); statErr == nil {
						continue
					}
				}
				_ = queries.DeleteChunksByDocumentID(c.Request().Context(), sqlc.DeleteChunksByDocumentIDParams{
					DocumentID:    entry.id,
					WorkspaceHash: workspace,
				})
				_, _ = queries.DeleteDocumentByIDAndWorkspace(c.Request().Context(), sqlc.DeleteDocumentByIDAndWorkspaceParams{
					ID:            entry.id,
					WorkspaceHash: workspace,
				})
				continue
			}
			if _, onDisk := diskFiles[path]; !onDisk {
				if _, statErr := os.Stat(path); statErr == nil {
					// File still exists on disk — was likely filtered (e.g., too large). Skip.
					continue
				}
				chunksErr := queries.DeleteChunksByDocumentID(c.Request().Context(), sqlc.DeleteChunksByDocumentIDParams{
					DocumentID:    entry.id,
					WorkspaceHash: workspace,
				})
				if chunksErr != nil {
					reqLog := LoggerFromCtx(c, logger)
					reqLog.Warn().Err(chunksErr).Str("path", path).Msg("delete chunks for deleted doc failed")
				}
				_, docErr := queries.DeleteDocumentByIDAndWorkspace(c.Request().Context(), sqlc.DeleteDocumentByIDAndWorkspaceParams{
					ID:            entry.id,
					WorkspaceHash: workspace,
				})
				if docErr != nil {
					reqLog := LoggerFromCtx(c, logger)
					reqLog.Warn().Err(docErr).Str("path", path).Msg("delete document failed")
				}
				if chunksErr == nil && docErr == nil {
					deleted++
				}
			}
		}
	}

	publishReindex(pub, workspace, "queued", 0, 0, deleted, skipped, "")

	reqLog := LoggerFromCtx(c, logger)
	reqLog.Info().
		Str("workspace", workspace).
		Str("root", root).
		Int("scanned", scanned).
		Int("skipped", skipped).
		Int("files_pending_ingest", filesPending).
		Int("deleted", deleted).
		Bool("watcher_triggered", watcherTriggered).
		Msg("incremental reindex queued")

	return c.JSON(http.StatusAccepted, reindexResponse{
		Status:           "queued",
		WatcherTriggered: watcherTriggered,
		Scanned:          scanned,
		Skipped:          skipped,
		FilesPending:     filesPending,
		Deleted:          deleted,
		DurationMs:       time.Since(start).Milliseconds(),
		Message:          fmt.Sprintf("Incremental rescan requested for workspace %s; verify persistence via stats", workspace),
	})
}

func walkCollectionFiles(col sqlc.Collection) (map[string]string, error) {
	result := make(map[string]string)
	if col.Path == "" {
		return result, nil
	}
	// Verify root is accessible before walking — an inaccessible root would
	// return an empty map which triggerIncremental would interpret as "all files
	// deleted", causing catastrophic data loss.
	if _, err := os.Stat(col.Path); err != nil {
		return nil, fmt.Errorf("walk collection %q: root path inaccessible: %w", col.Name, err)
	}
	err := filepath.WalkDir(col.Path, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			// Propagate root-level errors; skip inaccessible subdirectories/files.
			if path == col.Path {
				return walkErr
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil || info.Size() > maxIncrementalFileSize {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()
		h := sha256.New()
		if _, err := io.Copy(h, f); err != nil {
			return nil
		}
		result[path] = hex.EncodeToString(h.Sum(nil))
		return nil
	})
	return result, err
}

func collectionsToReindex(collections []sqlc.Collection, root string) []sqlc.Collection {
	if root == "" {
		return collections
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		absRoot = root
	}

	var matched []sqlc.Collection
	for _, col := range collections {
		absPath, err := filepath.Abs(col.Path)
		if err != nil {
			absPath = col.Path
		}
		if absPath == absRoot || col.Name == root {
			matched = append(matched, col)
		}
	}

	if len(matched) == 0 {
		return collections
	}
	return matched
}

func publishReindex(pub eventbus.Publisher, workspace, state string, enqueued, embedded, deleted, skipped int, errMsg string) {
	if pub == nil {
		return
	}
	m := map[string]any{
		"state":    state,
		"enqueued": enqueued,
		"embedded": embedded,
		"deleted":  deleted,
		"skipped":  skipped,
	}
	if errMsg != "" {
		m["error"] = errMsg
	}
	payload, _ := json.Marshal(m)
	pub.Publish(eventbus.Event{
		Type:      "reindex",
		Workspace: workspace,
		Payload:   payload,
		TS:        time.Now(),
	})
}

// TriggerUpdate godoc
// @Summary      Re-extract graph edges and symbols for a workspace
// @Description  Asynchronously re-extracts graph edges and symbols for all collections in the workspace
// @Tags         reindex
// @Produce      json
// @Success      202 {object} reindexResponse
// @Failure      400 {object} map[string]string
// @Security     WorkspaceRegisteredAuth
// @Security     CSRFToken
// @Router       /api/v1/update [post]
func TriggerUpdate(w *watcher.Watcher, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		workspace := c.Get("workspace").(string)

		if w != nil {
			go func() {
				ctx := context.Background()
				start := time.Now()
				edgeCount := w.ReextractEdgesForWorkspace(ctx, workspace)
				symCount := w.ReextractSymbolsForWorkspace(ctx, workspace)
				logger.Info().
					Str("workspace", workspace).
					Int("edges_files", edgeCount).
					Int("symbols_files", symCount).
					Dur("duration", time.Since(start)).
					Msg("update re-extraction complete")
			}()
		}

		return c.JSON(http.StatusAccepted, reindexResponse{
			Status:  "queued",
			Message: fmt.Sprintf("Update queued for all collections in workspace %s", workspace),
		})
	}
}
