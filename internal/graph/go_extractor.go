package graph

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"

	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

const goImportQuery = `
(import_spec path: (interpreted_string_literal) @path)
`

const goContainsQuery = `
(function_declaration name: (identifier) @name) @decl
(method_declaration name: (field_identifier) @name) @decl
(type_declaration (type_spec name: (type_identifier) @name)) @decl
(const_declaration (const_spec name: (identifier) @name)) @decl
(var_declaration (var_spec name: (identifier) @name)) @decl
`

const goCallQuery = `
(function_declaration name: (identifier) @fn_name body: (block) @body) @fn_decl
(method_declaration name: (field_identifier) @fn_name body: (block) @body) @fn_decl
`

const goCallExprQuery = `
(call_expression function: (identifier) @callee)
(call_expression function: (selector_expression field: (field_identifier) @callee))
`

type funcRange struct {
	name      string
	startByte uint32
	endByte   uint32
}

type GoGraphExtractor struct {
	lang            *gotreesitter.Language
	importQuery     *gotreesitter.Query
	containsQuery   *gotreesitter.Query
	callFuncQuery   *gotreesitter.Query
	callExprQuery   *gotreesitter.Query
}

func NewGoGraphExtractor() (*GoGraphExtractor, error) {
	lang := grammars.GoLanguage()

	iq, err := gotreesitter.NewQuery(goImportQuery, lang)
	if err != nil {
		return nil, fmt.Errorf("import query: %w", err)
	}
	cq, err := gotreesitter.NewQuery(goContainsQuery, lang)
	if err != nil {
		return nil, fmt.Errorf("contains query: %w", err)
	}
	fq, err := gotreesitter.NewQuery(goCallQuery, lang)
	if err != nil {
		return nil, fmt.Errorf("call func query: %w", err)
	}
	eq, err := gotreesitter.NewQuery(goCallExprQuery, lang)
	if err != nil {
		return nil, fmt.Errorf("call expr query: %w", err)
	}
	return &GoGraphExtractor{
		lang:          lang,
		importQuery:   iq,
		containsQuery: cq,
		callFuncQuery: fq,
		callExprQuery: eq,
	}, nil
}

func (e *GoGraphExtractor) Supports(ext string) bool {
	return ext == ".go"
}

func (e *GoGraphExtractor) ExtractEdges(filePath string, content []byte) ([]Edge, error) {
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

func (e *GoGraphExtractor) extractContains(bt *gotreesitter.BoundTree, tree *gotreesitter.Tree, content []byte, filePath string) []Edge {
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
			Language:   "go",
		})
	}
	return edges
}

func (e *GoGraphExtractor) extractImports(bt *gotreesitter.BoundTree, tree *gotreesitter.Tree, content []byte, filePath string) []Edge {
	matches := e.importQuery.Execute(tree)
	var edges []Edge
	seen := map[string]bool{}

	for _, match := range matches {
		for _, cap := range match.Captures {
			if cap.Name != "path" {
				continue
			}
			raw := bt.NodeText(cap.Node)
			importPath := strings.Trim(raw, `"`)
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
				Language:   "go",
			})
		}
	}
	return edges
}

func (e *GoGraphExtractor) extractCalls(bt *gotreesitter.BoundTree, tree *gotreesitter.Tree, content []byte, filePath string) []Edge {
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
			e := Edge{
				SourceNode: filePath + "::" + enclosing,
				TargetNode: callee,
				Kind:       EdgeCalls,
				SourceFile: filePath,
				Line:       lineForByte(content, cap.Node.StartByte()),
				Language:   "go",
			}
			if isInsideConditional(bt, cap.Node) {
				e.Metadata = map[string]any{"conditional": true}
			}
			edges = append(edges, e)
		}
	}
	return edges
}

func isInsideConditional(bt *gotreesitter.BoundTree, n *gotreesitter.Node) bool {
	for p := n.Parent(); p != nil; p = p.Parent() {
		typ := bt.NodeType(p)
		if typ == "function_declaration" || typ == "method_declaration" || typ == "func_literal" {
			return false
		}
		if typ == "if_statement" || typ == "expression_switch_statement" || typ == "type_switch_statement" || typ == "select_statement" {
			return true
		}
	}
	return false
}

func enclosingFunc(funcs []funcRange, byteOffset uint32) string {
	for _, f := range funcs {
		if byteOffset >= f.startByte && byteOffset <= f.endByte {
			return f.name
		}
	}
	return ""
}

func lineForByte(src []byte, byteOffset uint32) int {
	if int(byteOffset) > len(src) {
		byteOffset = uint32(len(src))
	}
	return bytes.Count(src[:byteOffset], []byte("\n")) + 1
}
