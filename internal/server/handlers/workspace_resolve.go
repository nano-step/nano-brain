package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"path/filepath"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/storage"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

type WorkspaceResolver interface {
	GetWorkspaceByHash(ctx context.Context, hash string) (sqlc.Workspace, error)
}

type workspaceResolveRequest struct {
	Path string `json:"path"`
}

type WorkspaceResolveResponse struct {
	WorkspaceHash string `json:"workspace_hash"`
	RootPath      string `json:"root_path"`
	Name          string `json:"name"`
	Registered    bool   `json:"registered"`
}

func ResolveWorkspace(q WorkspaceResolver, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		var req workspaceResolveRequest
		if err := c.Bind(&req); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
		}
		resp, err := ResolveWorkspacePath(c.Request().Context(), q, req.Path, logger)
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, resp)
	}
}

func ResolveWorkspacePath(ctx context.Context, q WorkspaceResolver, path string, logger zerolog.Logger) (WorkspaceResolveResponse, error) {
	if path == "" {
		return WorkspaceResolveResponse{}, echo.NewHTTPError(http.StatusBadRequest, "path is required")
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return WorkspaceResolveResponse{}, echo.NewHTTPError(http.StatusBadRequest, "invalid path")
	}
	hash, err := storage.WorkspaceHash(absPath)
	if err != nil {
		return WorkspaceResolveResponse{}, echo.NewHTTPError(http.StatusBadRequest, "invalid path")
	}

	ws, err := q.GetWorkspaceByHash(ctx, hash)
	if err == nil {
		return WorkspaceResolveResponse{
			WorkspaceHash: ws.Hash,
			RootPath:      ws.Path,
			Name:          ws.Name,
			Registered:    true,
		}, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return WorkspaceResolveResponse{
			WorkspaceHash: hash,
			RootPath:      absPath,
			Name:          filepath.Base(absPath),
			Registered:    false,
		}, nil
	}
	logger.Error().Err(err).Str("hash", hash).Msg("resolve workspace lookup failed")
	return WorkspaceResolveResponse{}, echo.NewHTTPError(http.StatusInternalServerError, "failed to resolve workspace")
}
