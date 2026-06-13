package graph

import (
	"fmt"
	"path/filepath"
	"strings"

	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

// echoVerbs is the set of Echo HTTP verb method names we recognise.
var echoVerbs = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "DELETE": true,
	"PATCH": true, "HEAD": true, "OPTIONS": true,
}

// EchoRouteExtractor implements graph.Extractor for Echo HTTP route registration.
// It emits EdgeHTTP and EdgeMiddleware edges by walking the Go AST via tree-sitter.
type EchoRouteExtractor struct {
	lang *gotreesitter.Language
}

// NewEchoRouteExtractor constructs a ready-to-use EchoRouteExtractor.
func NewEchoRouteExtractor() (*EchoRouteExtractor, error) {
	return &EchoRouteExtractor{lang: grammars.GoLanguage()}, nil
}

// Supports returns true for ".go" files.
func (e *EchoRouteExtractor) Supports(ext string) bool {
	return ext == ".go"
}

// ExtractEdges parses content as Go source and returns http + middleware edges
// for any Echo route/group/Use registrations found.
func (e *EchoRouteExtractor) ExtractEdges(filePath string, content []byte) ([]Edge, error) {
	parser := gotreesitter.NewParser(e.lang)
	tree, err := parser.Parse(content)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", filePath, err)
	}
	bt := gotreesitter.Bind(tree)
	defer bt.Release()

	relFile := filepath.ToSlash(filePath)
	lang := e.lang
	root := tree.RootNode()

	// Pre-pass 1: build callStart → assignedVar map by walking short_var_declaration nodes.
	// This tells us, for a call_expression at byte offset X, which variable it is assigned to.
	// e.g.  g := e.Group("/api")  →  callStart(Group call) → "g"
	callToVar := map[uint32]string{}
	walkNodes(root, lang, "short_var_declaration", func(n *gotreesitter.Node) {
		// short_var_declaration: left = expression_list, right = expression_list
		left := n.ChildByFieldName("left", lang)
		right := n.ChildByFieldName("right", lang)
		if left == nil || right == nil {
			return
		}
		// Get first identifier on left.
		varName := ""
		for i := 0; i < int(left.ChildCount()); i++ {
			child := left.Child(i)
			if child.Type(lang) == "identifier" {
				varName = bt.NodeText(child)
				break
			}
		}
		if varName == "" {
			return
		}
		// Get first call_expression on right.
		for i := 0; i < int(right.ChildCount()); i++ {
			child := right.Child(i)
			if child.Type(lang) == "call_expression" {
				callToVar[child.StartByte()] = varName
				break
			}
		}
	})

	// groupPrefix maps a local variable name to its accumulated path prefix.
	groupPrefix := map[string]string{}
	// groupMiddleware maps a receiver variable name to its Use()-registered middleware.
	groupMiddleware := map[string][]string{}

	var edges []Edge

	// Main walk: visit every call_expression.
	walkNodes(root, lang, "call_expression", func(callNode *gotreesitter.Node) {
		fnNode := callNode.ChildByFieldName("function", lang)
		if fnNode == nil || fnNode.Type(lang) != "selector_expression" {
			return
		}

		fieldNode := fnNode.ChildByFieldName("field", lang)
		operandNode := fnNode.ChildByFieldName("operand", lang)
		if fieldNode == nil || operandNode == nil {
			return
		}

		methodName := bt.NodeText(fieldNode)
		recvName := leafIdentText(bt, operandNode, lang)

		argsNode := callNode.ChildByFieldName("arguments", lang)
		if argsNode == nil {
			return
		}

		switch {
		case methodName == "Group":
			varName, ok := callToVar[callNode.StartByte()]
			if !ok {
				return
			}
			prefix := echoStringArg(bt, argsNode, lang, 0)
			groupPrefix[varName] = groupPrefix[recvName] + prefix
			// Trailing args to Group() are group-level middleware.
			mws := echoArgNames(bt, argsNode, lang, 1)
			groupMiddleware[varName] = append(groupMiddleware[varName], mws...)

		case methodName == "Use":
			mws := echoArgNames(bt, argsNode, lang, 0)
			groupMiddleware[recvName] = append(groupMiddleware[recvName], mws...)

		case echoVerbs[methodName]:
			rawPath := echoStringArg(bt, argsNode, lang, 0)
			if rawPath == "" {
				return
			}
			fullPath := groupPrefix[recvName] + rawPath
			line := lineForByte(content, callNode.StartByte())

			handlerNode := echoArgNode(argsNode, lang, 1)
			handlerName := echoHandlerName(bt, handlerNode, lang, methodName, fullPath)
			entryNode := methodName + " " + fullPath

			edges = append(edges, Edge{
				SourceNode: entryNode,
				TargetNode: handlerName,
				Kind:       EdgeHTTP,
				SourceFile: relFile,
				Line:       line,
				Language:   "go",
				Metadata: map[string]any{
					"method": methodName,
					"path":   fullPath,
				},
			})

			// Collect all applicable middleware: group-scoped + per-route trailing.
			allMWs := append([]string{}, groupMiddleware[recvName]...)
			allMWs = append(allMWs, echoArgNames(bt, argsNode, lang, 2)...)
			for _, mw := range allMWs {
				if mw == "" {
					continue
				}
				edges = append(edges, Edge{
					SourceNode: mw,
					TargetNode: handlerName,
					Kind:       EdgeMiddleware,
					SourceFile: relFile,
					Line:       line,
					Language:   "go",
				})
			}
		}
	})

	return edges, nil
}

