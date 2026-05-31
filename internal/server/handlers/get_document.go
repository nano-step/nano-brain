package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

type GetDocumentQuerier interface {
	GetDocumentByID(ctx context.Context, arg sqlc.GetDocumentByIDParams) (sqlc.Document, error)
	GetDocumentBySourcePath(ctx context.Context, arg sqlc.GetDocumentBySourcePathParams) (sqlc.Document, error)
}

type getDocumentRequest struct {
	Path string `json:"path"`
	ID   string `json:"id"`
}

type getDocumentResponse struct {
	ID            string   `json:"id"`
	Title         string   `json:"title"`
	Content       string   `json:"content"`
	SourcePath    string   `json:"source_path"`
	Collection    string   `json:"collection"`
	Tags          []string `json:"tags"`
	WorkspaceHash string   `json:"workspace_hash"`
	SupersedesID  string   `json:"supersedes_id,omitempty"`
	CreatedAt     string   `json:"created_at"`
	UpdatedAt     string   `json:"updated_at"`
}

func GetDocument(q GetDocumentQuerier, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		workspace := c.Get("workspace").(string)

		var req getDocumentRequest
		if err := c.Bind(&req); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
		}

		lookupPath := strings.TrimSpace(req.Path)
		lookupID := strings.TrimSpace(req.ID)

		if lookupPath == "" && lookupID == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "path or id is required")
		}

		var doc sqlc.Document
		var err error

		switch {
		case strings.HasPrefix(lookupPath, "#"):
			parsed, parseErr := uuid.Parse(strings.TrimPrefix(lookupPath, "#"))
			if parseErr != nil {
				return echo.NewHTTPError(http.StatusBadRequest, "invalid document ID in path")
			}
			doc, err = q.GetDocumentByID(c.Request().Context(), sqlc.GetDocumentByIDParams{
				ID:            parsed,
				WorkspaceHash: workspace,
			})
		case lookupPath != "":
			doc, err = q.GetDocumentBySourcePath(c.Request().Context(), sqlc.GetDocumentBySourcePathParams{
				SourcePath:    lookupPath,
				WorkspaceHash: workspace,
			})
		default:
			parsed, parseErr := uuid.Parse(lookupID)
			if parseErr != nil {
				return echo.NewHTTPError(http.StatusBadRequest, "invalid document id")
			}
			doc, err = q.GetDocumentByID(c.Request().Context(), sqlc.GetDocumentByIDParams{
				ID:            parsed,
				WorkspaceHash: workspace,
			})
		}

		if err != nil {
			if isNotFound(err) {
				return echo.NewHTTPError(http.StatusNotFound, "document not found")
			}
			logger.Error().Err(err).Str("workspace", workspace).Msg("get document failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to get document")
		}

		supersedes := ""
		if doc.SupersedesID.Valid {
			supersedes = doc.SupersedesID.UUID.String()
		}

		tags := doc.Tags
		if tags == nil {
			tags = []string{}
		}

		return c.JSON(http.StatusOK, getDocumentResponse{
			ID:            doc.ID.String(),
			Title:         doc.Title,
			Content:       doc.Content,
			SourcePath:    doc.SourcePath,
			Collection:    doc.Collection,
			Tags:          tags,
			WorkspaceHash: doc.WorkspaceHash,
			SupersedesID:  supersedes,
			CreatedAt:     doc.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt:     doc.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, sql.ErrNoRows)
}
