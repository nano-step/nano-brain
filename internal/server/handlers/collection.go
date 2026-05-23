package handlers

import (
	"context"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/watcher"
	"github.com/rs/zerolog"
)

type CollectionQuerier interface {
	UpsertCollection(ctx context.Context, arg sqlc.UpsertCollectionParams) (sqlc.Collection, error)
	ListCollections(ctx context.Context, workspaceHash string) ([]sqlc.Collection, error)
	GetCollectionByName(ctx context.Context, arg sqlc.GetCollectionByNameParams) (sqlc.Collection, error)
	RenameCollection(ctx context.Context, arg sqlc.RenameCollectionParams) (sqlc.Collection, error)
	DeleteCollection(ctx context.Context, arg sqlc.DeleteCollectionParams) error
	CountDocumentsByCollection(ctx context.Context, arg sqlc.CountDocumentsByCollectionParams) (int64, error)
}

type AddCollectionRequest struct {
	Workspace   string `json:"workspace"`
	Name        string `json:"name"`
	Path        string `json:"path"`
	GlobPattern string `json:"glob_pattern"`
}

type RenameCollectionRequest struct {
	Workspace string `json:"workspace"`
	NewName   string `json:"new_name"`
}

type CollectionResponse struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Path          string `json:"path"`
	GlobPattern   string `json:"glob_pattern"`
	UpdateMode    string `json:"update_mode"`
	DocumentCount int64  `json:"document_count"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

func toCollectionResponse(col sqlc.Collection, docCount int64) CollectionResponse {
	return CollectionResponse{
		ID:            col.ID.String(),
		Name:          col.Name,
		Path:          col.Path,
		GlobPattern:   col.GlobPattern,
		UpdateMode:    col.UpdateMode,
		DocumentCount: docCount,
		CreatedAt:     col.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:     col.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func AddCollection(q CollectionQuerier, fw *watcher.Watcher, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		var req AddCollectionRequest
		if err := c.Bind(&req); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
		}

		workspace, _ := c.Get("workspace").(string)
		if workspace == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "workspace is required")
		}

		if req.Name == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "name is required")
		}
		if req.Path == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "path is required")
		}

		if _, err := os.Stat(req.Path); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "path does not exist on filesystem")
		}

		globPattern := req.GlobPattern
		if globPattern == "" {
			globPattern = "**/*"
		}

		col, err := q.UpsertCollection(c.Request().Context(), sqlc.UpsertCollectionParams{
			WorkspaceHash: workspace,
			Name:          req.Name,
			Path:          req.Path,
			GlobPattern:   globPattern,
			UpdateMode:    "auto",
		})
		if err != nil {
			logger.Error().Err(err).Str("name", req.Name).Msg("upsert collection failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to add collection")
		}

		if fw != nil {
			if err := fw.Watch(col.Name, col.Path, col.WorkspaceHash, col.GlobPattern); err != nil {
				logger.Warn().Err(err).Str("name", col.Name).Msg("failed to attach watcher")
			}
		}

		return c.JSON(http.StatusCreated, toCollectionResponse(col, 0))
	}
}

func ListCollectionsHandler(q CollectionQuerier, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		workspace := c.QueryParam("workspace")
		if workspace == "" {
			workspace, _ = c.Get("workspace").(string)
		}
		if workspace == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "workspace is required")
		}

		cols, err := q.ListCollections(c.Request().Context(), workspace)
		if err != nil {
			logger.Error().Err(err).Str("workspace", workspace).Msg("list collections failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to list collections")
		}

		items := make([]CollectionResponse, 0, len(cols))
		for _, col := range cols {
			count, err := q.CountDocumentsByCollection(c.Request().Context(), sqlc.CountDocumentsByCollectionParams{
				Collection:    col.Name,
				WorkspaceHash: workspace,
			})
			if err != nil {
				logger.Error().Err(err).Str("collection", col.Name).Msg("count documents failed")
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to count documents")
			}
			items = append(items, toCollectionResponse(col, count))
		}

		return c.JSON(http.StatusOK, items)
	}
}

func RenameCollectionHandler(q CollectionQuerier, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		name := c.Param("name")
		if name == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "collection name is required")
		}

		var req RenameCollectionRequest
		if err := c.Bind(&req); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
		}

		workspace, _ := c.Get("workspace").(string)
		if workspace == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "workspace is required")
		}

		if req.NewName == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "new_name is required")
		}

		col, err := q.RenameCollection(c.Request().Context(), sqlc.RenameCollectionParams{
			Name:          name,
			Name_2:        req.NewName,
			WorkspaceHash: workspace,
		})
		if err != nil {
			logger.Error().Err(err).Str("old", name).Str("new", req.NewName).Msg("rename collection failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to rename collection")
		}

		return c.JSON(http.StatusOK, toCollectionResponse(col, 0))
	}
}

func RemoveCollection(q CollectionQuerier, fw *watcher.Watcher, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		name := c.Param("name")
		if name == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "collection name is required")
		}

		workspace, _ := c.Get("workspace").(string)
		if workspace == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "workspace is required")
		}

		col, err := q.GetCollectionByName(c.Request().Context(), sqlc.GetCollectionByNameParams{
			Name:          name,
			WorkspaceHash: workspace,
		})
		if err != nil {
			logger.Error().Err(err).Str("name", name).Msg("get collection failed")
			return echo.NewHTTPError(http.StatusNotFound, "collection not found")
		}

		if err := q.DeleteCollection(c.Request().Context(), sqlc.DeleteCollectionParams{
			Name:          name,
			WorkspaceHash: workspace,
		}); err != nil {
			logger.Error().Err(err).Str("name", name).Msg("delete collection failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to remove collection")
		}

		if fw != nil {
			if err := fw.Unwatch(col.Path); err != nil {
				logger.Warn().Err(err).Str("path", col.Path).Msg("failed to detach watcher")
			}
		}

		return c.NoContent(http.StatusNoContent)
	}
}
