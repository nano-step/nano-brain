package handlers

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

type ResetWorkspaceQuerier interface {
	CountDocumentsByWorkspace(ctx context.Context, workspaceHash string) (int64, error)
	DeleteDocumentsByWorkspace(ctx context.Context, workspaceHash string) error
	DeleteWorkspace(ctx context.Context, hash string) error
}

type resetWorkspaceRequest struct {
	Workspace string `json:"workspace"`
}

type resetWorkspaceResponse struct {
	DeletedDocuments int64  `json:"deleted_documents"`
	Workspace        string `json:"workspace"`
}

func ResetWorkspace(q ResetWorkspaceQuerier, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		var req resetWorkspaceRequest
		if err := c.Bind(&req); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
		}
		if req.Workspace == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "workspace is required")
		}

		ctx := c.Request().Context()

		count, err := q.CountDocumentsByWorkspace(ctx, req.Workspace)
		if err != nil {
			logger.Error().Err(err).Str("workspace", req.Workspace).Msg("count documents failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to count documents")
		}

		if err := q.DeleteDocumentsByWorkspace(ctx, req.Workspace); err != nil {
			logger.Error().Err(err).Str("workspace", req.Workspace).Msg("delete documents failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete documents")
		}

		if err := q.DeleteWorkspace(ctx, req.Workspace); err != nil {
			logger.Error().Err(err).Str("workspace", req.Workspace).Msg("delete workspace failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete workspace")
		}

		reqLog := LoggerFromCtx(c, logger)
		reqLog.Info().Str("workspace", req.Workspace).Int64("deleted_documents", count).Msg("workspace reset")

		return c.JSON(http.StatusOK, resetWorkspaceResponse{
			DeletedDocuments: count,
			Workspace:        req.Workspace,
		})
	}
}

var _ ResetWorkspaceQuerier = (*sqlc.Queries)(nil)
