package graph

import (
	"fmt"
	"path/filepath"
	"strings"

	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

const tsImportQuery = `
(import_statement source: (string) @path)
`

const tsContainsQuery = `
(function_declaration name: (identifier) @name) @decl
(class_declaration name: (type_identifier) @name) @decl
(interface_declaration name: (type_identifier) @name) @decl
(type_alias_declaration name: (type_identifier) @name) @decl
(enum_declaration name: (identifier) @name) @decl
(lexical_declaration (variable_declarator name: (identifier) @name)) @decl
`

const tsCallFuncQuery = `
(function_declaration name: (identifier) @fn_name body: (statement_block) @body) @fn_decl
(method_definition name: (property_identifier) @fn_name body: (statement_block) @body) @fn_decl
(lexical_declaration (variable_declarator name: (identifier) @fn_name value: (arrow_function body: (statement_block) @body))) @fn_decl
`

const tsCallExprQuery = `
(call_expression function: (identifier) @callee)
(call_expression function: (member_expression property: (property_identifier) @callee))
`

type TypeScriptGraphExtractor struct {
	lang          *gotreesitter.Language
	tsxLang       *gotreesitter.Language
	importQuery   *gotreesitter.Query
	tsxImportQ    *gotreesitter.Query
	containsQuery *gotreesitter.Query
	tsxContainsQ  *gotreesitter.Query
	callFuncQuery *gotreesitter.Query
	tsxCallFuncQ  *gotreesitter.Query
	callExprQuery *gotreesitter.Query
	tsxCallExprQ  *gotreesitter.Query
}

func NewTypeScriptGraphExtractor() (*TypeScriptGraphExtractor, error) {
	lang := grammars.TypescriptLanguage()
	tsxLang := grammars.TsxLanguage()

	iq, err := gotreesitter.NewQuery(tsImportQuery, lang)
	if err != nil {
		return nil, fmt.Errorf("ts import query: %w", err)
	}
	tsxIQ, err := gotreesitter.NewQuery(tsImportQuery, tsxLang)
	if err != nil {
		return nil, fmt.Errorf("tsx import query: %w", err)
	}
	cq, err := gotreesitter.NewQuery(tsContainsQuery, lang)
	if err != nil {
		return nil, fmt.Errorf("ts contains query: %w", err)
	}
	tsxCQ, err := gotreesitter.NewQuery(tsContainsQuery, tsxLang)
	if err != nil {
		return nil, fmt.Errorf("tsx contains query: %w", err)
	}
	fq, err := gotreesitter.NewQuery(tsCallFuncQuery, lang)
	if err != nil {
		return nil, fmt.Errorf("ts call func query: %w", err)
	}
	tsxFQ, err := gotreesitter.NewQuery(tsCallFuncQuery, tsxLang)
	if err != nil {
		return nil, fmt.Errorf("tsx call func query: %w", err)
	}
	eq, err := gotreesitter.NewQuery(tsCallExprQuery, lang)
	if err != nil {
		return nil, fmt.Errorf("ts call expr query: %w", err)
	}
	tsxEQ, err := gotreesitter.NewQuery(tsCallExprQuery, tsxLang)
	if err != nil {
		return nil, fmt.Errorf("tsx call expr query: %w", err)
	}

	return &TypeScriptGraphExtractor{
		lang:          lang,
		tsxLang:       tsxLang,
		importQuery:   iq,
		tsxImportQ:    tsxIQ,
		containsQuery: cq,
		tsxContainsQ:  tsxCQ,
		callFuncQuery: fq,
		tsxCallFuncQ:  tsxFQ,
		callExprQuery: eq,
		tsxCallExprQ:  tsxEQ,
	}, nil
}

func (e *TypeScriptGraphExtractor) Supports(ext string) bool {
	return ext == ".ts" || ext == ".tsx"
}

var _ ImportResolvingExtractor = (*TypeScriptGraphExtractor)(nil)

func (e *TypeScriptGraphExtractor) ExtractEdges(filePath string, content []byte) ([]Edge, error) {
	lang, importQ, containsQ, callFuncQ, callExprQ := e.lang, e.importQuery, e.containsQuery, e.callFuncQuery, e.callExprQuery
	if filepath.Ext(filePath) == ".tsx" {
		lang, importQ, containsQ, callFuncQ, callExprQ = e.tsxLang, e.tsxImportQ, e.tsxContainsQ, e.tsxCallFuncQ, e.tsxCallExprQ
	}

	parser := gotreesitter.NewParser(lang)
	tree, err := parser.Parse(content)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", filePath, err)
	}
	bt := gotreesitter.Bind(tree)
	defer bt.Release()

	relFile := filepath.ToSlash(filePath)

	var edges []Edge
	edges = append(edges, e.extractContains(bt, tree, content, relFile, containsQ)...)
	edges = append(edges, e.extractImports(bt, tree, content, relFile, lang, importQ)...)
	edges = append(edges, e.extractCalls(bt, tree, content, relFile, callFuncQ, callExprQ)...)
	return edges, nil
}

