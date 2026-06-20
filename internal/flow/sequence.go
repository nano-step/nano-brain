package flow

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/nano-brain/nano-brain/internal/graph"
)

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

const (
	maxCFGDepth    = 3
	maxSeqMessages = 50
)

func renderInternalLogic(
	entryID string,
	nodes map[string]graph.CFGNode,
	adj map[string][]graph.CFGEdge,
	actor string,
	depth int,
	msgCount *int,
	visited map[string]bool,
) []string {
	if *msgCount >= maxSeqMessages {
		return nil
	}
	if depth > maxCFGDepth {
		return nil
	}

	node, ok := nodes[entryID]
	if !ok {
		return nil
	}
	if visited[entryID] && node.Type != "merge" {
		return nil
	}
	visited[entryID] = true

	var lines []string
	selfMsg := func(label string) {
		if *msgCount >= maxSeqMessages {
			return
		}
		*msgCount++
		safeActor := sanitizeID(actor)
		lines = append(lines, fmt.Sprintf("    %s->>%s: %s", safeActor, safeActor, sanitizeLabel(label)))
	}

	switch node.Type {
	case "step", "merge", "start":
		if cleaned, ok := simplifyStepLabel(node.Label); ok {
			selfMsg(cleaned)
		}
		for _, e := range adj[entryID] {
			lines = append(lines, renderInternalLogic(e.To, nodes, adj, actor, depth, msgCount, visited)...)
			if *msgCount >= maxSeqMessages {
				break
			}
		}

	case "decision":
		edges := adj[entryID]
		if hasYesNoEdges(edges) {
			lines = append(lines, renderAltBlock(edges, nodes, adj, actor, depth, msgCount, visited)...)
		} else if hasLoopEdge(edges) {
			lines = append(lines, renderLoopBlock(edges, nodes, adj, actor, depth, msgCount, visited)...)
		} else if len(edges) == 1 {
			selfMsg(node.Label)
			lines = append(lines, renderInternalLogic(edges[0].To, nodes, adj, actor, depth, msgCount, visited)...)
		} else {
			selfMsg(node.Label)
			for _, e := range edges {
				lines = append(lines, renderInternalLogic(e.To, nodes, adj, actor, depth, msgCount, visited)...)
				if *msgCount >= maxSeqMessages {
					break
				}
			}
		}

	case "terminal":
		if node.Kind == "error" {
			selfMsg(fmt.Sprintf("throw %s", node.Label))
		} else {
			selfMsg(node.Label)
		}
	}

	return lines
}

func hasYesNoEdges(edges []graph.CFGEdge) bool {
	hasYes, hasNo := false, false
	for _, e := range edges {
		if e.Branch == "yes" {
			hasYes = true
		}
		if e.Branch == "no" {
			hasNo = true
		}
	}
	return hasYes && hasNo
}

func hasLoopEdge(edges []graph.CFGEdge) bool {
	for _, e := range edges {
		if e.Branch == "loop" {
			return true
		}
	}
	return false
}

func renderAltBlock(
	edges []graph.CFGEdge,
	nodes map[string]graph.CFGNode,
	adj map[string][]graph.CFGEdge,
	actor string,
	depth int,
	msgCount *int,
	visited map[string]bool,
) []string {
	var lines []string
	lines = append(lines, "    alt")
	for _, e := range edges {
		if *msgCount >= maxSeqMessages {
			break
		}
		label := e.Branch
		if label == "yes" {
			label = "condition"
		} else if label == "no" {
			label = "else"
		} else if strings.HasPrefix(label, "case:") {
			label = label[5:]
		}
		lines = append(lines, fmt.Sprintf("        %s", sanitizeLabel(label)))
		lines = append(lines, renderInternalLogic(e.To, nodes, adj, actor, depth+1, msgCount, visited)...)
	}
	lines = append(lines, "    end")
	return lines
}

