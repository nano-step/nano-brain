package flow

import (
	"fmt"
	"sort"
	"strings"
)

// sanitizeID converts a node name into a valid Mermaid node id by replacing
// characters that are invalid in Mermaid ids with underscores.
// Mermaid node IDs must start with a letter or underscore, not a number.
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
	id := replacer.Replace(name)
	if id == "" {
		id = "unknown"
	}
	// Mermaid IDs must start with a letter or underscore
	if len(id) > 0 && id[0] >= '0' && id[0] <= '9' {
		id = "n" + id
	}
	// Avoid Mermaid reserved keywords and arrow-type letters (call/end/x/o/…),
	// which break the flowchart/sequence parser when used as a bare node id.
	if mermaidReservedIDs[strings.ToLower(id)] {
		id = "n_" + id
	}
	return id
}

// mermaidReservedIDs are identifiers that cannot be used as bare node ids: flow
// chart keywords plus the single-letter arrow-type tokens (x, o, v).
var mermaidReservedIDs = map[string]bool{
	"graph": true, "subgraph": true, "end": true, "flowchart": true,
	"class": true, "classdef": true, "click": true, "style": true,
	"linkstyle": true, "direction": true, "call": true, "href": true,
	"link": true, "default": true, "interpolate": true,
	"x": true, "o": true, "v": true,
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

// sanitizeLabel escapes characters in a mermaid node label that would break parsing.
// Mermaid labels inside ["..."] must not contain unescaped quotes, brackets, or braces.
func sanitizeLabel(name string) string {
	replacer := strings.NewReplacer(
		"\"", "'",
		"[", " ",
		"]", " ",
		"{", " ",
		"}", " ",
		"<", " ",
		">", " ",
		"(", " ",
		")", " ",
		":", " ",
		";", " ",
		"\n", " ",
	)
	return replacer.Replace(name)
}

func truncateLabel(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen-1]) + "…"
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
		label := fmt.Sprintf("%s<br/>(%s)", sanitizeLabel(n.Name), string(n.Role))
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

	// Emit edges, collapsing duplicates. The model keeps one edge per call site
	// (distinct line numbers), but the flowchart has no line info, so multiple
	// calls between the same two nodes would render as identical arrows.
	emittedEdges := make(map[string]bool)
	for _, e := range edges {
		from := sanitizeID(e.From)
		to := sanitizeID(e.To)
		if from == "" || to == "" || from == "unknown" || to == "unknown" {
			continue
		}
		arrow := "-->"
		if e.Conditional || e.Kind == "middleware" {
			arrow = "-.->"
		}
		key := from + arrow + to
		if emittedEdges[key] {
			continue
		}
		emittedEdges[key] = true

		// Add condition label for conditional edges
		if e.Conditional && e.ConditionLabel != "" {
			label := truncateLabel(e.ConditionLabel, 80)
			sb.WriteString(fmt.Sprintf("    %s %s %s\n", from, arrow, to))
			sb.WriteString(fmt.Sprintf("    link 0,%s,%s,%s\n", from, to, sanitizeLabel(label)))
		} else {
			sb.WriteString(fmt.Sprintf("    %s %s %s\n", from, arrow, to))
		}
	}

	// Cross-service class definition and assignments.
	if len(crossServiceNodes) > 0 {
		sb.WriteString("\n    classDef crossService fill:#f9f,stroke:#a0a\n")
		// Sort node IDs so class assignments are emitted deterministically
		// (map iteration order is otherwise randomized).
		csIDs := make([]string, 0, len(crossServiceNodes))
		for nodeID := range crossServiceNodes {
			csIDs = append(csIDs, nodeID)
		}
		sort.Strings(csIDs)
		for _, nodeID := range csIDs {
			sb.WriteString(fmt.Sprintf("    class %s crossService\n", sanitizeID(nodeID)))
		}
	}

	return sb.String()
}
