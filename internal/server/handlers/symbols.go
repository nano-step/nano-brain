package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

type SymbolQuerier interface {
	ListSymbolsByWorkspace(ctx context.Context, arg sqlc.ListSymbolsByWorkspaceParams) ([]sqlc.ListSymbolsByWorkspaceRow, error)
}

type symbolItem struct {
	Name       string `json:"name"`
	Kind       string `json:"kind"`
	Language   string `json:"language"`
	Signature  string `json:"signature"`
	SourcePath string `json:"source_path"`
}

func ListSymbols(q SymbolQuerier, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		workspace := c.Get("workspace").(string)
		query := c.QueryParam("query")
		kind := c.QueryParam("kind")
		limitStr := c.QueryParam("limit")
		limit := int32(50)
		if limitStr != "" {
			if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
				limit = int32(v)
			}
		}

		rows, err := q.ListSymbolsByWorkspace(c.Request().Context(), sqlc.ListSymbolsByWorkspaceParams{
			WorkspaceHash: workspace,
			Column2:       query,
			Column3:       kind,
			Limit:         limit,
		})
		if err != nil {
			logger.Error().Err(err).Str("workspace", workspace).Msg("list symbols failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to list symbols")
		}

		items := make([]symbolItem, 0, len(rows))
		for _, r := range rows {
			item := symbolItem{
				Name:       r.Title,
				SourcePath: r.SourcePath,
			}
			if r.Metadata.Valid {
				var meta map[string]string
				if err := json.Unmarshal(r.Metadata.RawMessage, &meta); err == nil {
					item.Kind = meta["kind"]
					item.Language = meta["language"]
					item.Signature = meta["signature"]
				}
			}
			items = append(items, item)
		}
		return c.JSON(http.StatusOK, map[string]any{
			"symbols": items,
			"count":   len(items),
		})
	}
}
