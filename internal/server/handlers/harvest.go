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
