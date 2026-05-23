package handlers

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
)

type HarvestRunner interface {
	RunOnce(ctx context.Context) (harvested, skipped, errCount int)
}

type HarvestResponse struct {
	Harvested int `json:"harvested"`
	Skipped   int `json:"skipped"`
	Errors    int `json:"errors"`
}

func TriggerHarvest(runnerPtr *HarvestRunner, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		if runnerPtr == nil || *runnerPtr == nil {
			return echo.NewHTTPError(http.StatusServiceUnavailable, "no harvesters configured")
		}

		harvested, skipped, errCount := (*runnerPtr).RunOnce(c.Request().Context())

		return c.JSON(http.StatusOK, HarvestResponse{
			Harvested: harvested,
			Skipped:   skipped,
			Errors:    errCount,
		})
	}
}
