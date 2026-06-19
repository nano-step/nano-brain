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
	return fmt.Sprintf("%s (%s)", sanitizeLabel(n.Name), string(n.Role))
}

// msgAlias returns the participant alias used in arrow lines for a given node id.
func msgAlias(nodeID string, nodeByID map[string]FlowNode) string {
	if n, ok := nodeByID[nodeID]; ok {
		return participantAlias(n)
	}
	return sanitizeID(nodeID)
}

// groupActors maps each node ID to a system-level actor alias.
func groupActors(f Flow, serviceName string) map[string]string {
	actorForNode := make(map[string]string, len(f.Nodes))
	for _, n := range f.Nodes {
		switch n.Role {
		case RoleEntry:
			actorForNode[n.ID] = "Client"
		case RoleMiddleware:
			actorForNode[n.ID] = serviceName
		case RoleHandler, RoleFunc, RoleRepo, RoleService:
			actorForNode[n.ID] = serviceName
		case RoleIntegration:
			actorForNode[n.ID] = extractSystemName(n.Name)
		case RoleExternal:
			actorForNode[n.ID] = serviceName
		default:
			actorForNode[n.ID] = serviceName
		}
	}
	// Cross-service edges assign target to Service actor
	for _, e := range f.Edges {
		if e.Kind == "cross_service" && e.CrossServiceWorkspace != "" {
			ws := e.CrossServiceWorkspace
			if len(ws) > 8 {
				ws = ws[:8]
			}
			actorForNode[e.To] = "Service:" + ws
		}
	}
	return actorForNode
}

// extractSystemName extracts a clean system name from an integration/external node name.
func extractSystemName(name string) string {
	s := name
	for _, method := range []string{"GET ", "POST ", "PUT ", "DELETE ", "PATCH "} {
		if strings.HasPrefix(s, method) {
			s = s[len(method):]
			break
		}
	}
	// Remove template variables ${...}
	for {
		start := strings.Index(s, "${")
		if start < 0 {
			break
		}
		end := strings.Index(s[start:], "}")
		if end < 0 {
			break
		}
		s = s[:start] + s[start+end+1:]
	}
	s = strings.ReplaceAll(s, "/", " ")
	s = strings.ReplaceAll(s, "-", " ")
	s = strings.ReplaceAll(s, "_", " ")

	// Split on camelCase boundaries
	var words []string
	var current strings.Builder
	var prevLower bool
	for _, r := range s {
		if r >= 'A' && r <= 'Z' && prevLower && current.Len() > 0 {
			words = append(words, current.String())
			current.Reset()
		}
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			current.WriteRune(r)
			prevLower = (r >= 'a' && r <= 'z')
		} else {
			if current.Len() > 0 {
				words = append(words, current.String())
				current.Reset()
			}
			prevLower = false
		}
	}
	if current.Len() > 0 {
		words = append(words, current.String())
	}

	// Title-case first 1-2 words
	var result []string
	for _, w := range words {
		if len(w) == 0 {
			continue
		}
		result = append(result, strings.ToUpper(w[:1])+strings.ToLower(w[1:]))
		if len(result) >= 2 {
			break
		}
	}
	if len(result) == 0 {
		s = strings.TrimSpace(s)
		if len(s) > 20 {
			s = s[:20]
		}
		if s == "" {
			return "External"
		}
		return strings.ToUpper(s[:1]) + s[1:]
	}
	return strings.Join(result, " ")
}

