package handlers

import (
	"context"
	"database/sql"
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

type resetWorkspaceTxBeginner interface {
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
}

type resetWorkspaceRequest struct {
	Workspace string `json:"workspace"`
}

type resetWorkspaceResponse struct {
	DeletedDocuments int64  `json:"deleted_documents"`
	Workspace        string `json:"workspace"`
}

func ResetWorkspace(q ResetWorkspaceQuerier, db resetWorkspaceTxBeginner, logger zerolog.Logger) echo.HandlerFunc {
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

		if db == nil {
			if err := q.DeleteDocumentsByWorkspace(ctx, req.Workspace); err != nil {
				logger.Error().Err(err).Str("workspace", req.Workspace).Msg("delete documents failed")
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete documents")
			}
			if err := q.DeleteWorkspace(ctx, req.Workspace); err != nil {
				logger.Error().Err(err).Str("workspace", req.Workspace).Msg("delete workspace failed (docs already deleted)")
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete workspace")
			}
		} else {
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				logger.Error().Err(err).Str("workspace", req.Workspace).Msg("begin transaction failed")
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to begin transaction")
			}
			txq := sqlc.New(tx)
			if err := txq.DeleteDocumentsByWorkspace(ctx, req.Workspace); err != nil {
				_ = tx.Rollback()
				logger.Error().Err(err).Str("workspace", req.Workspace).Msg("delete documents failed")
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete documents")
			}
			if err := txq.DeleteWorkspace(ctx, req.Workspace); err != nil {
				_ = tx.Rollback()
				logger.Error().Err(err).Str("workspace", req.Workspace).Msg("delete workspace failed")
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete workspace")
			}
			if err := tx.Commit(); err != nil {
				logger.Error().Err(err).Str("workspace", req.Workspace).Msg("commit transaction failed")
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to commit transaction")
			}
		}

		reqLog := LoggerFromCtx(c, logger)
		reqLog.Info().
			Str("workspace", req.Workspace).
			Int64("deleted_documents", count).
			Bool("transactional", db != nil).
			Msg("workspace reset")

		return c.JSON(http.StatusOK, resetWorkspaceResponse{
			DeletedDocuments: count,
			Workspace:        req.Workspace,
		})
	}
}

var _ ResetWorkspaceQuerier = (*sqlc.Queries)(nil)
