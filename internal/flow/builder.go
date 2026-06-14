package flow

import (
	"strings"

	"github.com/nano-brain/nano-brain/internal/graph"
)

// Role classifies a node's function in the request flow.
type Role string

const (
	RoleEntry       Role = "entry"
	RoleMiddleware  Role = "middleware"
	RoleHandler     Role = "handler"
	RoleService     Role = "service"
	RoleRepo        Role = "repo"
	RoleExternal    Role = "external"
	RoleFunc        Role = "func"
	RoleIntegration Role = "integration" // outbound HTTP / queue / event call
)

// FlowNode is a node in the built flow.
type FlowNode struct {
	ID        string
	Name      string
	Role      Role
	Ambiguous bool // true when bare name reconciles to >1 source file
}

// FlowEdge is a directed edge in the built flow.
type FlowEdge struct {
	From        string
	To          string
	Kind        string // "http" | "middleware" | "calls" | "integration" | "cross_service"
	Line        int    // source line of the call site (0 = unknown)
	Conditional bool   // true when the call is inside an if/switch/select block

	// CrossServiceWorkspace is set when Kind is "cross_service", indicating
	// the target workspace hash (first 8 chars) the edge connects to.
	CrossServiceWorkspace string
}

// Flow is the result of BuildFlow.
type Flow struct {
	Entry  string
	Method string
	Path   string
	Nodes  []FlowNode
	Edges  []FlowEdge
}

// symbolPart returns the symbol portion of a source node id.
// For "file/path.go::FuncName" it returns "FuncName".
// For a bare name with no "::" it returns the whole string.
func symbolPart(sourceNode string) string {
	if idx := strings.LastIndex(sourceNode, "::"); idx >= 0 {
		return sourceNode[idx+2:]
	}
	return sourceNode
}

// classifyRole applies name heuristics to assign a role to a node.
// This is advisory — never load-bearing for traversal correctness.
func classifyRole(name string) Role {
	lower := strings.ToLower(name)
	if strings.Contains(lower, "service") || strings.Contains(lower, "svc") {
		return RoleService
	}
	if strings.Contains(lower, "repo") || strings.Contains(lower, "repository") || strings.Contains(lower, "store") {
		return RoleRepo
	}
	return RoleFunc
}

// edgeIndex holds two indices over a slice of edges:
//   - bySource: exact SourceNode → []Edge
//   - bySymbol: symbolPart(SourceNode) → []Edge (for reconciliation)
type edgeIndex struct {
	bySource map[string][]graph.Edge
	bySymbol map[string][]graph.Edge
}

func edgeConditional(e graph.Edge) bool {
	if e.Metadata == nil {
		return false
	}
	v, ok := e.Metadata["conditional"]
	if !ok {
		return false
	}
	b, ok := v.(bool)
	return ok && b
}

func buildIndex(edges []graph.Edge) edgeIndex {
	idx := edgeIndex{
		bySource: make(map[string][]graph.Edge),
		bySymbol: make(map[string][]graph.Edge),
	}
	for _, e := range edges {
		idx.bySource[e.SourceNode] = append(idx.bySource[e.SourceNode], e)
		sym := symbolPart(e.SourceNode)
		idx.bySymbol[sym] = append(idx.bySymbol[sym], e)
	}
	return idx
}

