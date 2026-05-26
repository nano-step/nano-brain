package handlers

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/embed"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/watcher"
	"github.com/rs/zerolog"
)

type ReindexQuerier interface {
	ResetAndReturnChunkIDsByCollection(ctx context.Context, arg sqlc.ResetAndReturnChunkIDsByCollectionParams) ([]uuid.UUID, error)
	ListCollections(ctx context.Context, workspaceHash string) ([]sqlc.Collection, error)
	DeleteSymbolDocumentsByCollection(ctx context.Context, arg sqlc.DeleteSymbolDocumentsByCollectionParams) error
}

type reindexRequest struct {
	Workspace string `json:"workspace"`
	Root      string `json:"root"`
}

type reindexResponse struct {
	Status           string `json:"status"`
	ChunksEnqueued   int64  `json:"chunks_enqueued"`
	WatcherTriggered bool   `json:"watcher_triggered"`
	Message          string `json:"message"`
}

func TriggerReindex(queries ReindexQuerier, w *watcher.Watcher, eq *embed.Queue, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		var req reindexRequest
		if err := c.Bind(&req); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
		}

		workspace := c.Get("workspace").(string)

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
					if eq.Enqueue(id) {
						totalChunks++
					}
				}
			} else {
				totalChunks += int64(len(ids))
			}
			_ = queries.DeleteSymbolDocumentsByCollection(c.Request().Context(), sqlc.DeleteSymbolDocumentsByCollectionParams{
				WorkspaceHash: workspace,
				Collection:    col.Name,
			})
			if w.TriggerRescanByName(col.Name, workspace) {
				watcherTriggered = true
			}
		}

		reqLog := LoggerFromCtx(c, logger)
		reqLog.Info().
			Str("workspace", workspace).
			Str("root", req.Root).
			Int64("chunks_enqueued", totalChunks).
			Bool("watcher_triggered", watcherTriggered).
			Msg("reindex queued")

		return c.JSON(http.StatusAccepted, reindexResponse{
			Status:           "queued",
			ChunksEnqueued:   totalChunks,
			WatcherTriggered: watcherTriggered,
			Message:          fmt.Sprintf("Reindex queued for workspace %s", workspace),
		})
	}
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

func TriggerUpdate(logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		workspace := c.Get("workspace").(string)

		reqLog := LoggerFromCtx(c, logger)
		reqLog.Info().
			Str("workspace", workspace).
			Msg("update queued for all collections")

		return c.JSON(http.StatusAccepted, reindexResponse{
			Status:  "queued",
			Message: fmt.Sprintf("Update queued for all collections in workspace %s", workspace),
		})
	}
}
