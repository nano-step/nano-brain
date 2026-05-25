package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/rs/zerolog"
)

type reloadResponse struct {
	Reloaded        []string `json:"reloaded"`
	Unchanged       []string `json:"unchanged"`
	RequiresRestart []string `json:"requires_restart"`
}

// ReloadConfig returns a handler that re-reads the YAML config and applies reloadable settings.
func ReloadConfig(configPath string, currentCfg func() *config.Config, applyCfg func(*config.Config, *config.ReloadResult), logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		cur := currentCfg()

		newCfg, result, err := config.Reload(configPath, cur)
		if err != nil {
			logger.Warn().Err(err).Msg("config reload failed")
			return c.JSON(http.StatusBadRequest, map[string]string{"error": err.Error()})
		}

		if len(result.Reloaded) > 0 {
			applyCfg(newCfg, result)
			logger.Info().Strs("reloaded", result.Reloaded).Msg("config reloaded")
		}

		if len(result.RequiresRestart) > 0 {
			logger.Warn().Strs("requires_restart", result.RequiresRestart).Msg("some settings require restart")
		}

		return c.JSON(http.StatusOK, reloadResponse{
			Reloaded:        result.Reloaded,
			Unchanged:       result.Unchanged,
			RequiresRestart: result.RequiresRestart,
		})
	}
}
