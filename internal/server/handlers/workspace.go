package handlers

import (
	"context"
	"net/http"
	"path/filepath"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/storage"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

type WorkspaceQuerier interface {
	UpsertWorkspace(ctx context.Context, arg sqlc.UpsertWorkspaceParams) (sqlc.Workspace, error)
	UpsertCollection(ctx context.Context, arg sqlc.UpsertCollectionParams) (sqlc.Collection, error)
	ListWorkspaces(ctx context.Context) ([]sqlc.Workspace, error)
	CountDocumentsByWorkspace(ctx context.Context, workspaceHash string) (int64, error)
}

type initRequest struct {
	RootPath string `json:"root_path"`
}

type initResponse struct {
	WorkspaceHash string `json:"workspace_hash"`
	RootPath      string `json:"root_path"`
	AgentsSnippet string `json:"agents_snippet"`
}

type workspaceItem struct {
	WorkspaceHash string    `json:"workspace_hash"`
	RootPath      string    `json:"root_path"`
	Name          string    `json:"name"`
	DocumentCount int64     `json:"document_count"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func InitWorkspace(q WorkspaceQuerier, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		var req initRequest
		if err := c.Bind(&req); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
		}
		if req.RootPath == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "root_path is required")
		}

		absPath, err := filepath.Abs(req.RootPath)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid root_path")
		}

		hash := storage.WorkspaceHash(absPath)
		name := filepath.Base(absPath)

		ws, err := q.UpsertWorkspace(c.Request().Context(), sqlc.UpsertWorkspaceParams{
			Hash: hash,
			Name: name,
			Path: absPath,
		})
		if err != nil {
			logger.Error().Err(err).Str("hash", hash).Msg("upsert workspace failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to register workspace")
		}

		memoryPath := "~/.nano-brain/memory/"
		sessionsPath := "~/.nano-brain/sessions/"

		if _, err := q.UpsertCollection(c.Request().Context(), sqlc.UpsertCollectionParams{
			WorkspaceHash: ws.Hash,
			Name:          "memory",
			Path:          memoryPath,
			GlobPattern:   "**/*",
			UpdateMode:    "auto",
		}); err != nil {
			logger.Error().Err(err).Str("hash", hash).Msg("upsert memory collection failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to create default collections")
		}

		if _, err := q.UpsertCollection(c.Request().Context(), sqlc.UpsertCollectionParams{
			WorkspaceHash: ws.Hash,
			Name:          "sessions",
			Path:          sessionsPath,
			GlobPattern:   "**/*",
			UpdateMode:    "auto",
		}); err != nil {
			logger.Error().Err(err).Str("hash", hash).Msg("upsert sessions collection failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to create default collections")
		}

		snippet := "## nano-brain Access\n\nnano-brain workspace: " + ws.Hash + "\n" +
			"nano-brain is accessed via CLI: `npx nano-brain <command>`."

		return c.JSON(http.StatusOK, initResponse{
			WorkspaceHash: ws.Hash,
			RootPath:      ws.Path,
			AgentsSnippet: snippet,
		})
	}
}

func ListWorkspaces(q WorkspaceQuerier, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		workspaces, err := q.ListWorkspaces(c.Request().Context())
		if err != nil {
			logger.Error().Err(err).Msg("list workspaces failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to list workspaces")
		}

		items := make([]workspaceItem, 0, len(workspaces))
		for _, ws := range workspaces {
			count, err := q.CountDocumentsByWorkspace(c.Request().Context(), ws.Hash)
			if err != nil {
				logger.Error().Err(err).Str("hash", ws.Hash).Msg("count documents failed")
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to count documents")
			}
			items = append(items, workspaceItem{
				WorkspaceHash: ws.Hash,
				RootPath:      ws.Path,
				Name:          ws.Name,
				DocumentCount: count,
				CreatedAt:     ws.CreatedAt,
				UpdatedAt:     ws.UpdatedAt,
			})
		}

		return c.JSON(http.StatusOK, items)
	}
}
