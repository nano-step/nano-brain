package graph

import (
	"strings"

	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
	zerolog "github.com/rs/zerolog"
)

type RubyCrossFileResolver struct {
	classIndex    *RubyClassIndex
	lang          *gotreesitter.Language
	logger        zerolog.Logger
	useFallback   bool
}

func NewRubyCrossFileResolver(classIndex *RubyClassIndex, logger zerolog.Logger) *RubyCrossFileResolver {
	return &RubyCrossFileResolver{
		classIndex:  classIndex,
		lang:        grammars.RubyLanguage(),
		logger:      logger,
		useFallback: true,
	}
}

func NewRubyCrossFileResolverNoFallback(classIndex *RubyClassIndex, logger zerolog.Logger) *RubyCrossFileResolver {
	return &RubyCrossFileResolver{
		classIndex:  classIndex,
		lang:        grammars.RubyLanguage(),
		logger:      logger,
		useFallback: false,
	}
}

func (r *RubyCrossFileResolver) ResolveEdges(edges []Edge, fileContents map[string][]byte) []Edge {
	seen := map[string]bool{}
	result := make([]Edge, 0, len(edges))
	result = append(result, edges...)

	for _, e := range edges {
		seen[e.SourceNode+"->"+e.TargetNode] = true
	}

	for filePath, content := range fileContents {
		newEdges := r.resolveFileAST(filePath, content)
		for _, e := range newEdges {
			key := e.SourceNode + "->" + e.TargetNode
			if seen[key] {
				continue
			}
			seen[key] = true
			result = append(result, e)
		}
	}

	return result
}

func (r *RubyCrossFileResolver) resolveFileAST(filePath string, content []byte) []Edge {
	parser := gotreesitter.NewParser(r.lang)
	tree, err := parser.Parse(content)
	if err != nil {
		return nil
	}
	bt := gotreesitter.Bind(tree)
	defer bt.Release()

	var edges []Edge
	r.walkCallNodes(bt, tree.RootNode(), filePath, content, &edges)
	return edges
}

func (r *RubyCrossFileResolver) walkCallNodes(bt *gotreesitter.BoundTree, node *gotreesitter.Node, filePath string, content []byte, edges *[]Edge) {
	if node == nil {
		return
	}
	if node.Type(r.lang) == "call" {
		r.processCallNode(bt, node, filePath, content, edges)
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		r.walkCallNodes(bt, node.Child(i), filePath, content, edges)
	}
}

func (r *RubyCrossFileResolver) processCallNode(bt *gotreesitter.BoundTree, callNode *gotreesitter.Node, filePath string, content []byte, edges *[]Edge) {
	methodNode := callNode.ChildByFieldName("method", r.lang)
	if methodNode == nil {
		return
	}
	methodName := bt.NodeText(methodNode)

	if methodName == "new" {
		return
	}

	receiverNode := callNode.ChildByFieldName("receiver", r.lang)
	className := extractReceiverClassName(bt, receiverNode, r.lang)

	if className == "" {
		return
	}

	enclosingClass := extractEnclosingClass(bt, callNode, r.lang)
	enclosingMethod := extractEnclosingMethod(bt, callNode, r.lang)

	sourceNode := filePath + "::" + enclosingClass + "#" + enclosingMethod
	line := lineForByte(content, callNode.StartByte())

	var entries []classEntry
	if r.useFallback {
		entries = r.classIndex.Lookup(className)
	} else {
		entries = r.classIndex.LookupStrict(className)
	}

	if len(entries) == 0 {
		*edges = append(*edges, Edge{
			SourceNode: sourceNode,
			TargetNode: methodName,
			Kind:       EdgeCalls,
			SourceFile: filePath,
			Line:       line,
			Language:   "ruby",
			Metadata:   map[string]any{"unresolved": true},
		})
		return
	}

	ambiguous := len(entries) > 1
	for _, entry := range entries {
		e := Edge{
			SourceNode: sourceNode,
			TargetNode: entry.FilePath + "::" + methodName,
			Kind:       EdgeCalls,
			SourceFile: filePath,
			Line:       line,
			Language:   "ruby",
		}
		if ambiguous {
			e.Metadata = map[string]any{"ambiguous": true}
		}
		*edges = append(*edges, e)
	}
}

func extractReceiverClassName(bt *gotreesitter.BoundTree, node *gotreesitter.Node, lang *gotreesitter.Language) string {
	if node == nil {
		return ""
	}
	switch node.Type(lang) {
	case "constant":
		return bt.NodeText(node)
	case "scope_resolution":
		return scopeResolutionText(bt, node)
	case "call":
		inner := node.ChildByFieldName("receiver", lang)
		return extractReceiverClassName(bt, inner, lang)
	default:
		return ""
	}
}

func extractEnclosingMethod(bt *gotreesitter.BoundTree, n *gotreesitter.Node, lang *gotreesitter.Language) string {
	for p := n.Parent(); p != nil; p = p.Parent() {
		nodeType := p.Type(lang)
		if nodeType == "method" || nodeType == "singleton_method" {
			nameNode := p.ChildByFieldName("name", lang)
			if nameNode != nil {
				return bt.NodeText(nameNode)
			}
		}
	}
	return ""
}

func (r *RubyCrossFileResolver) BuildReconcileEdges(edges []Edge) []Edge {
	var result []Edge

	for _, e := range edges {
		if e.Kind != EdgeHTTP {
			continue
		}
		handler := e.TargetNode
		parts := strings.SplitN(handler, "#", 2)
		if len(parts) != 2 {
			continue
		}
		ctrlShort := parts[0]
		action := parts[1]

		ctrlName := ctrlShort
		if idx := strings.LastIndex(ctrlShort, "::"); idx >= 0 {
			ctrlName = ctrlShort[idx+2:]
		}

		entries := r.classIndex.Lookup(ctrlShort)
		if len(entries) == 0 {
			entries = r.classIndex.Lookup(ctrlName)
		}
		if len(entries) == 0 {
			continue
		}

		for _, entry := range entries {
			result = append(result, Edge{
				SourceNode: handler,
				TargetNode: entry.FilePath + "::" + ctrlShort + "#" + action,
				Kind:       EdgeReconcile,
				SourceFile: e.SourceFile,
				Line:       e.Line,
				Language:   "ruby",
			})
		}
	}

	return result
}
