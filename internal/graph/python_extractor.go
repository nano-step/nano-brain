package graph

import (
	"fmt"
	"path/filepath"
	"strings"

	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

const pyGraphImportQuery = `
(import_statement name: (dotted_name) @path)
(import_from_statement module_name: (dotted_name) @path)
`

const pyGraphContainsQuery = `
(function_definition name: (identifier) @name) @decl
(class_definition name: (identifier) @name) @decl
`

const pyGraphAssignQuery = `
(assignment left: (identifier) @name) @decl
`

const pyGraphCallFuncQuery = `
(function_definition name: (identifier) @fn_name body: (block) @body) @fn_decl
`

const pyGraphCallExprQuery = `
(call function: (identifier) @callee)
(call function: (attribute attribute: (identifier) @callee))
`

type PythonGraphExtractor struct {
	lang          *gotreesitter.Language
	importQuery   *gotreesitter.Query
	containsQuery *gotreesitter.Query
	assignQuery   *gotreesitter.Query
	callFuncQuery *gotreesitter.Query
	callExprQuery *gotreesitter.Query
}

func NewPythonGraphExtractor() (*PythonGraphExtractor, error) {
	lang := grammars.PythonLanguage()

	iq, err := gotreesitter.NewQuery(pyGraphImportQuery, lang)
	if err != nil {
		return nil, fmt.Errorf("py import query: %w", err)
	}
	cq, err := gotreesitter.NewQuery(pyGraphContainsQuery, lang)
	if err != nil {
		return nil, fmt.Errorf("py contains query: %w", err)
	}
	aq, err := gotreesitter.NewQuery(pyGraphAssignQuery, lang)
	if err != nil {
		return nil, fmt.Errorf("py assign query: %w", err)
	}
	fq, err := gotreesitter.NewQuery(pyGraphCallFuncQuery, lang)
	if err != nil {
		return nil, fmt.Errorf("py call func query: %w", err)
	}
	eq, err := gotreesitter.NewQuery(pyGraphCallExprQuery, lang)
	if err != nil {
		return nil, fmt.Errorf("py call expr query: %w", err)
	}
	return &PythonGraphExtractor{
		lang:          lang,
		importQuery:   iq,
		containsQuery: cq,
		assignQuery:   aq,
		callFuncQuery: fq,
		callExprQuery: eq,
	}, nil
}

func (e *PythonGraphExtractor) Supports(ext string) bool {
	return ext == ".py"
}

func (e *PythonGraphExtractor) ExtractEdges(filePath string, content []byte) ([]Edge, error) {
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
	edges = append(edges, e.extractImports(bt, tree, content, relFile)...)
	edges = append(edges, e.extractCalls(bt, tree, content, relFile)...)
	return edges, nil
}

func (e *PythonGraphExtractor) extractContains(bt *gotreesitter.BoundTree, tree *gotreesitter.Tree, content []byte, filePath string) []Edge {
	var edges []Edge
	seen := map[string]bool{}

	matches := e.containsQuery.Execute(tree)
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
			Language:   "python",
		})
	}

	assignMatches := e.assignQuery.Execute(tree)
	for _, match := range assignMatches {
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
		if nameNode == nil || declNode == nil {
			continue
		}
		parent := declNode.Parent()
		if parent == nil || parent.Type(e.lang) != "module" {
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
			Language:   "python",
		})
	}

	return edges
}

func (e *PythonGraphExtractor) extractImports(bt *gotreesitter.BoundTree, tree *gotreesitter.Tree, content []byte, filePath string) []Edge {
	matches := e.importQuery.Execute(tree)
	var edges []Edge
	seen := map[string]bool{}

	for _, match := range matches {
		for _, cap := range match.Captures {
			if cap.Name != "path" {
				continue
			}
			importPath := bt.NodeText(cap.Node)
			importPath = strings.ReplaceAll(importPath, " ", "")
			if importPath == "" || seen[importPath] {
				continue
			}
			seen[importPath] = true
			edges = append(edges, Edge{
				SourceNode: filePath,
				TargetNode: importPath,
				Kind:       EdgeImports,
				SourceFile: filePath,
				Line:       lineForByte(content, cap.Node.StartByte()),
				Language:   "python",
			})
		}
	}
	return edges
}

func (e *PythonGraphExtractor) extractCalls(bt *gotreesitter.BoundTree, tree *gotreesitter.Tree, content []byte, filePath string) []Edge {
	funcMatches := e.callFuncQuery.Execute(tree)
	var funcs []funcRange
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
			callByte := cap.Node.StartByte()
			callee := bt.NodeText(cap.Node)
			if callee == "" {
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
				Line:       lineForByte(content, cap.Node.StartByte()),
				Language:   "python",
			})
		}
	}
	return edges
}