// BuildFlow builds a Flow by traversing graph edges starting from entry.
// maxDepth caps the number of `calls` hops from the handler.
// maxFanout caps the number of outgoing calls edges explored per node.
func BuildFlow(edges []graph.Edge, entry string, maxDepth, maxFanout int) Flow {
	idx := buildIndex(edges)

	flow := Flow{
		Entry: entry,
	}

	// Parse method and path from entry like "POST /api/topup".
	if parts := strings.SplitN(entry, " ", 2); len(parts) == 2 {
		flow.Method = parts[0]
		flow.Path = parts[1]
	}

	nodeMap := make(map[string]FlowNode) // id → FlowNode (dedup)
	type edgeKey struct {
		from, to, kind string
		line           int
	}
	edgeSet := make(map[edgeKey]FlowEdge)
	visited := make(map[string]bool) // resolved source node ids

	addNode := func(n FlowNode) {
		if _, exists := nodeMap[n.ID]; !exists {
			nodeMap[n.ID] = n
		}
	}
	addEdge := func(e FlowEdge) {
		key := edgeKey{from: e.From, to: e.To, kind: e.Kind, line: e.Line}
		edgeSet[key] = e
	}

	// Step 1: add the entry node.
	entryNode := FlowNode{ID: entry, Name: entry, Role: RoleEntry}
	addNode(entryNode)

	// Step 2: follow http edges from entry to handler(s).
	httpEdges := idx.bySource[entry]
	var handlerNames []string
	for _, e := range httpEdges {
		if e.Kind != graph.EdgeHTTP {
			continue
		}
		handlerName := e.TargetNode
		handlerNames = append(handlerNames, handlerName)
		handlerNode := FlowNode{ID: handlerName, Name: handlerName, Role: RoleHandler}
		addNode(handlerNode)
		addEdge(FlowEdge{From: entry, To: handlerName, Kind: "http"})
	}

	// Step 3: attach middleware edges (source → handler) as guards.
	// Middleware edges have source = middleware symbol, target = handler bare name.
	// We scan ALL edges looking for middleware edges whose target matches a handler name.
	handlerSet := make(map[string]bool)
	for _, h := range handlerNames {
		handlerSet[h] = true
	}
	for _, e := range edges {
		if e.Kind != graph.EdgeMiddleware {
			continue
		}
		if handlerSet[e.TargetNode] {
			mwNode := FlowNode{ID: e.SourceNode, Name: e.SourceNode, Role: RoleMiddleware}
			addNode(mwNode)
			addEdge(FlowEdge{From: e.SourceNode, To: e.TargetNode, Kind: "middleware"})
		}
	}

	// Step 4: BFS over calls edges with symbol-name reconciliation.
	// Each BFS item is a bare name to expand and its current depth.
	type bfsItem struct {
		bareName string
		depth    int
	}

	queue := make([]bfsItem, 0, len(handlerNames))
	for _, h := range handlerNames {
		queue = append(queue, bfsItem{bareName: h, depth: 0})
	}

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]

		if item.depth >= maxDepth {
			continue
		}

		// Reconcile bare name to source nodes.
		sourceNodes := idx.bySymbol[item.bareName]

		if len(sourceNodes) == 0 {
			// Already added as external leaf when first encountered; nothing to expand.
			continue
		}

		// Collect unique source node ids that call from this symbol.
		// We need to visit each unique source node (file::symbol).
		sourceNodeIDs := make(map[string]bool)
		for _, e := range sourceNodes {
			if e.Kind == graph.EdgeCalls {
				sourceNodeIDs[e.SourceNode] = true
			}
		}

		for sourceID := range sourceNodeIDs {
			if visited[sourceID] {
				continue
			}
			visited[sourceID] = true

			// Get outgoing edges from this source node.
			outEdges := idx.bySource[sourceID]
			fanout := 0
			for _, e := range outEdges {
				// Integration edges: emit as leaf nodes, never traverse further.
				if e.Kind == graph.EdgeIntegration {
					target := e.TargetNode
					if _, exists := nodeMap[target]; !exists {
						addNode(FlowNode{ID: target, Name: target, Role: RoleIntegration})
					}
					addEdge(FlowEdge{From: item.bareName, To: target, Kind: "integration", Line: e.Line, Conditional: edgeConditional(e)})
					continue
				}

				if e.Kind != graph.EdgeCalls {
					continue
				}
				if fanout >= maxFanout {
					break
				}
				fanout++

				target := e.TargetNode

				// Determine if target is external (no source node with that symbol).
				targetSources := idx.bySymbol[target]
				var targetRole Role
				if len(targetSources) == 0 {
					targetRole = RoleExternal
				} else {
					targetRole = classifyRole(target)
				}

				// Detect ambiguity: does this bare name appear in >1 distinct source file?
				ambiguous := false
				if len(targetSources) > 0 {
					files := make(map[string]bool)
					for _, ts := range targetSources {
						files[ts.SourceFile] = true
					}
					ambiguous = len(files) > 1
				}

				targetNode := FlowNode{
					ID:        target,
					Name:      target,
					Role:      targetRole,
					Ambiguous: ambiguous,
				}
				// If already exists with ambiguous=true, keep it; otherwise merge ambiguous flag.
				if existing, ok := nodeMap[target]; ok {
					if ambiguous && !existing.Ambiguous {
						existing.Ambiguous = true
						nodeMap[target] = existing
					}
				} else {
					addNode(targetNode)
				}

				addEdge(FlowEdge{From: item.bareName, To: target, Kind: "calls", Line: e.Line, Conditional: edgeConditional(e)})

				if targetRole != RoleExternal {
					queue = append(queue, bfsItem{bareName: target, depth: item.depth + 1})
				}
			}
		}
	}

	// Collect nodes and edges into slices.
	for _, n := range nodeMap {
		flow.Nodes = append(flow.Nodes, n)
	}
	for _, e := range edgeSet {
		flow.Edges = append(flow.Edges, e)
	}

	return flow
}