// RenderSequenceDiagram renders a Flow as a Mermaid sequenceDiagram.
func RenderSequenceDiagram(f Flow) string {
	actorForNode := groupActors(f, f.ServiceName)
	nodeByID := make(map[string]FlowNode, len(f.Nodes))
	for _, n := range f.Nodes {
		nodeByID[n.ID] = n
	}

	// Build adjacency map including middleware edges (for traversal)
	adj := make(map[string][]FlowEdge)
	for _, e := range f.Edges {
		adj[e.From] = append(adj[e.From], e)
	}
	for k := range adj {
		sort.Slice(adj[k], func(i, j int) bool {
			li, lj := adj[k][i].Line, adj[k][j].Line
			if li != lj && li > 0 && lj > 0 {
				return li < lj
			}
			return adj[k][i].To < adj[k][j].To
		})
	}

	// Middleware map: node ID → middleware node IDs (recursive chain)
	mwIDs := make(map[string]bool)
	for _, n := range f.Nodes {
		if n.Role == RoleMiddleware {
			mwIDs[n.ID] = true
		}
	}
	mwFor := make(map[string][]string)
	for _, e := range f.Edges {
		if e.Kind == "middleware" {
			mwFor[e.To] = append(mwFor[e.To], e.From)
		}
	}

	var resolveMWChain func(id string, visited map[string]bool) []string
	resolveMWChain = func(id string, visited map[string]bool) []string {
		if visited[id] {
			return nil
		}
		visited[id] = true
		var chain []string
		for _, mwID := range mwFor[id] {
			name := mwID
			if n, ok := nodeByID[mwID]; ok {
				name = n.Name
			}
			chain = append(chain, name)
			chain = append(chain, resolveMWChain(mwID, visited)...)
		}
		return chain
	}

	// DFS to collect participant order
	var participants []string
	seenActor := make(map[string]bool)
	seenNode := make(map[string]bool)
	var dfsParticipants func(id string)
	dfsParticipants = func(id string) {
		if seenNode[id] {
			return
		}
		seenNode[id] = true
		actor := actorForNode[id]
		if actor == "" {
			actor = "Backend"
		}
		if !seenActor[actor] {
			seenActor[actor] = true
			participants = append(participants, actor)
		}
		for _, e := range adj[id] {
			dfsParticipants(e.To)
		}
	}
	dfsParticipants(f.Entry)

	// DFS to produce messages
	type seqMsg struct {
		from, to, label string
		isNote          bool
		isConditional   bool
	}
	var messages []seqMsg
	seenMsg := make(map[string]bool)
	var dfsMessages func(fromID string)
	dfsMessages = func(fromID string) {
		if seenMsg[fromID] {
			return
		}
		seenMsg[fromID] = true

		// Emit middleware guard notes
		if _, isMW := mwIDs[fromID]; !isMW {
			if chain := resolveMWChain(fromID, make(map[string]bool)); len(chain) > 0 {
				for _, mwName := range chain {
					messages = append(messages, seqMsg{
						isNote: true,
						label:  fmt.Sprintf("guarded by %s", sanitizeLabel(mwName)),
					})
				}
			}
		}

		for _, e := range adj[fromID] {
			if mwIDs[fromID] || mwIDs[e.To] {
				dfsMessages(e.To)
				continue
			}

			toActor := actorForNode[e.To]
			if toActor == "" {
				toActor = "Backend"
			}
			fromActor := actorForNode[fromID]
			if fromActor == "" {
				fromActor = "Backend"
			}
			// Only emit cross-actor messages
			if fromActor != toActor {
				label := e.Kind
				if e.Kind == "http" && f.Entry != "" {
					label = f.Entry
				}
				messages = append(messages, seqMsg{
					from:          fromActor,
					to:            toActor,
					label:         sanitizeLabel(label),
					isConditional: e.Conditional,
				})
				// Return arrow
				messages = append(messages, seqMsg{
					from: toActor,
					to:   fromActor,
					label: "ok",
				})
			}
			dfsMessages(e.To)
		}
	}
	dfsMessages(f.Entry)

	// Render Mermaid
	var sb strings.Builder
	sb.WriteString("sequenceDiagram\n")

	seenAlias := make(map[string]bool)
	for _, actor := range participants {
		if seenAlias[actor] {
			continue
		}
		seenAlias[actor] = true
		if actor == "Client" {
			sb.WriteString("    participant Client\n")
		} else {
			sb.WriteString(fmt.Sprintf("    participant %s as \"%s\"\n", sanitizeID(actor), actor))
		}
	}
	sb.WriteString("\n")

	for _, m := range messages {
		if m.isNote {
			sb.WriteString(fmt.Sprintf("    Note over Backend: %s\n", m.label))
			continue
		}
		if m.isConditional {
			sb.WriteString("    opt conditional\n")
			sb.WriteString(fmt.Sprintf("    %s->>%s: %s\n", m.from, m.to, m.label))
			sb.WriteString("    end\n")
			continue
		}
		if m.label == "ok" {
			sb.WriteString(fmt.Sprintf("    %s-->>%s: %s\n", m.from, m.to, m.label))
		} else {
			sb.WriteString(fmt.Sprintf("    %s->>%s: %s\n", m.from, m.to, m.label))
		}
	}

	return sb.String()
}