func renderLoopBlock(
	edges []graph.CFGEdge,
	nodes map[string]graph.CFGNode,
	adj map[string][]graph.CFGEdge,
	actor string,
	depth int,
	msgCount *int,
	visited map[string]bool,
) []string {
	var lines []string
	lines = append(lines, "    loop")
	for _, e := range edges {
		if *msgCount >= maxSeqMessages {
			break
		}
		if e.Branch == "loop" {
			lines = append(lines, renderInternalLogic(e.To, nodes, adj, actor, depth+1, msgCount, visited)...)
		} else {
			lines = append(lines, fmt.Sprintf("        %s", sanitizeLabel(e.Branch)))
			lines = append(lines, renderInternalLogic(e.To, nodes, adj, actor, depth+1, msgCount, visited)...)
		}
	}
	lines = append(lines, "    end")
	return lines
}

func isLiteral(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return true
	}
	if (strings.HasPrefix(s, "'") && strings.HasSuffix(s, "'")) ||
		(strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"")) ||
		(strings.HasPrefix(s, "`") && strings.HasSuffix(s, "`")) {
		return true
	}
	if _, err := strconv.ParseFloat(s, 64); err == nil {
		return true
	}
	if s == "true" || s == "false" || s == "null" || s == "nil" || s == "undefined" {
		return true
	}
	if s == "{}" || s == "[]" {
		return true
	}
	return false
}

func rhsAfterAssignment(s string) string {
	idx := strings.Index(s, " = ")
	if idx < 0 {
		return s
	}
	return strings.TrimSpace(s[idx+3:])
}

func simplifyStepLabel(label string) (string, bool) {
	for _, prefix := range []string{"const ", "let ", "var "} {
		if strings.HasPrefix(label, prefix) {
			rhs := strings.TrimPrefix(rhsAfterAssignment(label[len(prefix):]), "await ")
			if isLiteral(rhs) {
				return "", false
			}
			return rhs, true
		}
	}
	if idx := strings.Index(label, " = "); idx >= 0 {
		rhs := strings.TrimSpace(strings.TrimPrefix(label[idx+3:], "await "))
		if isLiteral(rhs) {
			return "", false
		}
		return rhs, true
	}
	if strings.HasPrefix(label, "@") {
		if idx := strings.Index(label, " = "); idx >= 0 {
			rhs := strings.TrimSpace(strings.TrimPrefix(label[idx+3:], "await "))
			if isLiteral(rhs) {
				return "", false
			}
			return rhs, true
		}
	}
	return label, true
}

func RenderSequenceDiagramWithCFG(f Flow, cfgs FlowCFGs) string {
	actorForNode := groupActors(f, f.ServiceName)
	nodeByID := make(map[string]FlowNode, len(f.Nodes))
	for _, n := range f.Nodes {
		nodeByID[n.ID] = n
	}

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
				messages = append(messages, seqMsg{
					from:  toActor,
					to:    fromActor,
					label: "ok",
				})
			}
			dfsMessages(e.To)
		}
	}
	dfsMessages(f.Entry)

	if len(messages) > maxSeqMessages {
		messages = messages[:maxSeqMessages]
		messages = append(messages, seqMsg{
			isNote: true,
			label:  "Internal logic too complex — see full CFG at /api/v1/graph/flowchart",
		})
	}

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

	if cfgs != nil {
		for _, n := range f.Nodes {
			if n.Role == RoleHandler || n.Role == RoleFunc {
				if cfg, ok := cfgs[n.ID]; ok {
					cfgNodes := cfgNodeMap(cfg)
					cfgEdges := cfgAdj(cfg)
					var startID string
					for _, cn := range cfg.Nodes {
						if cn.Type == "start" {
							startID = cn.ID
							break
						}
					}
					if startID != "" {
						actor := actorForNode[n.ID]
						if actor == "" {
							actor = f.ServiceName
						}
						msgCount := 0
						lines := renderInternalLogic(startID, cfgNodes, cfgEdges, actor, 0, &msgCount, make(map[string]bool))
						for _, l := range lines {
							sb.WriteString(l + "\n")
						}
						if msgCount >= maxSeqMessages {
							sb.WriteString(fmt.Sprintf("    Note over %s: Internal logic too complex — see full CFG at /api/v1/graph/flowchart\n", sanitizeID(actor)))
						}
					}
				}
			}
		}
	}

	return sb.String()
}
