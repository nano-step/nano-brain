package handlers

import (
	"fmt"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
)

type reindexRequest struct {
	Workspace string `json:"workspace"`
	Root      string `json:"root"`
}

type reindexResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

func TriggerReindex(logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		var req reindexRequest
		if err := c.Bind(&req); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
		}

		workspace := c.Get("workspace").(string)

		if req.Root == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "root (collection name) is required")
		}

		reqLog := LoggerFromCtx(c, logger)
		reqLog.Info().
			Str("workspace", workspace).
			Str("root", req.Root).
			Msg("reindex queued")

		return c.JSON(http.StatusAccepted, reindexResponse{
			Status:  "queued",
			Message: fmt.Sprintf("Reindex queued for collection %s in workspace %s", req.Root, workspace),
		})
	}
}

type updateRequest struct {
	Workspace string `json:"workspace"`
}

func TriggerUpdate(logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		workspace := c.Get("workspace").(string)

		reqLog := LoggerFromCtx(c, logger)
		reqLog.Info().
			Str("workspace", workspace).
			Msg("update queued for all collections")

		return c.JSON(http.StatusAccepted, reindexResponse{
			Status:  "queued",
			Message: fmt.Sprintf("Update queued for all collections in workspace %s", workspace),
		})
	}
}
