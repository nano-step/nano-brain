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

	// ConditionLabel holds the predicate text for conditional edges (e.g. "err !== null").
	// Populated from graph_edges.metadata["condition_label"] during BuildFlow.
	ConditionLabel string

	// CrossServiceWorkspace is set when Kind is "cross_service", indicating
	// the target workspace hash (first 8 chars) the edge connects to.
	CrossServiceWorkspace string
}

// Flow is the result of BuildFlow.
type Flow struct {
	Entry       string
	Method      string
	Path        string
	ServiceName string // derived from source file path (e.g., "tradeit-backend")
	Nodes       []FlowNode
	Edges       []FlowEdge
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
	if strings.Contains(lower, "repo") || strings.Contains(lower, "repository") || strings.Contains(lower, "store") || strings.Contains(lower, "model") {
		return RoleRepo
	}
	return RoleFunc
}

// noiseExternalPrefixes / Suffixes identify logging and language-builtin calls
// (console.log, logger.Info, log.Println, fmt.Println, …) that add noise to
// flows. They are ONLY applied to external leaf calls — any callee defined in
// the workspace is always kept. Extend these lists to tune what's hidden.
var (
	noiseExternalPrefixes = []string{
		"console.", "log.", "fmt.print", "fmt.sprint", "fmt.fprint",
		"process.stdout", "process.stderr", "system.out", "system.err",
		"rails.logger", "logger.", "puts ", "p ", "pp ",
	}
	noiseExternalSuffixes = []string{".log", ".debug", ".trace", ".info", ".warn", ".warning", ".error"}
)

// isNoiseExternal reports whether an external callee is a logging/builtin call
// that should be hidden from flows.
func isNoiseExternal(name string) bool {
	if name == "" {
		return false
	}
	lower := strings.ToLower(name)
	if strings.Contains(lower, "logger") {
		return true
	}
	for _, p := range noiseExternalPrefixes {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}
	for _, s := range noiseExternalSuffixes {
		if strings.HasSuffix(lower, s) {
			return true
		}
	}
	return false
}

// maxReconcileFiles caps how widely a bare call name may reconcile. Above this,
// the name is treated as too generic to traverse (avoids graph explosion from
// names like get/find/handle defined in dozens of files).
const maxReconcileFiles = 8

// bareLoggingNames are method names hidden from flows regardless of role: they
// are almost always logging and frequently collide with a workspace symbol of
// the same name (so they would otherwise surface as a "func" node).
var bareLoggingNames = map[string]bool{
	"log": true, "info": true, "warn": true, "warning": true,
	"error": true, "debug": true, "trace": true, "fatal": true,
	"calllog": true, "addlog": true, "addlogstripe": true, "getlogprefix": true,
}

func isLoggingName(name string) bool {
	return bareLoggingNames[strings.ToLower(name)]
}

// isNoiseIntegration reports whether an integration target is a dynamic
// placeholder (<var:…>) or a multi-line code block mis-extracted as a topic.
func isNoiseIntegration(target string) bool {
	return strings.Contains(target, "<var:") || strings.ContainsAny(target, "\n\r")
}

