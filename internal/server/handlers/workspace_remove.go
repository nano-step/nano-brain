package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

type RemoveWorkspaceQuerier interface {
	GetWorkspaceByHash(ctx context.Context, hash string) (sqlc.Workspace, error)
	CountDocumentsByWorkspace(ctx context.Context, workspaceHash string) (int64, error)
	DeleteDocumentsByWorkspace(ctx context.Context, workspaceHash string) error
	DeleteWorkspace(ctx context.Context, hash string) error
}

type removeWorkspaceResponse struct {
	Workspace    string `json:"workspace"`
	DeletedDocs  int64  `json:"deleted_docs"`
	WorkspaceRemoved bool `json:"workspace_removed"`
}

func RemoveWorkspace(q RemoveWorkspaceQuerier, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		hash := c.Param("hash")
		if hash == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "workspace hash is required")
		}

		ctx := c.Request().Context()

		if _, err := q.GetWorkspaceByHash(ctx, hash); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return echo.NewHTTPError(http.StatusNotFound, "workspace not found")
			}
			logger.Error().Err(err).Str("workspace", hash).Msg("get workspace failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to look up workspace")
		}

		docCount, err := q.CountDocumentsByWorkspace(ctx, hash)
		if err != nil {
			logger.Error().Err(err).Str("workspace", hash).Msg("count documents failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to count documents")
		}

		if err := q.DeleteDocumentsByWorkspace(ctx, hash); err != nil {
			logger.Error().Err(err).Str("workspace", hash).Msg("delete documents failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete documents")
		}

		if err := q.DeleteWorkspace(ctx, hash); err != nil {
			logger.Error().Err(err).Str("workspace", hash).Msg("delete workspace failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete workspace")
		}

		reqLog := LoggerFromCtx(c, logger)
		reqLog.Info().
			Str("workspace", hash).
			Int64("deleted_docs", docCount).
			Msg("workspace removed")

		return c.JSON(http.StatusOK, removeWorkspaceResponse{
			Workspace:    hash,
			DeletedDocs:  docCount,
			WorkspaceRemoved: true,
		})
	}
}

var _ RemoveWorkspaceQuerier = (*sqlc.Queries)(nil)
