package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

type DocumentsQuerier interface {
	ListDocumentsByWorkspace(ctx context.Context, workspaceHash string) ([]sqlc.ListDocumentsByWorkspaceRow, error)
	ListDocumentsByWorkspacePaginated(ctx context.Context, arg sqlc.ListDocumentsByWorkspacePaginatedParams) ([]sqlc.ListDocumentsByWorkspacePaginatedRow, error)
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
	Documents  []documentListItem `json:"documents"`
	NextCursor *string            `json:"next_cursor,omitempty"`
}

func ListDocuments(q DocumentsQuerier, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		workspace, _ := c.Get("workspace").(string)
		if workspace == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "workspace is required")
		}
		ctx := c.Request().Context()

		cursorStr := strings.TrimSpace(c.QueryParam("cursor"))
		limitStr := strings.TrimSpace(c.QueryParam("limit"))

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

		limit := int32(100)
		if limitStr != "" {
			n, err := strconv.ParseInt(limitStr, 10, 32)
			if err != nil || n < 1 || n > 500 {
				return echo.NewHTTPError(http.StatusBadRequest, "invalid limit (1-500)")
			}
			limit = int32(n)
		}

		if cursorStr == "" {
			rows, err := q.ListDocumentsByWorkspace(ctx, workspace)
			if err != nil {
				logger.Error().Err(err).Str("workspace", workspace).Msg("list documents failed")
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to list documents")
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
			items = append(items, toDocumentItemFromRow(r))
		}

		return c.JSON(http.StatusOK, listDocumentsResponse{Documents: items})
	}

		cursorParts := strings.SplitN(cursorStr, ",", 2)
		if len(cursorParts) != 2 {
			return echo.NewHTTPError(http.StatusBadRequest, "cursor must be <timestamp>,<id>")
		}
		cursorTS, err := time.Parse(time.RFC3339Nano, cursorParts[0])
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid cursor timestamp")
		}
		cursorID, err := uuid.Parse(cursorParts[1])
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid cursor id")
		}

		rows, err := q.ListDocumentsByWorkspacePaginated(ctx, sqlc.ListDocumentsByWorkspacePaginatedParams{
			WorkspaceHash: workspace,
			Column2:       cursorTS,
			Column3:       cursorID,
			Column4:       false,
			Limit:         limit,
		})
		if err != nil {
			logger.Error().Err(err).Str("workspace", workspace).Msg("list documents paginated failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to list documents")
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
			items = append(items, toDocumentItemFromPaginatedRow(r))
		}

		var nextCursor *string
		if len(items) > 0 {
			last := items[len(items)-1]
			s := fmt.Sprintf("%s,%s", last.UpdatedAt.Format(time.RFC3339Nano), last.ID.String())
			nextCursor = &s
		}

		return c.JSON(http.StatusOK, listDocumentsResponse{Documents: items, NextCursor: nextCursor})
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

func toDocumentListItem(
	id uuid.UUID,
	title, collection, sourcePath string,
	tags []string,
	createdAt, updatedAt time.Time,
	supersedesID uuid.NullUUID,
	supersededByID uuid.UUID,
) documentListItem {
	item := documentListItem{
		ID:         id,
		Title:      title,
		Collection: collection,
		SourcePath: sourcePath,
		Tags:       tags,
		CreatedAt:  createdAt,
		UpdatedAt:  updatedAt,
	}
	if item.Tags == nil {
		item.Tags = []string{}
	}
	if supersedesID.Valid {
		s := supersedesID.UUID.String()
		item.SupersedesID = &s
	}
	if supersededByID != uuid.Nil {
		s := supersededByID.String()
		item.SupersededByID = &s
	}
	return item
}

func toDocumentItemFromRow(r sqlc.ListDocumentsByWorkspaceRow) documentListItem {
	return toDocumentListItem(r.ID, r.Title, r.Collection, r.SourcePath, r.Tags, r.CreatedAt, r.UpdatedAt, r.SupersedesID, r.SupersededByID)
}

func toDocumentItemFromPaginatedRow(r sqlc.ListDocumentsByWorkspacePaginatedRow) documentListItem {
	return toDocumentListItem(r.ID, r.Title, r.Collection, r.SourcePath, r.Tags, r.CreatedAt, r.UpdatedAt, r.SupersedesID, r.SupersededByID)
}
