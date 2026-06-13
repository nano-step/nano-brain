package handlers

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/nano-brain/nano-brain/internal/config"
	"github.com/nano-brain/nano-brain/internal/flow"
	"github.com/nano-brain/nano-brain/internal/graph"
	"github.com/nano-brain/nano-brain/internal/storage/sqlc"
	"github.com/rs/zerolog"
)

// FlowQuerier is the storage interface used by GraphFlow.
type FlowQuerier interface {
	ListAllEdgesByWorkspace(ctx context.Context, workspaceHash string) ([]sqlc.GraphEdge, error)
}

type flowRequest struct {
	Entry    string `json:"entry"`
	MaxDepth int    `json:"max_depth"`
	Format   string `json:"format"` // "mermaid" (default) | "json"
}

type flowNode struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	Ambiguous bool   `json:"ambiguous,omitempty"`
}

type flowResponse struct {
	Found     bool       `json:"found"`
	Entry     string     `json:"entry"`
	Method    string     `json:"method,omitempty"`
	Path      string     `json:"path,omitempty"`
	Chain     []flowNode `json:"chain"`
	Externals []flowNode `json:"externals"`
	Mermaid   string     `json:"mermaid,omitempty"`
}

// GraphFlow handles POST /api/v1/graph/flow.
// It loads all workspace edges, builds a flow starting from entry, and
// returns the node chain plus an optional Mermaid diagram.
func GraphFlow(q FlowQuerier, flowCfg config.FlowConfig, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		if !flowCfg.Enabled {
			return c.JSON(http.StatusOK, map[string]any{
				"found":    false,
				"message": "flow indexing disabled",
			})
		}

		workspace := c.Get("workspace").(string)

		var req flowRequest
		if err := c.Bind(&req); err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
		}
		if req.Entry == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "entry is required")
		}
		if req.Format == "" {
			req.Format = "mermaid"
		}
		maxDepth := req.MaxDepth
		if maxDepth <= 0 || maxDepth > flowCfg.MaxDepth {
			maxDepth = flowCfg.MaxDepth
		}

		ctx := c.Request().Context()
		rawEdges, err := q.ListAllEdgesByWorkspace(ctx, workspace)
		if err != nil {
			logger.Error().Err(err).Str("workspace", workspace).Str("entry", req.Entry).Msg("flow query failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "flow query failed")
		}

		edges := convertEdges(rawEdges)

		// An entry is only "found" if an http edge actually originates from it.
		// BuildFlow always emits the entry node itself, so node count cannot
		// distinguish a real flow from an unknown entry.
		if !hasHTTPEntry(edges, req.Entry) {
			return c.JSON(http.StatusOK, map[string]any{
				"found":   false,
				"entry":   req.Entry,
				"message": "entry not found among http edges",
			})
		}

		f := flow.BuildFlow(edges, req.Entry, maxDepth, flowCfg.MaxFanout)

		chain, externals := splitNodes(f.Nodes)

		resp := flowResponse{
			Found:     true,
			Entry:     f.Entry,
			Method:    f.Method,
			Path:      f.Path,
			Chain:     chain,
			Externals: externals,
		}
		if req.Format != "json" {
			resp.Mermaid = flow.RenderFlowchart(f)
		}

		return c.JSON(http.StatusOK, resp)
	}
}

// hasHTTPEntry reports whether any http edge originates from the given entry node.
func hasHTTPEntry(edges []graph.Edge, entry string) bool {
	for _, e := range edges {
		if e.Kind == graph.EdgeHTTP && e.SourceNode == entry {
			return true
		}
	}
	return false
}

// convertEdges maps sqlc.GraphEdge rows to graph.Edge values.
// Metadata JSON is decoded but only used when it carries extra fields;
// the SourceFile and Language fields stored in the DB row are used directly.
func convertEdges(rows []sqlc.GraphEdge) []graph.Edge {
	out := make([]graph.Edge, 0, len(rows))
	for _, r := range rows {
		e := graph.Edge{
			SourceNode: r.SourceNode,
			TargetNode: r.TargetNode,
			Kind:       graph.EdgeKind(r.EdgeType),
			SourceFile: r.SourceFile,
		}
		// Decode metadata JSON to extract language and any extra fields.
		if len(r.Metadata) > 0 {
			var meta map[string]any
			if err := json.Unmarshal(r.Metadata, &meta); err == nil {
				if lang, ok := meta["language"].(string); ok {
					e.Language = lang
				}
				e.Metadata = meta
			}
		}
		out = append(out, e)
	}
	return out
}

// splitNodes separates the flow nodes into regular chain nodes and external nodes.
func splitNodes(nodes []flow.FlowNode) (chain []flowNode, externals []flowNode) {
	for _, n := range nodes {
		fn := flowNode{
			ID:        n.ID,
			Name:      n.Name,
			Role:      string(n.Role),
			Ambiguous: n.Ambiguous,
		}
		if n.Role == flow.RoleExternal {
			externals = append(externals, fn)
		} else {
			chain = append(chain, fn)
		}
	}
	if chain == nil {
		chain = []flowNode{}
	}
	if externals == nil {
		externals = []flowNode{}
	}
	return chain, externals
}
