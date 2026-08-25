package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/storage"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/nano-brain/nano-brain/internal/watcher"
	"github.com/rs/zerolog"
)

type WorkspaceQuerier interface {
	UpsertWorkspace(ctx context.Context, arg sqlc.UpsertWorkspaceParams) (sqlc.Workspace, error)
	UpsertCollection(ctx context.Context, arg sqlc.UpsertCollectionParams) (sqlc.Collection, error)
	ListWorkspacesWithStats(ctx context.Context) ([]sqlc.ListWorkspacesWithStatsRow, error)
}

type initRequest struct {
	RootPath string `json:"root_path"`
}

type initResponse struct {
	WorkspaceHash string `json:"workspace_hash"`
	RootPath      string `json:"root_path"`
	Name          string `json:"name"`
	AgentsSnippet string `json:"agents_snippet"`
}

// workspaceItem is the JSON shape of a single workspace returned by
// GET /api/v1/workspaces. Field names match web/src/api/types.ts Workspace
// interface. Renaming these JSON tags is a breaking API change — see
// openspec/specs/workspaces-api-contract for the canonical contract.
type workspaceItem struct {
	Hash                string     `json:"hash"`
	RootPath            string     `json:"root_path"`
	Name                string     `json:"name"`
	DocumentCount       int64      `json:"doc_count"`
	ChunkCount          int64      `json:"chunk_count"`
	LastDocumentUpdated *time.Time `json:"last_document_updated"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// listWorkspacesResponse wraps the workspaces array. The wrapper enables
// future extension (pagination, totals) without breaking clients.
type listWorkspacesResponse struct {
	Workspaces []workspaceItem `json:"workspaces"`
}

func initWorkspace(ctx context.Context, q WorkspaceQuerier, hash, name, absPath string) (sqlc.Workspace, error) {
	ws, err := q.UpsertWorkspace(ctx, sqlc.UpsertWorkspaceParams{
		Hash: hash,
		Name: name,
		Path: absPath,
	})
	if err != nil {
		return sqlc.Workspace{}, err
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return sqlc.Workspace{}, fmt.Errorf("failed to get home directory: %w", err)
	}
	memoryPath := filepath.Join(home, ".nano-brain", "memory")
	sessionsPath := filepath.Join(home, ".nano-brain", "sessions")

	if _, err := q.UpsertCollection(ctx, sqlc.UpsertCollectionParams{
		WorkspaceHash: ws.Hash,
		Name:          collectionNameMemory,
		Path:          memoryPath,
		GlobPattern:   "**/*",
		UpdateMode:    "auto",
	}); err != nil {
		return sqlc.Workspace{}, err
	}

	if _, err := q.UpsertCollection(ctx, sqlc.UpsertCollectionParams{
		WorkspaceHash: ws.Hash,
		Name:          collectionNameSessions,
		Path:          sessionsPath,
		GlobPattern:   "**/*",
		UpdateMode:    "auto",
	}); err != nil {
		return sqlc.Workspace{}, err
	}

	if _, err := q.UpsertCollection(ctx, sqlc.UpsertCollectionParams{
		WorkspaceHash: ws.Hash,
		Name:          "code",
		Path:          absPath,
		GlobPattern:   "**/*",
		UpdateMode:    "auto",
	}); err != nil {
		return sqlc.Workspace{}, err
	}

	return ws, nil
}

// validateRootPath rejects a root_path that the daemon cannot walk. The
// watcher and walkCollectionFiles both stat col.Path on the daemon's own
// filesystem, so a path the daemon cannot read can never be indexed —
// registering it anyway would only create a permanently-empty collection.
func validateRootPath(path string) error {
	if _, err := os.Stat(path); err != nil && os.IsNotExist(err) {
		return fmt.Errorf("root_path does not exist: %s", path)
	}
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("root_path cannot be read: %s", path)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("root_path cannot be read: %s", path)
	}
	if !info.IsDir() {
		return fmt.Errorf("root_path is not a directory: %s", path)
	}
	if _, err := f.Readdirnames(1); err != nil && err != io.EOF {
		return fmt.Errorf("root_path cannot be read: %s", path)
	}
	return nil
}

// InitWorkspace godoc
// @Summary      Register a workspace
// @Description  Registers a root path as a workspace and creates its default collections (memory, sessions, code). When db is non-nil, all three DB operations are wrapped in a single transaction.
// @Tags         workspaces
// @Accept       json
// @Produce      json
// @Param        request body initRequest true "Workspace root path"
// @Success      200 {object} initResponse
// @Failure      400 {object} map[string]string
// @Failure      500 {object} map[string]string
// @Router       /api/v1/init [post]
func InitWorkspace(q WorkspaceQuerier, db *sql.DB, fw *watcher.Watcher, watcherCfg config.WatcherConfig, logger zerolog.Logger) echo.HandlerFunc {
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

		if err := validateRootPath(absPath); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, err.Error())
		}

		hash, err := storage.WorkspaceHash(absPath)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid root_path")
		}
		name := filepath.Base(absPath)

		var ws sqlc.Workspace
		if db != nil {
			tx, err := db.BeginTx(c.Request().Context(), nil)
			if err != nil {
				logger.Error().Err(err).Msg("begin transaction failed")
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to register workspace")
			}
			ws, err = initWorkspace(c.Request().Context(), sqlc.New(tx), hash, name, absPath)
			if err != nil {
				_ = tx.Rollback()
				logger.Error().Err(err).Str("hash", hash).Msg("init workspace failed")
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to register workspace")
			}
			if err := tx.Commit(); err != nil {
				logger.Error().Err(err).Msg("commit transaction failed")
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to register workspace")
			}
		} else {
			ws, err = initWorkspace(c.Request().Context(), q, hash, name, absPath)
			if err != nil {
				logger.Error().Err(err).Str("hash", hash).Msg("init workspace failed")
				return echo.NewHTTPError(http.StatusInternalServerError, "failed to register workspace")
			}
		}

		if fw != nil {
			// Only disk-backed collections are watched. memory and sessions are
			// DB-backed; attaching them here made init-time behaviour disagree
			// with daemon startup, which already skips a collection whose path
			// does not exist.
			cfgExclude, cfgExtensions := watcherCfg.ResolveFilterForPath(absPath)
			if err := fw.WatchWithFilter("code", absPath, hash, "**/*", cfgExclude, cfgExtensions); err != nil {
				logger.Warn().Err(err).Str("collection", "code").Msg("failed to attach watcher after init")
			}
		}

		snippet := "## nano-brain Access\n\nnano-brain workspace: " + ws.Hash + "\n" +
			"nano-brain is accessed via CLI: `npx nano-brain <command>`."

		reqLog := LoggerFromCtx(c, logger)
		reqLog.Info().
			Str("workspace_hash", ws.Hash).
			Str("root_path", ws.Path).
			Msg("workspace registered")

		return c.JSON(http.StatusOK, initResponse{
			WorkspaceHash: ws.Hash,
			RootPath:      ws.Path,
			Name:          ws.Name,
			AgentsSnippet: snippet,
		})
	}
}

// ListWorkspaces godoc
// @Summary      List registered workspaces
// @Description  Returns all registered workspaces with document/chunk counts
// @Tags         workspaces
// @Produce      json
// @Success      200 {object} listWorkspacesResponse
// @Failure      500 {object} map[string]string
// @Router       /api/v1/workspaces [get]
func ListWorkspaces(q WorkspaceQuerier, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		rows, err := q.ListWorkspacesWithStats(c.Request().Context())
		if err != nil {
			logger.Error().Err(err).Msg("list workspaces failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to list workspaces")
		}

		items := make([]workspaceItem, 0, len(rows))
		for _, r := range rows {
			item := workspaceItem{
				Hash:          r.Hash,
				RootPath:      r.Path,
				Name:          r.Name,
				DocumentCount: r.DocumentCount,
				ChunkCount:    r.ChunkCount,
				CreatedAt:     r.CreatedAt,
				UpdatedAt:     r.UpdatedAt,
			}
			if t, ok := r.LastDocumentUpdated.(time.Time); ok {
				item.LastDocumentUpdated = &t
			}
			items = append(items, item)
		}

		return c.JSON(http.StatusOK, listWorkspacesResponse{Workspaces: items})
	}
}
