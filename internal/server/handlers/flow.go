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
	ListConsumerEntryNodesByWorkspace(ctx context.Context, workspaceHash string) ([]sqlc.GraphEdge, error)
	ListHTTPEndpointsByWorkspace(ctx context.Context, workspaceHash string) ([]sqlc.ListHTTPEndpointsByWorkspaceRow, error)
}

// FlowMaterializer is the interface for triggering flow materialization.
type FlowMaterializer interface {
	Trigger(ctx context.Context, workspaceHash string)
}

type flowRequest struct {
	Entry            string   `json:"entry"`
	MaxDepth         int      `json:"max_depth"`
	Format           string   `json:"format"` // "mermaid" (default) | "json"
	StitchWorkspaces []string `json:"stitch_workspaces"`
}

type flowNode struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	Ambiguous bool   `json:"ambiguous,omitempty"`
}

type flowGraphEdge struct {
	From           string `json:"from"`
	To             string `json:"to"`
	Kind           string `json:"kind"`
	Line           int    `json:"line,omitempty"`
	Conditional    bool   `json:"conditional,omitempty"`
	ConditionLabel string `json:"condition_label,omitempty"`
}

type flowResponse struct {
	Found     bool           `json:"found"`
	Entry     string         `json:"entry"`
	Method    string         `json:"method,omitempty"`
	Path      string         `json:"path,omitempty"`
	Chain     []flowNode     `json:"chain"`
	Externals []flowNode     `json:"externals"`
	Nodes     []flowNode     `json:"nodes"`
	Edges     []flowGraphEdge `json:"edges"`
	Mermaid   string         `json:"mermaid,omitempty"`
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

		if len(req.StitchWorkspaces) > 0 {
			publishEdges := filterPublishEdges(edges)
			stitched := flow.Stitch(ctx, publishEdges, req.StitchWorkspaces, q)
			appendStitchedToFlow(&f, stitched)
		}

		chain, externals := splitNodes(f.Nodes)

		nodes := make([]flowNode, 0, len(f.Nodes))
		for _, n := range f.Nodes {
			nodes = append(nodes, flowNode{
				ID:        n.ID,
				Name:      n.Name,
				Role:      string(n.Role),
				Ambiguous: n.Ambiguous,
			})
		}
		graphEdges := make([]flowGraphEdge, 0, len(f.Edges))
		for _, e := range f.Edges {
			graphEdges = append(graphEdges, flowGraphEdge{
				From:           e.From,
				To:             e.To,
				Kind:           e.Kind,
				Line:           e.Line,
				Conditional:    e.Conditional,
				ConditionLabel: e.ConditionLabel,
			})
		}

		resp := flowResponse{
			Found:     true,
			Entry:     f.Entry,
			Method:    f.Method,
			Path:      f.Path,
			Chain:     chain,
			Externals: externals,
			Nodes:     nodes,
			Edges:     graphEdges,
		}
		if diagram := flow.Render(f, req.Format); diagram != "" {
			resp.Mermaid = diagram
		}

		return c.JSON(http.StatusOK, resp)
	}
}

func FlowMaterialize(getMat func() FlowMaterializer, flowCfg config.FlowConfig, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		if !flowCfg.Enabled {
			return c.JSON(http.StatusOK, map[string]any{
				"status":  "skipped",
				"message": "flow indexing disabled",
			})
		}

		workspace, _ := c.Get("workspace").(string)
		if workspace == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "workspace is required")
		}

		mat := getMat()
		if mat == nil {
			return echo.NewHTTPError(http.StatusServiceUnavailable, "flow materialization not configured")
		}

		// Detach from the request context: this goroutine outlives the HTTP
		// response, so c.Request().Context() would be cancelled immediately
		// after we return, aborting materialization mid-run.
		go mat.Trigger(context.Background(), workspace)
		logger.Info().Str("workspace", workspace).Msg("flow materialization triggered")

		return c.JSON(http.StatusOK, map[string]any{
			"status":  "queued",
			"message": "flow materialization triggered",
		})
	}
}

type httpEndpoint struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

func ListFlowEndpoints(q FlowQuerier, logger zerolog.Logger) echo.HandlerFunc {
	return func(c echo.Context) error {
		workspace := c.QueryParam("workspace")
		if workspace == "" {
			workspace, _ = c.Get("workspace").(string)
		}
		if workspace == "" {
			return echo.NewHTTPError(http.StatusBadRequest, "workspace is required")
		}

		ctx := c.Request().Context()
		rows, err := q.ListHTTPEndpointsByWorkspace(ctx, workspace)
		if err != nil {
			logger.Error().Err(err).Str("workspace", workspace).Msg("list flow endpoints failed")
			return echo.NewHTTPError(http.StatusInternalServerError, "list flow endpoints failed")
		}

		endpoints := make([]httpEndpoint, 0, len(rows))
		for _, r := range rows {
			endpoints = append(endpoints, httpEndpoint{
				Source: r.SourceNode,
				Target: r.TargetNode,
			})
		}

		return c.JSON(http.StatusOK, map[string]any{
			"endpoints": endpoints,
		})
	}
}

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
				if line, ok := meta["line"].(float64); ok {
					e.Line = int(line)
				}
				e.Metadata = meta
			}
		}
		out = append(out, e)
	}
	return out
}

// filterPublishEdges returns integration edges that carry a non-empty topic.
func filterPublishEdges(edges []graph.Edge) []graph.Edge {
	var out []graph.Edge
	for _, e := range edges {
		if e.Kind != graph.EdgeIntegration {
			continue
		}
		if topic, ok := e.Metadata["topic"].(string); ok && topic != "" {
			out = append(out, e)
		}
	}
	return out
}

// appendStitchedToFlow adds cross-service edges and their target nodes to the flow.
func appendStitchedToFlow(f *flow.Flow, stitched []flow.FlowEdge) {
	if len(stitched) == 0 {
		return
	}
	existing := make(map[string]bool, len(f.Nodes))
	for _, n := range f.Nodes {
		existing[n.ID] = true
	}
	for _, se := range stitched {
		if !existing[se.To] {
			existing[se.To] = true
			f.Nodes = append(f.Nodes, flow.FlowNode{
				ID:   se.To,
				Name: se.To,
				Role: flow.RoleIntegration,
			})
		}
		f.Edges = append(f.Edges, se)
	}
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
