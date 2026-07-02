package handlers

import (
	"context"
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

type TagQuerier interface {
	ListTagsByWorkspace(ctx context.Context, workspaceHash string) ([]sqlc.ListTagsByWorkspaceRow, error)
}

type tagItem struct {
	Tag   string `json:"tag"`
	Count int64  `json:"count"`
}

// ListTags godoc
// @Summary      List tags in a workspace
// @Description  Returns all tags used in the workspace with document counts per tag
// @Tags         tags
// @Produce      json
// @Success      200 {array} tagItem
// @Failure      500 {object} map[string]string
// @Security     WorkspaceAuth
// @Router       /api/v1/tags [get]
func ListTags(q TagQuerier, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		workspace := c.Get("workspace").(string)

		rows, err := q.ListTagsByWorkspace(c.Request().Context(), workspace)
		if err != nil {
			logger.Error().Err(err).Str("workspace", workspace).Msg("list tags failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to list tags")
		}

		items := make([]tagItem, 0, len(rows))
		for _, r := range rows {
			items = append(items, tagItem{
				Tag:   fmt.Sprint(r.Tag),
				Count: r.Count,
			})
		}

		return c.JSON(http.StatusOK, items)
	}
}
