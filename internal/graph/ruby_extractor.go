package graph

import (
	"fmt"
	"path/filepath"
	"strings"

	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

const rubyGraphContainsQuery = `
(method name: (identifier) @name) @decl
(singleton_method name: (identifier) @name) @decl
(class name: (constant) @name) @decl
(class name: (scope_resolution) @name) @decl
(module name: (constant) @name) @decl
(module name: (scope_resolution) @name) @decl
`

const rubyGraphCallFuncQuery = `
(method name: (identifier) @fn_name body: (body_statement) @body) @fn_decl
(singleton_method name: (identifier) @fn_name body: (body_statement) @body) @fn_decl
`

const rubyGraphCallExprQuery = `
(call (identifier) @callee)
(call (constant) (identifier) @callee)
(identifier) @callee
`

type RubyGraphExtractor struct {
	lang          *gotreesitter.Language
	containsQuery *gotreesitter.Query
	callFuncQuery *gotreesitter.Query
	callExprQuery *gotreesitter.Query
}

func NewRubyGraphExtractor() (*RubyGraphExtractor, error) {
	lang := grammars.RubyLanguage()

	cq, err := gotreesitter.NewQuery(rubyGraphContainsQuery, lang)
	if err != nil {
		return nil, fmt.Errorf("ruby contains query: %w", err)
	}
	fq, err := gotreesitter.NewQuery(rubyGraphCallFuncQuery, lang)
	if err != nil {
		return nil, fmt.Errorf("ruby call func query: %w", err)
	}
	eq, err := gotreesitter.NewQuery(rubyGraphCallExprQuery, lang)
	if err != nil {
		return nil, fmt.Errorf("ruby call expr query: %w", err)
	}
	return &RubyGraphExtractor{
		lang:          lang,
		containsQuery: cq,
		callFuncQuery: fq,
		callExprQuery: eq,
	}, nil
}

func (e *RubyGraphExtractor) Supports(ext string) bool {
	return ext == ".rb"
}

func (e *RubyGraphExtractor) RequiresFrameworks() []string {
	return []string{"rails"}
}

func (e *RubyGraphExtractor) ExtractEdges(filePath string, content []byte) ([]Edge, error) {
	parser := gotreesitter.NewParser(e.lang)
	tree, err := parser.Parse(content)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", filePath, err)
	}
	bt := gotreesitter.Bind(tree)
	defer bt.Release()

	relFile := filepath.ToSlash(filePath)

	var edges []Edge
	edges = append(edges, e.extractContains(bt, tree, content, relFile)...)
	edges = append(edges, e.extractCalls(bt, tree, content, relFile)...)
	return edges, nil
}

func (e *RubyGraphExtractor) extractContains(bt *gotreesitter.BoundTree, tree *gotreesitter.Tree, content []byte, filePath string) []Edge {
	matches := e.containsQuery.Execute(tree)
	var edges []Edge
	seen := map[string]bool{}

	for _, match := range matches {
		var nameNode *gotreesitter.Node
		var declNode *gotreesitter.Node
		for _, cap := range match.Captures {
			switch cap.Name {
			case "name":
				nameNode = cap.Node
			case "decl":
				declNode = cap.Node
			}
		}
		if nameNode == nil {
			continue
		}
		fullName := extractRubyFullName(bt, nameNode)
		name := fullName
		if idx := strings.LastIndex(name, "::"); idx >= 0 {
			name = name[idx+2:]
		}

		isMethod := declNode != nil && (bt.NodeType(declNode) == "method" || bt.NodeType(declNode) == "singleton_method")
		targetNode := filePath + "::" + name
		if isMethod {
			className := extractEnclosingClass(bt, declNode, e.lang)
			if className != "" {
				targetNode = filePath + "::" + className + "#" + name
			}
		}

		if seen[targetNode] {
			continue
		}
		seen[targetNode] = true
		edges = append(edges, Edge{
			SourceNode: filePath,
			TargetNode: targetNode,
			Kind:       EdgeContains,
			SourceFile: filePath,
			Line:       lineForByte(content, nameNode.StartByte()),
			Language:   "ruby",
		})
	}
	return edges
}

func extractRubyFullName(bt *gotreesitter.BoundTree, n *gotreesitter.Node) string {
	if bt.NodeType(n) != "scope_resolution" {
		return bt.NodeText(n)
	}
	var parts []string
	for i := 0; i < int(n.ChildCount()); i++ {
		child := n.Child(i)
		if child == nil {
			continue
		}
		text := bt.NodeText(child)
		if text != "::" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "::")
}

func extractEnclosingClass(bt *gotreesitter.BoundTree, n *gotreesitter.Node, lang *gotreesitter.Language) string {
	for p := n.Parent(); p != nil; p = p.Parent() {
		nodeType := bt.NodeType(p)
		if nodeType == "class" || nodeType == "module" {
			nameNode := p.ChildByFieldName("name", lang)
			if nameNode == nil {
				continue
			}
			return extractRubyFullName(bt, nameNode)
		}
	}
	return ""
}

func (e *RubyGraphExtractor) extractCalls(bt *gotreesitter.BoundTree, tree *gotreesitter.Tree, content []byte, filePath string) []Edge {
	funcMatches := e.callFuncQuery.Execute(tree)
	var funcs []funcRange
	funcClass := map[string]string{}
	methodNames := map[string]bool{}
	for _, match := range funcMatches {
		var name string
		var declNode *gotreesitter.Node
		for _, cap := range match.Captures {
			switch cap.Name {
			case "fn_name":
				name = bt.NodeText(cap.Node)
			case "fn_decl":
				declNode = cap.Node
			}
		}
		if name != "" && declNode != nil {
			methodNames[name] = true
			funcs = append(funcs, funcRange{
				name:      name,
				startByte: declNode.StartByte(),
				endByte:   declNode.EndByte(),
			})
			if className := extractEnclosingClass(bt, declNode, e.lang); className != "" {
				funcClass[name] = className
			}
		}
	}

	if len(funcs) == 0 {
		return nil
	}

	callMatches := e.callExprQuery.Execute(tree)
	seen := map[string]bool{}
	var edges []Edge

	for _, match := range callMatches {
		for _, cap := range match.Captures {
			if cap.Name != "callee" {
				continue
			}
			calleeNode := cap.Node
			callee := bt.NodeText(calleeNode)
			if callee == "" || callee == "self" {
				continue
			}
			callByte := calleeNode.StartByte()

			enclosing := enclosingFunc(funcs, callByte)
			if enclosing == "" {
				continue
			}
			key := enclosing + "->" + callee
			if seen[key] {
				continue
			}
			seen[key] = true
			sourceNode := filePath + "::" + enclosing
			if className := funcClass[enclosing]; className != "" {
				sourceNode = filePath + "::" + className + "#" + enclosing
			}
			edges = append(edges, Edge{
				SourceNode: sourceNode,
				TargetNode: callee,
				Kind:       EdgeCalls,
				SourceFile: filePath,
				Line:       lineForByte(content, callByte),
				Language:   "ruby",
			})
		}
	}
	return edges
}
