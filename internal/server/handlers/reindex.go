package handlers

import (
	"context"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/watcher"
	"github.com/rs/zerolog"
)

type ReindexQuerier interface {
	ResetEmbedStatusByCollection(ctx context.Context, arg sqlc.ResetEmbedStatusByCollectionParams) (int64, error)
}

type reindexRequest struct {
	Workspace string `json:"workspace"`
	Root      string `json:"root"`
}

type reindexResponse struct {
	Status          string `json:"status"`
	ChunksEnqueued  int64  `json:"chunks_enqueued"`
	WatcherTriggered bool  `json:"watcher_triggered"`
	Message         string `json:"message"`
}

func TriggerReindex(queries ReindexQuerier, w *watcher.Watcher, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		var req reindexRequest
		if err := c.Bind(&req); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
		}

		workspace := c.Get("workspace").(string)

		if req.Root == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "root (collection name) is required")
		}

		n, err := queries.ResetEmbedStatusByCollection(c.Request().Context(), sqlc.ResetEmbedStatusByCollectionParams{
			WorkspaceHash: workspace,
			Collection:    req.Root,
		})
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, fmt.Sprintf("reset embed status: %v", err))
		}

		triggered := w.TriggerRescanByName(req.Root, workspace)

		reqLog := LoggerFromCtx(c, logger)
		reqLog.Info().
			Str("workspace", workspace).
			Str("root", req.Root).
			Int64("chunks_enqueued", n).
			Bool("watcher_triggered", triggered).
			Msg("reindex queued")

		return c.JSON(http.StatusAccepted, reindexResponse{
			Status:          "queued",
			ChunksEnqueued:  n,
			WatcherTriggered: triggered,
			Message:         fmt.Sprintf("Reindex queued for collection %s in workspace %s", req.Root, workspace),
		})
	}
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
