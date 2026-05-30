package handlers

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/rs/zerolog"
)

// GetConfig returns the current resolved config with secrets redacted.
func GetConfig(cfgPath string, currentCfg func() *config.Config, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		cfg := currentCfg()
		redacted := config.RedactSecrets(cfg)
		resp := map[string]interface{}{
			"config": redacted,
		}
		if c.QueryParam("include_source") == "true" {
			resp["source"] = cfgPath
		}
		return c.JSON(http.StatusOK, resp)
	}
}

// PatchConfig applies a single-field patch to the config YAML and triggers reload.
func PatchConfig(cfgPath string, currentCfg func() *config.Config, reload func(), logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		var req config.PatchRequest
		if err := c.Bind(&req); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
		}
		if req.Path == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "path is required")
		}

		if config.IsSecretFieldPath(req.Path) {
			return echo.NewHTTPError(http.StatusBadRequest, "cannot patch secret field: "+req.Path)
		}
		if !config.IsPatchableFieldPath(req.Path) {
			return echo.NewHTTPError(http.StatusUnprocessableEntity, "field not patchable: "+req.Path)
		}

		if err := config.ApplyPatch(cfgPath, req); err != nil {
			reqLog := LoggerFromCtx(c, logger)
			reqLog.Warn().Err(err).Str("path", req.Path).Msg("config patch failed")
			return echo.NewHTTPError(http.StatusUnprocessableEntity, err.Error())
		}

		reload()

		reqLog := LoggerFromCtx(c, logger)
		reqLog.Info().Str("path", req.Path).Msg("config patched")
		return c.JSON(http.StatusOK, map[string]string{
			"status": "patched",
			"path":   req.Path,
		})
	}
}
