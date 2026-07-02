package handlers

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
)

type HarvestRunner interface {
	RunOnce(ctx context.Context) (harvested, skipped, errCount int)
}

type HarvestResponse struct {
	Harvested int `json:"harvested"`
	Skipped   int `json:"skipped"`
	Errors    int `json:"errors"`
}

// TriggerHarvest godoc
// @Summary      Trigger a session harvest run
// @Description  Runs a single harvest pass (OpenCode/Claude Code session ingestion) synchronously
// @Tags         harvest
// @Produce      json
// @Success      200 {object} HarvestResponse
// @Failure      503 {object} map[string]string
// @Router       /api/harvest [post]
func TriggerHarvest(getRunner func() HarvestRunner) echo.HandlerFunc {
	return func(c echo.Context) error {
		runner := getRunner()
		if runner == nil {
			return echo.NewHTTPError(http.StatusServiceUnavailable, "no harvesters configured")
		}

		harvested, skipped, errCount := runner.RunOnce(c.Request().Context())

		return c.JSON(http.StatusOK, HarvestResponse{
			Harvested: harvested,
			Skipped:   skipped,
			Errors:    errCount,
		})
	}
}
