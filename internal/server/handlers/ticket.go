package handlers

import (
	"context"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/storage"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

// TicketQuerier is the storage interface required by TicketHandler.
type TicketQuerier interface {
	ListDocumentsByTag(ctx context.Context, arg sqlc.ListDocumentsByTagParams) ([]sqlc.ListDocumentsByTagRow, error)
}

// TicketSessionResult is a single session result returned by the ticket endpoint.
type TicketSessionResult struct {
	SessionID     string   `json:"session_id"`
	Title         string   `json:"title"`
	Source        string   `json:"source"`
	WorkspaceHash string   `json:"workspace_hash"`
	SourcePath    string   `json:"source_path"`
	Tags          []string `json:"tags"`
	Snippet       string   `json:"snippet"`
}

const (
	ticketCollection = "sessions"
	ticketMaxLimit   = 50
	snippetMaxLen    = 300
)

// snippet returns up to snippetMaxLen characters of content, trimmed of whitespace.
func snippet(content string) string {
	content = strings.TrimSpace(content)
	runes := []rune(content)
	if len(runes) <= snippetMaxLen {
		return content
	}
	return string(runes[:snippetMaxLen])
}

// TicketHandler returns an echo.HandlerFunc that queries all sessions tagged
// with "ticket:<id>" across ALL workspaces and returns a JSON array of results.
//
// GET /api/v1/sessions/by-ticket?ticket=DEV-4706
// GET /api/v1/sessions/by-ticket?ticket=%2342   (URL-encoded #42)
//
// TicketHandler godoc
// @Summary      Find sessions by ticket ID across all workspaces
// @Description  Queries sessions tagged "ticket:<id>" across all workspaces (not scoped to a single workspace)
// @Tags         tickets
// @Produce      json
// @Success      200 {array} TicketSessionResult
// @Failure      400 {object} map[string]string
// @Failure      500 {object} map[string]string
// @Router       /api/v1/sessions/by-ticket [get]
func TicketHandler(q TicketQuerier, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		ticketParam := strings.TrimSpace(c.QueryParam("ticket"))
		if ticketParam == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "ticket query parameter is required")
		}

		// The write path (harvest/tickets.go) stores ticket IDs uppercased
		// (e.g. "ticket:DEV-4706"); ANY(tags) on a TEXT[] is case-sensitive, so
		// the query input must be uppercased to match. "#42"-style IDs have no
		// letters, so ToUpper is a no-op for them — consistent with the write path.
		tagValue := "ticket:" + strings.ToUpper(ticketParam)

		ctx := c.Request().Context()
		rows, err := q.ListDocumentsByTag(ctx, sqlc.ListDocumentsByTagParams{
			Column1:    tagValue,
			Collection: ticketCollection,
			Limit:      int32(ticketMaxLimit),
		})
		if err != nil {
			logger.Error().Err(err).Str("ticket", ticketParam).Msg("ticket: ListDocumentsByTag failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to query sessions by ticket")
		}

		results := make([]TicketSessionResult, 0, len(rows))
		for _, row := range rows {
			tags := row.Tags
			if tags == nil {
				tags = []string{}
			}
			// Prefer source_path scheme (more precise); fall back to tag scan.
			src := storage.SourceFromPath(row.SourcePath)
			if src == "unknown" {
				src = sourceFromTags(tags)
			}
			results = append(results, TicketSessionResult{
				SessionID:     row.ID.String(),
				Title:         row.Title,
				Source:        src,
				WorkspaceHash: row.WorkspaceHash,
				SourcePath:    row.SourcePath,
				Tags:          tags,
				Snippet:       snippet(row.Content),
			})
		}

		return c.JSON(http.StatusOK, results)
	}
}
