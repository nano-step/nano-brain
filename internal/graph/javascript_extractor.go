package graph

import (
	"fmt"
	"path/filepath"
	"strings"

	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

const jsGraphImportQuery = `
(import_statement source: (string) @path)
`

const jsGraphContainsQuery = `
(function_declaration name: (identifier) @name) @decl
(class_declaration name: (identifier) @name) @decl
(lexical_declaration (variable_declarator name: (identifier) @name)) @decl
`

const jsGraphCallFuncQuery = `
(function_declaration name: (identifier) @fn_name body: (statement_block) @body) @fn_decl
(method_definition name: (property_identifier) @fn_name body: (statement_block) @body) @fn_decl
(lexical_declaration (variable_declarator name: (identifier) @fn_name value: (arrow_function body: (statement_block) @body))) @fn_decl
`

const jsGraphCallExprQuery = `
(call_expression function: (identifier) @callee)
(call_expression function: (member_expression property: (property_identifier) @callee))
`

type JavaScriptGraphExtractor struct {
	lang          *gotreesitter.Language
	importQuery   *gotreesitter.Query
	containsQuery *gotreesitter.Query
	callFuncQuery *gotreesitter.Query
	callExprQuery *gotreesitter.Query
}

func NewJavaScriptGraphExtractor() (*JavaScriptGraphExtractor, error) {
	lang := grammars.JavascriptLanguage()

	iq, err := gotreesitter.NewQuery(jsGraphImportQuery, lang)
	if err != nil {
		return nil, fmt.Errorf("js import query: %w", err)
	}
	cq, err := gotreesitter.NewQuery(jsGraphContainsQuery, lang)
	if err != nil {
		return nil, fmt.Errorf("js contains query: %w", err)
	}
	fq, err := gotreesitter.NewQuery(jsGraphCallFuncQuery, lang)
	if err != nil {
		return nil, fmt.Errorf("js call func query: %w", err)
	}
	eq, err := gotreesitter.NewQuery(jsGraphCallExprQuery, lang)
	if err != nil {
		return nil, fmt.Errorf("js call expr query: %w", err)
	}
	return &JavaScriptGraphExtractor{
		lang:          lang,
		importQuery:   iq,
		containsQuery: cq,
		callFuncQuery: fq,
		callExprQuery: eq,
	}, nil
}

func (e *JavaScriptGraphExtractor) Supports(ext string) bool {
	return ext == ".js" || ext == ".jsx"
}

var _ ImportResolvingExtractor = (*JavaScriptGraphExtractor)(nil)

func (e *JavaScriptGraphExtractor) ExtractEdges(filePath string, content []byte) ([]Edge, error) {
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

// ExtractEdgesWithImportContext is identical to ExtractEdges except imports
// edges are resolved via ic (see ImportResolvingExtractor). ExtractEdges
// itself is untouched so existing callers/tests keep seeing raw specifiers.
func (e *JavaScriptGraphExtractor) ExtractEdgesWithImportContext(filePath string, content []byte, ic ImportContext) ([]Edge, error) {
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
	edges = append(edges, e.extractImportsResolved(bt, tree, content, relFile, ic)...)
	edges = append(edges, e.extractCalls(bt, tree, content, relFile)...)
	return edges, nil
}

func (e *JavaScriptGraphExtractor) extractContains(bt *gotreesitter.BoundTree, tree *gotreesitter.Tree, content []byte, filePath string) []Edge {
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
			Language:   "javascript",
		})
	}
	return edges
}

func (e *JavaScriptGraphExtractor) extractImports(bt *gotreesitter.BoundTree, tree *gotreesitter.Tree, content []byte, filePath string) []Edge {
	matches := e.importQuery.Execute(tree)
	var edges []Edge
	seen := map[string]bool{}

	for _, match := range matches {
		for _, cap := range match.Captures {
			if cap.Name != "path" {
				continue
			}
			raw := bt.NodeText(cap.Node)
			importPath := strings.Trim(raw, "\"'`")
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
				Language:   "javascript",
			})
		}
	}

	root := tree.RootNode()
	e.walkRequireJS(bt, root, content, filePath, seen, &edges)

	return edges
}