// sameFileIDs returns the subset of ids whose defining file equals parentFile.
// It returns nil when parentFile is empty (no scoping possible).
func sameFileIDs(ids map[string]bool, fileByID map[string]string, parentFile string) map[string]bool {
	if parentFile == "" {
		return nil
	}
	var same map[string]bool
	for id := range ids {
		if fileByID[id] == parentFile {
			if same == nil {
				same = make(map[string]bool)
			}
			same[id] = true
		}
	}
	return same
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

func edgeConditionLabel(e graph.Edge) string {
	if e.Metadata == nil {
		return ""
	}
	v, ok := e.Metadata["condition_label"]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
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
	// Each BFS item is a bare name to expand, its depth, and the file of the
	// caller that referenced it (used to scope reconciliation).
	type bfsItem struct {
		bareName   string
		depth      int
		parentFile string
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
			if exact, ok := idx.bySource[item.bareName]; ok {
				sourceNodes = exact
			}
		}

		if len(sourceNodes) == 0 {
			// Already added as external leaf when first encountered; nothing to expand.
			continue
		}

		// Collect unique source node ids (file::symbol) that call from this
		// symbol, along with the file each is defined in.
		sourceNodeIDs := make(map[string]bool)
		fileByID := make(map[string]string)
		fileSet := make(map[string]bool)
		for _, e := range sourceNodes {
			if e.Kind == graph.EdgeCalls || e.Kind == graph.EdgeReconcile {
				sourceNodeIDs[e.SourceNode] = true
				fileByID[e.SourceNode] = e.SourceFile
				fileSet[e.SourceFile] = true
			}
		}

		// Scope reconciliation: prefer a definition in the caller's own file.
		// If none match and the bare name resolves across too many files, it's a
		// generic name (get/find/handle/…) — skip it to avoid graph explosion.
		// Reconcile edges are always included (they are explicit connections).
		reconcileIDs := make(map[string]bool)
		for id := range sourceNodeIDs {
			for _, e := range sourceNodes {
				if e.SourceNode == id && e.Kind == graph.EdgeReconcile {
					reconcileIDs[id] = true
					break
				}
			}
		}
		if same := sameFileIDs(sourceNodeIDs, fileByID, item.parentFile); len(same) > 0 {
			// Merge reconcile IDs that aren't in the same-file set.
			for id := range reconcileIDs {
				same[id] = true
			}
			sourceNodeIDs = same
		} else if len(fileSet) > maxReconcileFiles {
			// Keep reconcile IDs even when generic-name filter triggers.
			if len(reconcileIDs) > 0 {
				sourceNodeIDs = reconcileIDs
			} else {
				continue
			}
		}

		for sourceID := range sourceNodeIDs {
			if visited[sourceID] {
				continue
			}
			visited[sourceID] = true
			callerFile := fileByID[sourceID]

			// Get outgoing edges from this source node.
			outEdges := idx.bySource[sourceID]
			fanout := 0
			for _, e := range outEdges {
				// Integration edges: emit as leaf nodes, never traverse further.
				if e.Kind == graph.EdgeIntegration {
					target := e.TargetNode
					// Skip dynamic placeholders (<var:…>) and multi-line code
					// blocks mis-extracted as topics — they're noise, not routes.
					if isNoiseIntegration(target) {
						continue
					}
					if _, exists := nodeMap[target]; !exists {
						addNode(FlowNode{ID: target, Name: target, Role: RoleIntegration})
					}
					addEdge(FlowEdge{From: item.bareName, To: target, Kind: "integration", Line: e.Line, Conditional: edgeConditional(e), ConditionLabel: edgeConditionLabel(e)})
					continue
				}

				if e.Kind == graph.EdgeReconcile {
					target := symbolPart(e.TargetNode)
					if _, exists := nodeMap[target]; !exists {
						addNode(FlowNode{ID: target, Name: target, Role: classifyRole(target)})
					}
					queue = append(queue, bfsItem{bareName: target, depth: item.depth, parentFile: callerFile})
					continue
				}

				if e.Kind != graph.EdgeCalls {
					continue
				}

				target := e.TargetNode

				// Determine if target is external (no source node with that symbol).
				targetSources := idx.bySymbol[target]

				// Skip logging noise before the fanout check so it doesn't consume
				// budget. Bare logging names (log/info/warn/error/debug) are hidden
				// regardless of role — they frequently collide with a workspace
				// symbol. Qualified logging/builtin externals (console.*, logger.*,
				// …) are hidden too.
				if isLoggingName(target) || (len(targetSources) == 0 && isNoiseExternal(target)) {
					continue
				}

				if fanout >= maxFanout {
					break
				}
				fanout++
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

				addEdge(FlowEdge{From: item.bareName, To: target, Kind: "calls", Line: e.Line, Conditional: edgeConditional(e), ConditionLabel: edgeConditionLabel(e)})

				if targetRole != RoleExternal {
					queue = append(queue, bfsItem{bareName: target, depth: item.depth + 1, parentFile: callerFile})
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

	// Derive service name from the source file paths of handler edges.
	if flow.ServiceName == "" {
		flow.ServiceName = deriveServiceName(edges)
	}

	return flow
}

// deriveServiceName extracts a service name from the source file paths of edges.
// It looks at the first edge's SourceFile and extracts the first path component
// after the workspace root (e.g., "tradeit-backend/server/controllers/trade.js" → "tradeit-backend").
// Handles absolute paths (Unix and Windows) by stripping the prefix before splitting.
func deriveServiceName(edges []graph.Edge) string {
	fileCounts := make(map[string]int)
	for _, e := range edges {
		if e.SourceFile == "" {
			continue
		}
		sf := e.SourceFile
		// Handle Windows drive-letter paths (e.g. "C:/Users/..." → strip "C:")
		if len(sf) >= 2 && sf[1] == ':' {
			sf = sf[2:]
		}
		// Strip leading slashes (handles both "/" and "//" and "///")
		sf = strings.TrimLeft(sf, "/")
		if sf == "" {
			continue
		}
		parts := strings.SplitN(sf, "/", 2)
		if len(parts) > 0 {
			fileCounts[parts[0]]++
		}
	}
	best := ""
	bestCount := 0
	for name, count := range fileCounts {
		if count > bestCount {
			best = name
			bestCount = count
		}
	}
	if best != "" {
		return best
	}
	return "Backend"
}
