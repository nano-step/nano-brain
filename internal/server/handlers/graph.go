package handlers

import (
	"context"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

type GraphQuerier interface {
	GetOutgoingEdges(ctx context.Context, arg sqlc.GetOutgoingEdgesParams) ([]sqlc.GraphEdge, error)
	GetIncomingEdges(ctx context.Context, arg sqlc.GetIncomingEdgesParams) ([]sqlc.GraphEdge, error)
}

type graphQueryRequest struct {
	Node      string `json:"node"`
	Direction string `json:"direction"`
	EdgeType  string `json:"edge_type"`
}

type graphEdgeItem struct {
	Source   string `json:"source"`
	Target   string `json:"target"`
	EdgeType string `json:"edge_type"`
}

type graphQueryResponse struct {
	Node      string          `json:"node"`
	Direction string          `json:"direction"`
	Edges     []graphEdgeItem `json:"edges"`
}

func GraphQuery(q GraphQuerier, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		workspace := c.Get("workspace").(string)

		var req graphQueryRequest
		if err := c.Bind(&req); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
		}
		if req.Node == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "node is required")
		}
		if req.Direction == "" {
			req.Direction = "out"
		}

		ctx := c.Request().Context()
		var rows []sqlc.GraphEdge
		var err error

		switch req.Direction {
		case "in":
			rows, err = q.GetIncomingEdges(ctx, sqlc.GetIncomingEdgesParams{
				WorkspaceHash: workspace,
				TargetNode:    req.Node,
				Column3:       req.EdgeType,
			})
		case "both":
			out, errOut := q.GetOutgoingEdges(ctx, sqlc.GetOutgoingEdgesParams{
				WorkspaceHash: workspace,
				SourceNode:    req.Node,
				Column3:       req.EdgeType,
			})
			in, errIn := q.GetIncomingEdges(ctx, sqlc.GetIncomingEdgesParams{
				WorkspaceHash: workspace,
				TargetNode:    req.Node,
				Column3:       req.EdgeType,
			})
			if errOut != nil {
				err = errOut
			} else if errIn != nil {
				err = errIn
			} else {
				rows = append(out, in...)
			}
		default:
			rows, err = q.GetOutgoingEdges(ctx, sqlc.GetOutgoingEdgesParams{
				WorkspaceHash: workspace,
				SourceNode:    req.Node,
				Column3:       req.EdgeType,
			})
		}

		if err != nil {
			logger.Error().Err(err).Str("workspace", workspace).Str("node", req.Node).Msg("graph query failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "graph query failed")
		}

		items := make([]graphEdgeItem, 0, len(rows))
		for _, r := range rows {
			items = append(items, graphEdgeItem{
				Source:   r.SourceNode,
				Target:   r.TargetNode,
				EdgeType: r.EdgeType,
			})
		}

		return c.JSON(http.StatusOK, graphQueryResponse{
			Node:      req.Node,
			Direction: req.Direction,
			Edges:     items,
		})
	}
}
