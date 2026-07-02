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
	BinaryPath string
	LoadConfig func() (*config.Config, error)
}

// Doctor godoc
// @Summary      Run environment/prerequisite checks
// @Description  Runs config and binary prerequisite checks and reports pass/fail status for each
// @Tags         doctor
// @Produce      json
// @Success      200 {object} map[string]interface{}
// @Router       /api/v1/doctor [get]
func Doctor(deps DoctorDeps, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		cfg, cfgErr := deps.LoadConfig()
		checks := doctor.RunAll(deps.ConfigPath, cfg, cfgErr, deps.BinaryPath)

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