// ExtractEdgesWithImportContext is identical to ExtractEdges except imports
// edges are resolved via ic (see ImportResolvingExtractor). ExtractEdges
// itself is untouched so existing callers/tests keep seeing raw specifiers.
func (e *TypeScriptGraphExtractor) ExtractEdgesWithImportContext(filePath string, content []byte, ic ImportContext) ([]Edge, error) {
	lang, importQ, containsQ, callFuncQ, callExprQ := e.lang, e.importQuery, e.containsQuery, e.callFuncQuery, e.callExprQuery
	if filepath.Ext(filePath) == ".tsx" {
		lang, importQ, containsQ, callFuncQ, callExprQ = e.tsxLang, e.tsxImportQ, e.tsxContainsQ, e.tsxCallFuncQ, e.tsxCallExprQ
	}

	parser := gotreesitter.NewParser(lang)
	tree, err := parser.Parse(content)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", filePath, err)
	}
	bt := gotreesitter.Bind(tree)
	defer bt.Release()

	relFile := filepath.ToSlash(filePath)

	var edges []Edge
	edges = append(edges, e.extractContains(bt, tree, content, relFile, containsQ)...)
	edges = append(edges, e.extractImportsResolved(bt, tree, content, relFile, lang, importQ, ic)...)
	edges = append(edges, e.extractCalls(bt, tree, content, relFile, callFuncQ, callExprQ)...)
	return edges, nil
}

func (e *TypeScriptGraphExtractor) extractContains(bt *gotreesitter.BoundTree, tree *gotreesitter.Tree, content []byte, filePath string, q *gotreesitter.Query) []Edge {
	matches := q.Execute(tree)
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
			Language:   "typescript",
		})
	}
	return edges
}

func (e *TypeScriptGraphExtractor) extractImports(bt *gotreesitter.BoundTree, tree *gotreesitter.Tree, content []byte, filePath string, lang *gotreesitter.Language, q *gotreesitter.Query) []Edge {
	matches := q.Execute(tree)
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
				Language:   "typescript",
			})
		}
	}

	edges = append(edges, e.extractRequireImports(bt, tree, content, filePath, lang, seen)...)

	return edges
}

func (e *TypeScriptGraphExtractor) extractRequireImports(bt *gotreesitter.BoundTree, tree *gotreesitter.Tree, content []byte, filePath string, lang *gotreesitter.Language, seen map[string]bool) []Edge {
	root := tree.RootNode()
	var edges []Edge
	e.walkRequire(bt, root, content, filePath, lang, seen, &edges)
	return edges
}

func (e *TypeScriptGraphExtractor) walkRequire(bt *gotreesitter.BoundTree, node *gotreesitter.Node, content []byte, filePath string, lang *gotreesitter.Language, seen map[string]bool, edges *[]Edge) {
	if node == nil {
		return
	}
	if node.Type(lang) == "call_expression" {
		fnNode := node.ChildByFieldName("function", lang)
		if fnNode != nil && fnNode.Type(lang) == "identifier" && bt.NodeText(fnNode) == "require" {
			argsNode := node.ChildByFieldName("arguments", lang)
			if argsNode != nil {
				argNode := firstChildOfType(argsNode, lang, "string")
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
							Language:   "typescript",
						})
					}
				}
			}
		}
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		e.walkRequire(bt, node.Child(i), content, filePath, lang, seen, edges)
	}
}

// extractImportsResolved mirrors extractImports but resolves each raw import
// specifier via ic (relative/aliased specifiers become workspace-relative
// paths; bare packages and unresolvable specifiers pass through unchanged).
func (e *TypeScriptGraphExtractor) extractImportsResolved(bt *gotreesitter.BoundTree, tree *gotreesitter.Tree, content []byte, filePath string, lang *gotreesitter.Language, q *gotreesitter.Query, ic ImportContext) []Edge {
	matches := q.Execute(tree)
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
				Language:   "typescript",
				Metadata:   meta,
			})
		}
	}

	edges = append(edges, e.extractRequireImportsResolved(bt, tree, content, filePath, lang, seen, ic)...)

	return edges
}

func (e *TypeScriptGraphExtractor) extractRequireImportsResolved(bt *gotreesitter.BoundTree, tree *gotreesitter.Tree, content []byte, filePath string, lang *gotreesitter.Language, seen map[string]bool, ic ImportContext) []Edge {
	root := tree.RootNode()
	var edges []Edge
	e.walkRequireResolved(bt, root, content, filePath, lang, seen, ic, &edges)
	return edges
}

func (e *TypeScriptGraphExtractor) walkRequireResolved(bt *gotreesitter.BoundTree, node *gotreesitter.Node, content []byte, filePath string, lang *gotreesitter.Language, seen map[string]bool, ic ImportContext, edges *[]Edge) {
	if node == nil {
		return
	}
	if node.Type(lang) == "call_expression" {
		fnNode := node.ChildByFieldName("function", lang)
		if fnNode != nil && fnNode.Type(lang) == "identifier" && bt.NodeText(fnNode) == "require" {
			argsNode := node.ChildByFieldName("arguments", lang)
			if argsNode != nil {
				argNode := firstChildOfType(argsNode, lang, "string")
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
							Language:   "typescript",
							Metadata:   meta,
						})
					}
				}
			}
		}
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		e.walkRequireResolved(bt, node.Child(i), content, filePath, lang, seen, ic, edges)
	}
}

func (e *TypeScriptGraphExtractor) extractCalls(bt *gotreesitter.BoundTree, tree *gotreesitter.Tree, content []byte, filePath string, funcQ, exprQ *gotreesitter.Query) []Edge {
	funcMatches := funcQ.Execute(tree)
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

	callMatches := exprQ.Execute(tree)
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
				Language:   "typescript",
			})
		}
	}
	return edges
}

func firstChildOfType(node *gotreesitter.Node, lang *gotreesitter.Language, nodeType string) *gotreesitter.Node {
	for i := 0; i < int(node.ChildCount()); i++ {
		c := node.Child(i)
		if c != nil && c.Type(lang) == nodeType {
			return c
		}
	}
	return nil
}

