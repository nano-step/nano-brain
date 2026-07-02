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
	DeleteCodeSummarizationUsageByWorkspace(ctx context.Context, workspaceHash string) error
	DeleteCodeSummarizationFailuresByWorkspace(ctx context.Context, workspaceHash string) error
	DeleteWorkspace(ctx context.Context, hash string) error
}

type removeWorkspaceTxBeginner interface {
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
}

type removeWorkspaceResponse struct {
	Workspace        string `json:"workspace"`
	DeletedDocs      int64  `json:"deleted_docs"`
	WorkspaceRemoved bool   `json:"workspace_removed"`
}

// RemoveWorkspace godoc
// @Summary      Remove a workspace
// @Description  Deletes a workspace and all its documents. When db is non-nil, deletion runs inside a transaction.
// @Tags         workspaces
// @Produce      json
// @Param        hash path string true "Workspace hash"
// @Success      200 {object} removeWorkspaceResponse
// @Failure      400 {object} map[string]string
// @Failure      404 {object} map[string]string
// @Failure      500 {object} map[string]string
// @Router       /api/v1/workspaces/{hash} [delete]
func RemoveWorkspace(q RemoveWorkspaceQuerier, db removeWorkspaceTxBeginner, logger zerolog.Logger) echo.HandlerFunc {
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

	if db == nil {
		if err := q.DeleteDocumentsByWorkspace(ctx, hash); err != nil {
			logger.Error().Err(err).Str("workspace", hash).Msg("delete documents failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete documents")
		}
		if err := q.DeleteCodeSummarizationUsageByWorkspace(ctx, hash); err != nil {
			logger.Warn().Err(err).Str("workspace", hash).Msg("failed to cleanup code summarization usage")
		}
		if err := q.DeleteCodeSummarizationFailuresByWorkspace(ctx, hash); err != nil {
			logger.Warn().Err(err).Str("workspace", hash).Msg("failed to cleanup code summarization failures")
		}
		if err := q.DeleteWorkspace(ctx, hash); err != nil {
			logger.Error().Err(err).Str("workspace", hash).Msg("delete workspace failed (docs already deleted)")
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete workspace")
		}
		} else {
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				logger.Error().Err(err).Str("workspace", hash).Msg("begin transaction failed")
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to begin transaction")
			}
			txq := sqlc.New(tx)
			if err := txq.DeleteDocumentsByWorkspace(ctx, hash); err != nil {
				_ = tx.Rollback()
				logger.Error().Err(err).Str("workspace", hash).Msg("delete documents failed")
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete documents")
			}
			if err := txq.DeleteCodeSummarizationUsageByWorkspace(ctx, hash); err != nil {
				logger.Warn().Err(err).Str("workspace", hash).Msg("failed to cleanup code summarization usage")
			}
			if err := txq.DeleteCodeSummarizationFailuresByWorkspace(ctx, hash); err != nil {
				logger.Warn().Err(err).Str("workspace", hash).Msg("failed to cleanup code summarization failures")
			}
			if err := txq.DeleteWorkspace(ctx, hash); err != nil {
				_ = tx.Rollback()
				logger.Error().Err(err).Str("workspace", hash).Msg("delete workspace failed")
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete workspace")
			}
			if err := tx.Commit(); err != nil {
				logger.Error().Err(err).Str("workspace", hash).Msg("commit transaction failed")
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to commit transaction")
			}
		}

		reqLog := LoggerFromCtx(c, logger)
		reqLog.Info().
			Str("workspace", hash).
			Int64("deleted_docs", docCount).
			Bool("transactional", db != nil).
			Msg("workspace removed")

		return c.JSON(http.StatusOK, removeWorkspaceResponse{
			Workspace:        hash,
			DeletedDocs:      docCount,
			WorkspaceRemoved: true,
		})
	}
}

var _ RemoveWorkspaceQuerier = (*sqlc.Queries)(nil)
