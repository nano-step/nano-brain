package graph

import (
	"fmt"
	"path/filepath"

	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

const rubyGraphContainsQuery = `
(method name: (identifier) @name) @decl
(singleton_method name: (identifier) @name) @decl
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
		for _, cap := range match.Captures {
			if cap.Name == "name" {
				nameNode = cap.Node
			}
		}
		if nameNode == nil {
			continue
		}
		name := bt.NodeText(nameNode)
		if seen[name] {
			continue
		}
		seen[name] = true
		edges = append(edges, Edge{
			SourceNode: filePath,
			TargetNode: filePath + "::" + name,
			Kind:       EdgeContains,
			SourceFile: filePath,
			Line:       lineForByte(content, nameNode.StartByte()),
			Language:   "ruby",
		})
	}
	return edges
}

func (e *RubyGraphExtractor) extractCalls(bt *gotreesitter.BoundTree, tree *gotreesitter.Tree, content []byte, filePath string) []Edge {
	funcMatches := e.callFuncQuery.Execute(tree)
	var funcs []funcRange
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

			if !methodNames[callee] {
				continue
			}

			enclosing := enclosingFunc(funcs, callByte)
			if enclosing == "" {
				continue
			}
			key := enclosing + "->" + callee
			if seen[key] {
				continue
			}
			seen[key] = true
			edges = append(edges, Edge{
				SourceNode: filePath + "::" + enclosing,
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
