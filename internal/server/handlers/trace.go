package handlers

import (
	"context"
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

type TraceQuerier interface {
	GetOutgoingEdges(ctx context.Context, arg sqlc.GetOutgoingEdgesParams) ([]sqlc.GraphEdge, error)
	GetOutgoingEdgesBySymbol(ctx context.Context, arg sqlc.GetOutgoingEdgesBySymbolParams) ([]sqlc.GraphEdge, error)
}

type traceRequest struct {
	Node     string `json:"node"`
	MaxDepth int    `json:"max_depth"`
}

type traceStep struct {
	Node  string `json:"node"`
	Depth int    `json:"depth"`
	Via   string `json:"via"`
}

type traceResponse struct {
	Entry string      `json:"entry"`
	Chain []traceStep `json:"chain"`
}

// GraphTrace godoc
// @Summary      Trace a call chain from an entry node
// @Description  Breadth-first traces the outgoing call chain from an entry node, up to max_depth
// @Tags         graph
// @Accept       json
// @Produce      json
// @Param        request body traceRequest true "Trace query"
// @Success      200 {object} traceResponse
// @Failure      400 {object} map[string]string
// @Security     WorkspaceAuth
// @Router       /api/v1/graph/trace [post]
func GraphTrace(q TraceQuerier, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		workspace := c.Get("workspace").(string)

		var req traceRequest
		if err := c.Bind(&req); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
		}
		if req.Node == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "node is required")
		}
		if req.MaxDepth <= 0 || req.MaxDepth > 10 {
			req.MaxDepth = 5
		}

		ctx := c.Request().Context()
		chain, err := traceCallChain(ctx, q, workspace, req.Node, req.MaxDepth)
		if err != nil {
			logger.Error().Err(err).Str("workspace", workspace).Str("node", req.Node).Msg("trace query failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "trace query failed")
		}

		return c.JSON(http.StatusOK, traceResponse{
			Entry: req.Node,
			Chain: chain,
		})
	}
}

func traceCallChain(ctx context.Context, q TraceQuerier, workspace, entry string, maxDepth int) ([]traceStep, error) {
	seen := map[string]bool{entry: true}
	var chain []traceStep

	type frame struct {
		node  string
		depth int
		via   string
	}
	queue := []frame{{node: entry, depth: 0, via: ""}}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		if cur.depth >= maxDepth {
			continue
		}

		var edges []sqlc.GraphEdge
		var err error
		if strings.Contains(cur.node, "::") {
			// Qualified name (e.g. "file.go::Func") — exact match, no cross-file noise.
			edges, err = q.GetOutgoingEdges(ctx, sqlc.GetOutgoingEdgesParams{
				WorkspaceHash: workspace,
				SourceNode:    cur.node,
				Column3:       "calls",
			})
		} else {
			// Bare name (e.g. "BM25SearchAll") — reconcile to all defining source nodes.
			edges, err = q.GetOutgoingEdgesBySymbol(ctx, sqlc.GetOutgoingEdgesBySymbolParams{
				WorkspaceHash: workspace,
				SourceNode:    cur.node,
				Column3:       "calls",
			})
		}
		if err != nil {
			return nil, err
		}

		for _, e := range edges {
			if seen[e.TargetNode] {
				continue
			}
			seen[e.TargetNode] = true
			step := traceStep{
				Node:  e.TargetNode,
				Depth: cur.depth + 1,
				Via:   cur.node,
			}
			chain = append(chain, step)
			queue = append(queue, frame{node: e.TargetNode, depth: cur.depth + 1, via: cur.node})
		}
	}
	return chain, nil
}
