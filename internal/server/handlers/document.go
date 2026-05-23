package handlers

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
	"github.com/sqlc-dev/pqtype"
)

type DocumentQuerier interface {
	UpsertDocument(ctx context.Context, arg sqlc.UpsertDocumentParams) (sqlc.UpsertDocumentRow, error)
}

type WriteRequest struct {
	Content    string          `json:"content"`
	Tags       []string        `json:"tags"`
	Collection string          `json:"collection"`
	Title      string          `json:"title"`
	SourcePath string          `json:"source_path"`
	Metadata   json.RawMessage `json:"metadata"`
}

type WriteResponse struct {
	ID            string `json:"id"`
	Hash          string `json:"hash"`
	Collection    string `json:"collection"`
	WorkspaceHash string `json:"workspace_hash"`
}

func WriteDocument(q DocumentQuerier, db *sql.DB, logger zerolog.Logger, maxFileSize int64) echo.HandlerFunc {
	return func(c echo.Context) error {
		var req WriteRequest
		if err := c.Bind(&req); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
		}

		workspace, _ := c.Get("workspace").(string)
		if workspace == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "workspace is required")
		}

		if req.Content == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "content is required")
		}

		if int64(len(req.Content)) > maxFileSize {
			return echo.NewHTTPError(http.StatusBadRequest, "content exceeds maximum allowed size")
		}

		sum := sha256.Sum256([]byte(req.Content))
		contentHash := hex.EncodeToString(sum[:])

	// Default to "memory" collection for API writes. The schema default is "default"
	// but the handler always sets collection explicitly.
	collection := req.Collection
	if collection == "" {
		collection = "memory"
	}

	tags := req.Tags
	if tags == nil {
		tags = []string{}
	}

	// Default to empty JSON object for metadata. If provided by user, use it.
	meta := pqtype.NullRawMessage{RawMessage: []byte(`{}`), Valid: true}
	if len(req.Metadata) > 0 {
		meta = pqtype.NullRawMessage{RawMessage: req.Metadata, Valid: true}
	}

	params := sqlc.UpsertDocumentParams{
		WorkspaceHash: workspace,
		ContentHash:   contentHash,
		Title:         req.Title,
		Content:       req.Content,
		SourcePath:    req.SourcePath,
		Collection:    collection,
		Tags:          tags,
		Metadata:      meta,
	}

		var row sqlc.UpsertDocumentRow
		if db != nil {
			tx, err := db.BeginTx(c.Request().Context(), nil)
			if err != nil {
				logger.Error().Err(err).Msg("begin transaction failed")
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to write document")
			}
			row, err = sqlc.New(tx).UpsertDocument(c.Request().Context(), params)
			if err != nil {
				_ = tx.Rollback()
				logger.Error().Err(err).Msg("upsert document failed")
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to write document")
			}
			if err := tx.Commit(); err != nil {
				logger.Error().Err(err).Msg("commit transaction failed")
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to write document")
			}
		} else {
			var err error
			row, err = q.UpsertDocument(c.Request().Context(), params)
			if err != nil {
				logger.Error().Err(err).Msg("upsert document failed")
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to write document")
			}
		}

		return c.JSON(http.StatusCreated, WriteResponse{
			ID:            row.ID.String(),
			Hash:          row.ContentHash,
			Collection:    row.Collection,
			WorkspaceHash: row.WorkspaceHash,
		})
	}
}
