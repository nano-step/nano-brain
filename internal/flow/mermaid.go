package flow

import (
	"fmt"
	"sort"
	"strings"
)

// sanitizeID converts a node name into a valid Mermaid node id by replacing
// characters that are invalid in Mermaid ids with underscores.
func sanitizeID(name string) string {
	replacer := strings.NewReplacer(
		" ", "_",
		"/", "_",
		".", "_",
		":", "_",
		"-", "_",
		"(", "_",
		")", "_",
		"<", "_",
		">", "_",
		"[", "_",
		"]", "_",
		"{", "_",
		"}", "_",
		"@", "_",
		"#", "_",
		"$", "_",
		"%", "_",
		"^", "_",
		"&", "_",
		"*", "_",
		"+", "_",
		"=", "_",
		"|", "_",
		"\\", "_",
		"\"", "_",
		"'", "_",
		"`", "_",
		",", "_",
		";", "_",
		"!", "_",
		"?", "_",
		"~", "_",
	)
	return replacer.Replace(name)
}

// Render renders a Flow using the requested format:
//   - "sequence" → Mermaid sequenceDiagram
//   - "json" → empty string (caller handles JSON separately)
//   - anything else (including "mermaid") → Mermaid graph TD
func Render(f Flow, format string) string {
	switch format {
	case "sequence":
		return RenderSequenceDiagram(f)
	case "json":
		return ""
	default:
		return RenderFlowchart(f)
	}
}

// RenderFlowchart renders a Flow as a Mermaid graph TD diagram.
// Output is deterministic: nodes and edges are sorted before emission.
func RenderFlowchart(f Flow) string {
	var sb strings.Builder
	sb.WriteString("graph TD\n")

	// Sort nodes by ID for deterministic output.
	nodes := make([]FlowNode, len(f.Nodes))
	copy(nodes, f.Nodes)
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].ID < nodes[j].ID
	})

	// Identify cross-service target nodes.
	crossServiceNodes := make(map[string]string)
	for _, e := range f.Edges {
		if e.Kind == "cross_service" && e.CrossServiceWorkspace != "" {
			crossServiceNodes[e.To] = e.CrossServiceWorkspace
		}
	}

	// Emit node declarations.
	for _, n := range nodes {
		id := sanitizeID(n.ID)
		label := fmt.Sprintf("%s<br/>(%s)", n.Name, string(n.Role))
		if ws, ok := crossServiceNodes[n.ID]; ok {
			label += fmt.Sprintf("<br/>ws: %s", ws)
		}
		sb.WriteString(fmt.Sprintf("    %s[\"%s\"]\n", id, label))
	}

	// Sort edges for deterministic output.
	edges := make([]FlowEdge, len(f.Edges))
	copy(edges, f.Edges)
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		if edges[i].To != edges[j].To {
			return edges[i].To < edges[j].To
		}
		return edges[i].Kind < edges[j].Kind
	})

	// Emit edges.
	for _, e := range edges {
		from := sanitizeID(e.From)
		to := sanitizeID(e.To)
		if e.Conditional || e.Kind == "middleware" {
			sb.WriteString(fmt.Sprintf("    %s -.-> %s\n", from, to))
		} else {
			sb.WriteString(fmt.Sprintf("    %s --> %s\n", from, to))
		}
	}

	// Cross-service class definition and assignments.
	if len(crossServiceNodes) > 0 {
		sb.WriteString("\n    classDef crossService fill:#f9f,stroke:#a0a\n")
		for nodeID := range crossServiceNodes {
			sb.WriteString(fmt.Sprintf("    class %s crossService\n", sanitizeID(nodeID)))
		}
	}

	return sb.String()
}
