package handlers

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

type MultiGetQuerier interface {
	GetDocumentByID(ctx context.Context, arg sqlc.GetDocumentByIDParams) (sqlc.Document, error)
	GetDocumentBySourcePath(ctx context.Context, arg sqlc.GetDocumentBySourcePathParams) (sqlc.Document, error)
}

type multiGetRequest struct {
	Paths []string `json:"paths"`
	IDs   []string `json:"ids"`
}

type multiGetResponse struct {
	Results  []getDocumentResponse `json:"results"`
	NotFound []string              `json:"not_found"`
}

// MultiGet godoc
// @Summary      Get multiple documents in one call
// @Description  Fetches multiple documents by source paths and/or IDs within the workspace
// @Tags         documents
// @Accept       json
// @Produce      json
// @Param        request body multiGetRequest true "Paths and/or IDs to look up"
// @Success      200 {object} multiGetResponse
// @Failure      400 {object} map[string]string
// @Failure      500 {object} map[string]string
// @Security     WorkspaceAuth
// @Router       /api/v1/multi-get [post]
func MultiGet(q MultiGetQuerier, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		workspace := c.Get("workspace").(string)

		var req multiGetRequest
		if err := c.Bind(&req); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
		}

		if len(req.Paths) == 0 && len(req.IDs) == 0 {
			return echo.NewHTTPError(http.StatusBadRequest, "paths or ids is required")
		}

		ctx := c.Request().Context()
		results := make([]getDocumentResponse, 0)
		notFound := make([]string, 0)

		for _, p := range req.Paths {
			doc, err := q.GetDocumentBySourcePath(ctx, sqlc.GetDocumentBySourcePathParams{
				SourcePath:    p,
				WorkspaceHash: workspace,
			})
			if err != nil {
				if isNotFound(err) {
					notFound = append(notFound, p)
					continue
				}
				logger.Error().Err(err).Str("workspace", workspace).Str("path", p).Msg("multi-get path failed")
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to get document")
			}
			results = append(results, docToResponse(doc))
		}

		for _, rawID := range req.IDs {
			parsed, parseErr := uuid.Parse(rawID)
			if parseErr != nil {
				notFound = append(notFound, rawID)
				continue
			}
			doc, err := q.GetDocumentByID(ctx, sqlc.GetDocumentByIDParams{
				ID:            parsed,
				WorkspaceHash: workspace,
			})
			if err != nil {
				if isNotFound(err) {
					notFound = append(notFound, rawID)
					continue
				}
				logger.Error().Err(err).Str("workspace", workspace).Str("id", rawID).Msg("multi-get id failed")
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to get document")
			}
			results = append(results, docToResponse(doc))
		}

		return c.JSON(http.StatusOK, multiGetResponse{
			Results:  results,
			NotFound: notFound,
		})
	}
}

func docToResponse(doc sqlc.Document) getDocumentResponse {
	supersedes := ""
	if doc.SupersedesID.Valid {
		supersedes = doc.SupersedesID.UUID.String()
	}
	tags := doc.Tags
	if tags == nil {
		tags = []string{}
	}
	return getDocumentResponse{
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
	}
}