// ─── AST helpers ─────────────────────────────────────────────────────────────

// walkNodes calls fn for every node of the given type in the subtree (pre-order).
func walkNodes(node *gotreesitter.Node, lang *gotreesitter.Language, nodeType string, fn func(*gotreesitter.Node)) {
	if node == nil {
		return
	}
	if node.Type(lang) == nodeType {
		fn(node)
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		walkNodes(node.Child(i), lang, nodeType, fn)
	}
}

// leafIdentText returns the rightmost identifier text from a potentially chained
// selector (e.g. "s.echo" → "echo"; "e" → "e").
func leafIdentText(bt *gotreesitter.BoundTree, node *gotreesitter.Node, lang *gotreesitter.Language) string {
	if node == nil {
		return ""
	}
	switch node.Type(lang) {
	case "identifier":
		return bt.NodeText(node)
	case "selector_expression":
		field := node.ChildByFieldName("field", lang)
		if field != nil {
			return bt.NodeText(field)
		}
	}
	return bt.NodeText(node)
}

// echoStringArg returns the unquoted string value of the nth positional argument
// in an argument_list node (skipping punctuation tokens).
func echoStringArg(bt *gotreesitter.BoundTree, argList *gotreesitter.Node, lang *gotreesitter.Language, n int) string {
	node := echoArgNode(argList, lang, n)
	if node == nil {
		return ""
	}
	t := node.Type(lang)
	if t == "interpreted_string_literal" || t == "raw_string_literal" {
		return strings.Trim(bt.NodeText(node), "`\"")
	}
	return ""
}

// echoArgNode returns the nth value node inside an argument_list (skipping
// comma/paren punctuation tokens).
func echoArgNode(argList *gotreesitter.Node, lang *gotreesitter.Language, n int) *gotreesitter.Node {
	if argList == nil {
		return nil
	}
	idx := 0
	for i := 0; i < int(argList.ChildCount()); i++ {
		child := argList.Child(i)
		t := child.Type(lang)
		if t == "," || t == "(" || t == ")" {
			continue
		}
		if idx == n {
			return child
		}
		idx++
	}
	return nil
}

// echoArgNames extracts bare symbol names from arguments starting at startIdx
// (0-based, after skipping punctuation).
func echoArgNames(bt *gotreesitter.BoundTree, argList *gotreesitter.Node, lang *gotreesitter.Language, startIdx int) []string {
	if argList == nil {
		return nil
	}
	var names []string
	idx := 0
	for i := 0; i < int(argList.ChildCount()); i++ {
		child := argList.Child(i)
		t := child.Type(lang)
		if t == "," || t == "(" || t == ")" {
			continue
		}
		if idx >= startIdx {
			if name := echoSymbolName(bt, child, lang); name != "" {
				names = append(names, name)
			}
		}
		idx++
	}
	return names
}

// echoSymbolName extracts a bare function/middleware name from an expression node.
func echoSymbolName(bt *gotreesitter.BoundTree, node *gotreesitter.Node, lang *gotreesitter.Language) string {
	if node == nil {
		return ""
	}
	switch node.Type(lang) {
	case "identifier":
		return bt.NodeText(node)
	case "selector_expression":
		field := node.ChildByFieldName("field", lang)
		if field != nil {
			return bt.NodeText(field)
		}
	case "call_expression":
		fn := node.ChildByFieldName("function", lang)
		if fn != nil {
			return echoSymbolName(bt, fn, lang)
		}
	}
	return ""
}

// echoHandlerName derives the bare handler name from a handler argument node.
// For inline closures it returns a synthetic "<inline:METHOD path>" name so the
// edge is never emitted with an empty target.
func echoHandlerName(bt *gotreesitter.BoundTree, node *gotreesitter.Node, lang *gotreesitter.Language, method, path string) string {
	if node == nil {
		return "<inline:" + method + " " + path + ">"
	}
	switch node.Type(lang) {
	case "func_literal":
		return "<inline:" + method + " " + path + ">"
	case "identifier":
		return bt.NodeText(node)
	case "selector_expression":
		// Method value: h.HandleGraph or handlers.WriteDocument (not called yet).
		field := node.ChildByFieldName("field", lang)
		if field != nil {
			return bt.NodeText(field)
		}
		return bt.NodeText(node)
	case "call_expression":
		// Factory call: handlers.WriteDocument(...) or WriteDocument(...)
		fn := node.ChildByFieldName("function", lang)
		if fn != nil {
			return echoHandlerName(bt, fn, lang, method, path)
		}
		return bt.NodeText(node)
	}
	name := bt.NodeText(node)
	if name == "" {
		return "<inline:" + method + " " + path + ">"
	}
	return name
}
