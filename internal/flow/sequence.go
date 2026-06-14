package flow

import (
	"fmt"
	"sort"
	"strings"
)

// participantAlias returns the Mermaid alias for a node.
// Entry nodes are represented as "Client" to keep diagrams readable.
func participantAlias(n FlowNode) string {
	if n.Role == RoleEntry {
		return "Client"
	}
	return sanitizeID(n.ID)
}

// participantLabel returns the human-readable label shown inside the participant box.
func participantLabel(n FlowNode) string {
	if n.Role == RoleEntry {
		return "Client"
	}
	return fmt.Sprintf("%s (%s)", n.Name, string(n.Role))
}

// msgAlias returns the participant alias used in arrow lines for a given node id.
func msgAlias(nodeID string, nodeByID map[string]FlowNode) string {
	if n, ok := nodeByID[nodeID]; ok {
		return participantAlias(n)
	}
	return sanitizeID(nodeID)
}

// RenderSequenceDiagram renders a Flow as a Mermaid sequenceDiagram.
//
// Participants are ordered by first appearance in a DFS from the entry node.
// Middleware participants are inserted before the handler they guard.
// Messages are emitted in DFS traversal order.
// The entry node is displayed as "Client" to keep diagrams readable.
func RenderSequenceDiagram(f Flow) string {
	nodeByID := make(map[string]FlowNode, len(f.Nodes))
	for _, n := range f.Nodes {
		nodeByID[n.ID] = n
	}

	// Build adjacency maps.
	// adj: from → outgoing non-middleware edges (http + calls)
	// mwFor: handler id → []middleware ids that guard it
	adj := make(map[string][]FlowEdge)
	mwFor := make(map[string][]string)

	for _, e := range f.Edges {
		if e.Kind == "middleware" {
			mwFor[e.To] = append(mwFor[e.To], e.From)
		} else {
			adj[e.From] = append(adj[e.From], e)
		}
	}

	// Sort adjacency slices: by source line when known, then alphabetically for determinism.
	for k := range adj {
		sort.Slice(adj[k], func(i, j int) bool {
			li, lj := adj[k][i].Line, adj[k][j].Line
			if li != lj && li > 0 && lj > 0 {
				return li < lj
			}
			return adj[k][i].To < adj[k][j].To
		})
	}
	for k := range mwFor {
		sort.Strings(mwFor[k])
	}

	// DFS to produce participant order.
	// Middleware participants are injected immediately before the handler they guard.
	var participants []FlowNode
	seenP := make(map[string]bool)

	var dfsParticipants func(id string)
	dfsParticipants = func(id string) {
		if seenP[id] {
			return
		}
		seenP[id] = true
		if n, ok := nodeByID[id]; ok {
			participants = append(participants, n)
		}
		// Inject middleware for this node (if it is a handler).
		for _, mwID := range mwFor[id] {
			if !seenP[mwID] {
				seenP[mwID] = true
				if n, ok := nodeByID[mwID]; ok {
					participants = append(participants, n)
				}
			}
		}
		for _, e := range adj[id] {
			dfsParticipants(e.To)
		}
	}
	dfsParticipants(f.Entry)

	// Append any nodes not reached by DFS (should be rare).
	var remaining []FlowNode
	for _, n := range f.Nodes {
		if !seenP[n.ID] {
			remaining = append(remaining, n)
		}
	}
	sort.Slice(remaining, func(i, j int) bool { return remaining[i].ID < remaining[j].ID })
	participants = append(participants, remaining...)

	// DFS to produce ordered message list.
	type msg struct {
		from, to, label string
		isNote          bool
		isIntegration   bool // true → render as dotted async arrow (-->>)
		noteOver        string // comma-separated participant aliases for Note over
	}
	var messages []msg
	seenM := make(map[string]bool)

	var dfsMessages func(fromID string)
	dfsMessages = func(fromID string) {
		if seenM[fromID] {
			return
		}
		seenM[fromID] = true

		// Emit middleware guard notes before the first outgoing edge from a handler.
		for _, mwID := range mwFor[fromID] {
			mwAlias := msgAlias(mwID, nodeByID)
			handlerAlias := msgAlias(fromID, nodeByID)
			messages = append(messages, msg{
				isNote:   true,
				noteOver: fmt.Sprintf("%s,%s", mwAlias, handlerAlias),
				label:    fmt.Sprintf("guarded by %s", nodeByID[mwID].Name),
			})
		}

		for _, e := range adj[fromID] {
			fromAlias := msgAlias(fromID, nodeByID)
			toAlias := msgAlias(e.To, nodeByID)
			label := e.Kind
			if e.Kind == "http" && f.Entry != "" {
				label = f.Entry
			}
			messages = append(messages, msg{
				from:         fromAlias,
				to:           toAlias,
				label:        label,
				isIntegration: e.Kind == "integration",
			})
			dfsMessages(e.To)
		}
	}
	dfsMessages(f.Entry)

	// Render.
	var sb strings.Builder
	sb.WriteString("sequenceDiagram\n")

	// Participant declarations — deduplicate by alias since entry → Client.
	seenAlias := make(map[string]bool)
	for _, p := range participants {
		alias := participantAlias(p)
		if seenAlias[alias] {
			continue
		}
		seenAlias[alias] = true
		label := participantLabel(p)
		if alias == label {
			sb.WriteString(fmt.Sprintf("    participant %s\n", alias))
		} else {
			sb.WriteString(fmt.Sprintf("    participant %s as %s\n", alias, label))
		}
	}

	sb.WriteString("\n")

	// Messages.
	for _, m := range messages {
		if m.isNote {
			sb.WriteString(fmt.Sprintf("    Note over %s: %s\n", m.noteOver, m.label))
		} else if m.isIntegration {
			sb.WriteString(fmt.Sprintf("    %s->>%s: %s\n", m.from, m.to, m.label))
			sb.WriteString(fmt.Sprintf("    Note right of %s: integration\n", m.to))
		} else {
			sb.WriteString(fmt.Sprintf("    %s->>%s: %s\n", m.from, m.to, m.label))
		}
	}

	return sb.String()
}
