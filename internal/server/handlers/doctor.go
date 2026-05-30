package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/health/doctor"
	"github.com/rs/zerolog"
)

// DoctorDeps provides the dependencies for the doctor handler.
type DoctorDeps struct {
	ConfigPath string
	LoadConfig func() (*config.Config, error)
}

// Doctor returns the prerequisite checks as JSON.
func Doctor(deps DoctorDeps, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		cfg, cfgErr := deps.LoadConfig()
		checks := doctor.RunAll(deps.ConfigPath, cfg, cfgErr)

		allPassed := true
		for _, ch := range checks {
			if ch.Status == "fail" {
				allPassed = false
				break
			}
		}

		return c.JSON(http.StatusOK, map[string]interface{}{
			"checks":     checks,
			"all_passed": allPassed,
		})
	}
}
