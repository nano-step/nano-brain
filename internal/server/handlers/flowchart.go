package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/graph"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

// FlowchartQuerier is the storage interface used by GraphFlowchart.
type FlowchartQuerier interface {
	ListAllEdgesByWorkspace(ctx context.Context, workspaceHash string) ([]sqlc.GraphEdge, error)
	GetFunctionFlowchartByHandler(ctx context.Context, arg sqlc.GetFunctionFlowchartByHandlerParams) (sqlc.FunctionFlowchart, error)
}

type flowchartRequest struct {
	Entry string `json:"entry"`
}

type flowchartResponse struct {
	Found  bool            `json:"found"`
	Entry  string          `json:"entry"`
	Method string          `json:"method,omitempty"`
	Path   string          `json:"path,omitempty"`
	Status string          `json:"status,omitempty"`
	CFG    json.RawMessage `json:"cfg"`
}

// GraphFlowchart handles POST /api/v1/graph/flowchart.
// It resolves an HTTP route entry (or handler symbol) to its handler function
// and returns that function's stored control-flow graph. Returns found:false
// when no flowchart is stored for the resolved handler (e.g. a non-JS/TS
// handler, since Phase 1b extracts CFGs for JS/TS only).
func GraphFlowchart(q FlowchartQuerier, flowCfg config.FlowConfig, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		if !flowCfg.Enabled {
			return c.JSON(http.StatusOK, map[string]any{
				"found":   false,
				"message": "flow indexing disabled",
			})
		}

		workspace, ok := c.Get("workspace").(string)
		if !ok || workspace == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "workspace is required")
		}

		var req flowchartRequest
		if err := c.Bind(&req); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
		}
		if req.Entry == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "entry is required")
		}

		ctx := c.Request().Context()

		// Resolve the entry to a handler name + (optional) method/path.
		handler, method, path, err := resolveFlowchartHandler(ctx, q, workspace, req.Entry)
		if err != nil {
			logger.Error().Err(err).Str("workspace", workspace).Str("entry", req.Entry).Msg("flowchart resolution failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "flowchart query failed")
		}

		fc, err := q.GetFunctionFlowchartByHandler(ctx, sqlc.GetFunctionFlowchartByHandlerParams{
			WorkspaceHash: workspace,
			Entry:         handler,
		})
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return c.JSON(http.StatusOK, flowchartResponse{
					Found:  false,
					Entry:  req.Entry,
					Method: method,
					Path:   path,
					CFG:    nil,
				})
			}
			logger.Error().Err(err).Str("workspace", workspace).Str("handler", handler).Msg("flowchart query failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "flowchart query failed")
		}

		return c.JSON(http.StatusOK, flowchartResponse{
			Found:  true,
			Entry:  fc.Entry,
			Method: method,
			Path:   path,
			Status: fc.Status,
			CFG:    fc.Cfg,
		})
	}
}

// resolveFlowchartHandler maps a request entry to a handler name to look up.
// If the entry matches an HTTP edge source (a route like "POST /purchase"),
// the edge's target handler is used and method/path are extracted. Otherwise
// the entry is treated as a handler name/symbol directly. The returned handler
// is reduced to its final dotted segment (e.g. "ctrl.create" -> "create") so
// it can match a stored CFG entry's function name.
func resolveFlowchartHandler(ctx context.Context, q FlowchartQuerier, workspace, entry string) (handler, method, path string, err error) {
	rawEdges, err := q.ListAllEdgesByWorkspace(ctx, workspace)
	if err != nil {
		return "", "", "", err
	}

	for _, e := range rawEdges {
		if graph.EdgeKind(e.EdgeType) == graph.EdgeHTTP && e.SourceNode == entry {
			method, path = parseRoute(e.SourceNode, e.Metadata)
			return lastDottedSegment(e.TargetNode), method, path, nil
		}
	}

	// Not an HTTP route — treat the entry itself as a handler name/symbol.
	if idx := strings.Index(entry, "::"); idx >= 0 {
		return entry[idx+2:], "", "", nil
	}
	return lastDottedSegment(entry), "", "", nil
}

// parseRoute extracts method and path for a route node like "POST /purchase".
// Prefers metadata fields when present, falling back to splitting the node.
func parseRoute(node string, metadata json.RawMessage) (method, path string) {
	if len(metadata) > 0 {
		var meta map[string]any
		if json.Unmarshal(metadata, &meta) == nil {
			if m, ok := meta["method"].(string); ok {
				method = m
			}
			if p, ok := meta["path"].(string); ok {
				path = p
			}
		}
	}
	if method == "" || path == "" {
		if idx := strings.IndexByte(node, ' '); idx > 0 {
			if method == "" {
				method = node[:idx]
			}
			if path == "" {
				path = strings.TrimSpace(node[idx+1:])
			}
		}
	}
	return method, path
}

// lastDottedSegment returns the substring after the final '.', e.g.
// "controller.create" -> "create". Symbols without a dot are returned as-is.
func lastDottedSegment(s string) string {
	if idx := strings.LastIndexByte(s, '.'); idx >= 0 && idx < len(s)-1 {
		return s[idx+1:]
	}
	return s
}