func (e *JavaScriptGraphExtractor) walkRequireJS(bt *gotreesitter.BoundTree, node *gotreesitter.Node, content []byte, filePath string, seen map[string]bool, edges *[]Edge) {
	if node == nil {
		return
	}
	if node.Type(e.lang) == "call_expression" {
		fnNode := node.ChildByFieldName("function", e.lang)
		if fnNode != nil && fnNode.Type(e.lang) == "identifier" && bt.NodeText(fnNode) == "require" {
			argsNode := node.ChildByFieldName("arguments", e.lang)
			if argsNode != nil {
				argNode := firstChildOfType(argsNode, e.lang, "string")
				if argNode != nil {
					raw := bt.NodeText(argNode)
					importPath := strings.Trim(raw, "\"'`")
					if importPath != "" && !seen[importPath] {
						seen[importPath] = true
						*edges = append(*edges, Edge{
							SourceNode: filePath,
							TargetNode: importPath,
							Kind:       EdgeImports,
							SourceFile: filePath,
							Line:       lineForByte(content, fnNode.StartByte()),
							Language:   "javascript",
						})
					}
				}
			}
		}
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		e.walkRequireJS(bt, node.Child(i), content, filePath, seen, edges)
	}
}

// extractImportsResolved mirrors extractImports but resolves each raw import
// specifier via ic (relative/aliased specifiers become workspace-relative
// paths; bare packages and unresolvable specifiers pass through unchanged).
func (e *JavaScriptGraphExtractor) extractImportsResolved(bt *gotreesitter.BoundTree, tree *gotreesitter.Tree, content []byte, filePath string, ic ImportContext) []Edge {
	matches := e.importQuery.Execute(tree)
	var edges []Edge
	seen := map[string]bool{}

	for _, match := range matches {
		for _, cap := range match.Captures {
			if cap.Name != "path" {
				continue
			}
			raw := bt.NodeText(cap.Node)
			importPath := strings.Trim(raw, "\"'`")
			if importPath == "" || seen[importPath] {
				continue
			}
			seen[importPath] = true
			target, meta := resolveImport(ic, importPath, filePath)
			edges = append(edges, Edge{
				SourceNode: filePath,
				TargetNode: target,
				Kind:       EdgeImports,
				SourceFile: filePath,
				Line:       lineForByte(content, cap.Node.StartByte()),
				Language:   "javascript",
				Metadata:   meta,
			})
		}
	}

	root := tree.RootNode()
	e.walkRequireJSResolved(bt, root, content, filePath, seen, ic, &edges)

	return edges
}

func (e *JavaScriptGraphExtractor) walkRequireJSResolved(bt *gotreesitter.BoundTree, node *gotreesitter.Node, content []byte, filePath string, seen map[string]bool, ic ImportContext, edges *[]Edge) {
	if node == nil {
		return
	}
	if node.Type(e.lang) == "call_expression" {
		fnNode := node.ChildByFieldName("function", e.lang)
		if fnNode != nil && fnNode.Type(e.lang) == "identifier" && bt.NodeText(fnNode) == "require" {
			argsNode := node.ChildByFieldName("arguments", e.lang)
			if argsNode != nil {
				argNode := firstChildOfType(argsNode, e.lang, "string")
				if argNode != nil {
					raw := bt.NodeText(argNode)
					importPath := strings.Trim(raw, "\"'`")
					if importPath != "" && !seen[importPath] {
						seen[importPath] = true
						target, meta := resolveImport(ic, importPath, filePath)
						*edges = append(*edges, Edge{
							SourceNode: filePath,
							TargetNode: target,
							Kind:       EdgeImports,
							SourceFile: filePath,
							Line:       lineForByte(content, fnNode.StartByte()),
							Language:   "javascript",
							Metadata:   meta,
						})
					}
				}
			}
		}
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		e.walkRequireJSResolved(bt, node.Child(i), content, filePath, seen, ic, edges)
	}
}

func (e *JavaScriptGraphExtractor) extractCalls(bt *gotreesitter.BoundTree, tree *gotreesitter.Tree, content []byte, filePath string) []Edge {
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
				Language:   "javascript",
			})
		}
	}
	return edges
}
