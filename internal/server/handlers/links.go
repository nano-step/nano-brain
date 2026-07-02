package handlers

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

// BacklinksQuerier is the DB interface for backlink queries.
type BacklinksQuerier interface {
	ListBacklinksByTarget(ctx context.Context, arg sqlc.ListBacklinksByTargetParams) ([]sqlc.ListBacklinksByTargetRow, error)
	CountBacklinksByTarget(ctx context.Context, arg sqlc.CountBacklinksByTargetParams) (int64, error)
}

type backlinkItem struct {
	ID         uuid.UUID `json:"id"`
	Title      string    `json:"title"`
	Collection string    `json:"collection"`
	UpdatedAt  time.Time `json:"updated_at"`
	Tags       []string  `json:"tags"`
	Snippet    string    `json:"snippet"`
}

type backlinksResponse struct {
	Total int64          `json:"total"`
	Items []backlinkItem `json:"items"`
	DocID string         `json:"doc_id"`
}

// Backlinks returns documents that link to the given doc_id.
//
// @Summary      List documents that link to a document
// @Description  Returns documents that link to the given doc_id, paginated
// @Tags         links
// @Produce      json
// @Param        doc_id path string true "Target document ID"
// @Param        limit query int false "Max results (default 20, max 100)"
// @Param        offset query int false "Result offset"
// @Success      200 {object} backlinksResponse
// @Failure      400 {object} map[string]string
// @Security     WorkspaceAuth
// @Router       /api/v1/links/{doc_id}/backlinks [get]
func Backlinks(q BacklinksQuerier, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		docID := c.Param("doc_id")
		if docID == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "doc_id is required")
		}
		workspace := c.Get("workspace").(string)

		limit := 20
		if l := c.QueryParam("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
				limit = n
			}
		}
		offset := 0
		if o := c.QueryParam("offset"); o != "" {
			if n, err := strconv.Atoi(o); err == nil && n >= 0 {
				offset = n
			}
		}

		ctx := c.Request().Context()

		total, err := q.CountBacklinksByTarget(ctx, sqlc.CountBacklinksByTargetParams{
			WorkspaceHash: workspace,
			TargetNode:    docID,
		})
		if err != nil {
			logger.Error().Err(err).Msg("backlinks count failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "backlinks query failed")
		}

		rows, err := q.ListBacklinksByTarget(ctx, sqlc.ListBacklinksByTargetParams{
			WorkspaceHash: workspace,
			TargetNode:    docID,
			Limit:         int32(limit),
			Offset:        int32(offset),
		})
		if err != nil {
			logger.Error().Err(err).Msg("backlinks list failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "backlinks query failed")
		}

		items := make([]backlinkItem, 0, len(rows))
		for _, r := range rows {
			tags := r.Tags
			if tags == nil {
				tags = []string{}
			}
			items = append(items, backlinkItem{
				ID:         r.ID,
				Title:      r.Title,
				Collection: r.Collection,
				UpdatedAt:  r.UpdatedAt,
				Tags:       tags,
				Snippet:    extractSnippet(r.Content, docID, 100),
			})
		}

		return c.JSON(http.StatusOK, backlinksResponse{
			Total: total,
			Items: items,
			DocID: docID,
		})
	}
}

func extractSnippet(content, target string, radius int) string {
	lower := strings.ToLower(content)
	lowerTarget := strings.ToLower(target)

	patterns := []string{
		"[[" + lowerTarget + "]]",
		lowerTarget,
	}

	idx := -1
	for _, p := range patterns {
		idx = strings.Index(lower, p)
		if idx >= 0 {
			break
		}
	}
	if idx < 0 {
		if len(content) <= radius*2 {
			return content
		}
		return content[:radius*2] + "..."
	}

	start := idx - radius
	if start < 0 {
		start = 0
	}
	end := idx + len(target) + radius
	if end > len(content) {
		end = len(content)
	}
	snippet := content[start:end]
	if start > 0 {
		snippet = "..." + snippet
	}
	if end < len(content) {
		snippet = snippet + "..."
	}
	return snippet
}

// LinkQueryResolver is the interface for resolving document IDs and titles.
type LinkQueryResolver interface {
	ResolveID(ctx context.Context, workspace string, id uuid.UUID) (bool, error)
	ResolveTitle(ctx context.Context, workspace, title string) ([]uuid.UUID, error)
}

type resolveResponse struct {
	Results []resolveResult `json:"results"`
	Query   string          `json:"query"`
}

type resolveResult struct {
	ID    uuid.UUID `json:"id"`
	Match string    `json:"match"`
}

// ResolveLink wraps a LinkResolver to resolve titles or IDs via HTTP.
//
// @Summary      Resolve a link query to document IDs
// @Description  Resolves a query string as either a document UUID or a title match
// @Tags         links
// @Produce      json
// @Param        query query string true "Document ID or title to resolve"
// @Success      200 {object} resolveResponse
// @Failure      400 {object} map[string]string
// @Security     WorkspaceAuth
// @Router       /api/v1/links/resolve [get]
func ResolveLink(resolver LinkQueryResolver, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		workspace := c.Get("workspace").(string)
		query := c.QueryParam("query")
		if query == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "query is required")
		}

		ctx := c.Request().Context()
		var results []resolveResult

		if uid, err := uuid.Parse(query); err == nil {
			exists, err := resolver.ResolveID(ctx, workspace, uid)
			if err != nil {
				logger.Error().Err(err).Msg("resolve ID failed")
				return echo.NewHTTPError(http.StatusInternalServerError, "resolve failed")
			}
			if exists {
				results = append(results, resolveResult{ID: uid, Match: "id"})
			}
		} else {
			ids, err := resolver.ResolveTitle(ctx, workspace, query)
			if err != nil {
				logger.Error().Err(err).Msg("resolve title failed")
				return echo.NewHTTPError(http.StatusInternalServerError, "resolve failed")
			}
			for _, id := range ids {
				results = append(results, resolveResult{ID: id, Match: "title"})
			}
		}

		if results == nil {
			results = []resolveResult{}
		}

		return c.JSON(http.StatusOK, resolveResponse{
			Results: results,
			Query:   query,
		})
	}
}
