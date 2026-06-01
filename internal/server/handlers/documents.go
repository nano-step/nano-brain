package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

type DocumentsQuerier interface {
	ListDocumentsByWorkspace(ctx context.Context, workspaceHash string) ([]sqlc.ListDocumentsByWorkspaceRow, error)
	DeleteDocumentByIDAndWorkspace(ctx context.Context, arg sqlc.DeleteDocumentByIDAndWorkspaceParams) (int64, error)
}

// documentListItem matches web/src/api/types.ts:Document interface for the
// fields actually consumed by MemoryPanel. content + metadata are loaded
// on-demand via POST /api/v1/get when a doc is opened in DocDrawer.
// See openspec/specs/documents-list-endpoint for the canonical contract.
type documentListItem struct {
	ID             uuid.UUID `json:"id"`
	Title          string    `json:"title"`
	Collection     string    `json:"collection"`
	SourcePath     string    `json:"source_path"`
	Tags           []string  `json:"tags"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
	SupersedesID   *string   `json:"supersedes_id"`
	SupersededByID *string   `json:"superseded_by_id"`
}

type listDocumentsResponse struct {
	Documents []documentListItem `json:"documents"`
}

func ListDocuments(q DocumentsQuerier, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		workspace, _ := c.Get("workspace").(string)
		if workspace == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "workspace is required")
		}
		ctx := c.Request().Context()

		rows, err := q.ListDocumentsByWorkspace(ctx, workspace)
		if err != nil {
			logger.Error().Err(err).Str("workspace", workspace).Msg("list documents failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to list documents")
		}

		textFilter := strings.ToLower(strings.TrimSpace(c.QueryParam("text")))
		collectionFilter := strings.TrimSpace(c.QueryParam("collection"))
		var tagFilter []string
		if t := strings.TrimSpace(c.QueryParam("tags")); t != "" {
			for _, tag := range strings.Split(t, ",") {
				if tag = strings.TrimSpace(tag); tag != "" {
					tagFilter = append(tagFilter, tag)
				}
			}
		}

		items := make([]documentListItem, 0, len(rows))
		for _, r := range rows {
			if collectionFilter != "" && r.Collection != collectionFilter {
				continue
			}
			if textFilter != "" && !strings.Contains(strings.ToLower(r.Title), textFilter) {
				continue
			}
			if len(tagFilter) > 0 && !anyTagMatches(r.Tags, tagFilter) {
				continue
			}
			items = append(items, toDocumentListItem(r))
		}

		return c.JSON(http.StatusOK, listDocumentsResponse{Documents: items})
	}
}

func DeleteDocument(q DocumentsQuerier, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		workspace, _ := c.Get("workspace").(string)
		if workspace == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "workspace is required")
		}
		idStr := c.Param("id")
		id, err := uuid.Parse(idStr)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid document id")
		}

		rows, err := q.DeleteDocumentByIDAndWorkspace(c.Request().Context(), sqlc.DeleteDocumentByIDAndWorkspaceParams{
			ID:            id,
			WorkspaceHash: workspace,
		})
		if err != nil {
			logger.Error().Err(err).Str("workspace", workspace).Str("id", idStr).Msg("delete document failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to delete document")
		}
		if rows == 0 {
			return echo.NewHTTPError(http.StatusNotFound, "document not found")
		}

		return c.JSON(http.StatusOK, map[string]string{"deleted_id": idStr})
	}
}

func anyTagMatches(docTags, filter []string) bool {
	for _, dt := range docTags {
		for _, ft := range filter {
			if dt == ft {
				return true
			}
		}
	}
	return false
}

func toDocumentListItem(r sqlc.ListDocumentsByWorkspaceRow) documentListItem {
	item := documentListItem{
		ID:         r.ID,
		Title:      r.Title,
		Collection: r.Collection,
		SourcePath: r.SourcePath,
		Tags:       r.Tags,
		CreatedAt:  r.CreatedAt,
		UpdatedAt:  r.UpdatedAt,
	}
	if item.Tags == nil {
		item.Tags = []string{}
	}
	if r.SupersedesID.Valid {
		s := r.SupersedesID.UUID.String()
		item.SupersedesID = &s
	}
	if r.SupersededByID != uuid.Nil {
		s := r.SupersededByID.String()
		item.SupersededByID = &s
	}
	return item
}
