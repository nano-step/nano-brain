package graph

import (
	"fmt"
	"path/filepath"
	"strings"

	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

const vueInjectionQuery = `
(script_element
  (raw_text) @injection.content
)
(#set! injection.language "typescript")
`

var _ Extractor = (*VueSFCExtractor)(nil)

type VueSFCExtractor struct {
	ip            *gotreesitter.InjectionParser
	tsImportQ     *gotreesitter.Query
	tsContainsQ   *gotreesitter.Query
	tsCallFuncQ   *gotreesitter.Query
	tsCallExprQ   *gotreesitter.Query
	jsImportQ     *gotreesitter.Query
	jsContainsQ   *gotreesitter.Query
	jsCallFuncQ   *gotreesitter.Query
	jsCallExprQ   *gotreesitter.Query
	tsLang        *gotreesitter.Language
	jsLang        *gotreesitter.Language
}

func NewVueSFCExtractor() (*VueSFCExtractor, error) {
	vueLang := grammars.VueLanguage()
	tsLang := grammars.TypescriptLanguage()
	jsLang := grammars.JavascriptLanguage()

	ip := gotreesitter.NewInjectionParser()
	ip.RegisterLanguage("vue", vueLang)
	ip.RegisterLanguage("typescript", tsLang)
	ip.RegisterLanguage("javascript", jsLang)
	if err := ip.RegisterInjectionQuery("vue", vueInjectionQuery); err != nil {
		return nil, fmt.Errorf("vue injection query: %w", err)
	}

	tsImportQ, err := gotreesitter.NewQuery(tsImportQuery, tsLang)
	if err != nil {
		return nil, fmt.Errorf("ts import query: %w", err)
	}
	tsContainsQ, err := gotreesitter.NewQuery(tsContainsQuery, tsLang)
	if err != nil {
		return nil, fmt.Errorf("ts contains query: %w", err)
	}
	tsCallFuncQ, err := gotreesitter.NewQuery(tsCallFuncQuery, tsLang)
	if err != nil {
		return nil, fmt.Errorf("ts call func query: %w", err)
	}
	tsCallExprQ, err := gotreesitter.NewQuery(tsCallExprQuery, tsLang)
	if err != nil {
		return nil, fmt.Errorf("ts call expr query: %w", err)
	}

	jsImportQ, err := gotreesitter.NewQuery(jsGraphImportQuery, jsLang)
	if err != nil {
		return nil, fmt.Errorf("js import query: %w", err)
	}
	jsContainsQ, err := gotreesitter.NewQuery(jsGraphContainsQuery, jsLang)
	if err != nil {
		return nil, fmt.Errorf("js contains query: %w", err)
	}
	jsCallFuncQ, err := gotreesitter.NewQuery(jsGraphCallFuncQuery, jsLang)
	if err != nil {
		return nil, fmt.Errorf("js call func query: %w", err)
	}
	jsCallExprQ, err := gotreesitter.NewQuery(jsGraphCallExprQuery, jsLang)
	if err != nil {
		return nil, fmt.Errorf("js call expr query: %w", err)
	}

	return &VueSFCExtractor{
		ip:          ip,
		tsImportQ:   tsImportQ,
		tsContainsQ: tsContainsQ,
		tsCallFuncQ: tsCallFuncQ,
		tsCallExprQ: tsCallExprQ,
		jsImportQ:   jsImportQ,
		jsContainsQ: jsContainsQ,
		jsCallFuncQ: jsCallFuncQ,
		jsCallExprQ: jsCallExprQ,
		tsLang:      tsLang,
		jsLang:      jsLang,
	}, nil
}

func (e *VueSFCExtractor) Supports(ext string) bool {
	return ext == ".vue"
}

func (e *VueSFCExtractor) ExtractEdges(filePath string, content []byte) ([]Edge, error) {
	result, err := e.ip.Parse(content, "vue")
	if err != nil {
		return nil, fmt.Errorf("parse vue sfc %s: %w", filePath, err)
	}
	defer func() {
		if result != nil {
			result.Tree.Release()
		}
	}()

	relFile := filepath.ToSlash(filePath)

	var edges []Edge
	for _, inj := range result.Injections {
		if inj.Tree == nil || inj.Language == "" {
			continue
		}
		lang, importQ, containsQ, callFuncQ, callExprQ := e.resolveLang(inj.Language)
		if lang == nil {
			continue
		}

		bt := gotreesitter.Bind(inj.Tree)
		tree := inj.Tree

		edges = append(edges, e.extractContains(bt, tree, content, relFile, containsQ)...)
		edges = append(edges, e.extractImports(bt, tree, content, relFile, lang, importQ)...)
		edges = append(edges, e.extractCalls(bt, tree, content, relFile, callFuncQ, callExprQ)...)

		bt.Release()
	}

	return edges, nil
}

func (e *VueSFCExtractor) resolveLang(langName string) (*gotreesitter.Language, *gotreesitter.Query, *gotreesitter.Query, *gotreesitter.Query, *gotreesitter.Query) {
	switch langName {
	case "typescript":
		return e.tsLang, e.tsImportQ, e.tsContainsQ, e.tsCallFuncQ, e.tsCallExprQ
	case "javascript":
		return e.jsLang, e.jsImportQ, e.jsContainsQ, e.jsCallFuncQ, e.jsCallExprQ
	default:
		return nil, nil, nil, nil, nil
	}
}

func (e *VueSFCExtractor) extractContains(bt *gotreesitter.BoundTree, tree *gotreesitter.Tree, content []byte, filePath string, q *gotreesitter.Query) []Edge {
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
			Language:   "vue",
		})
	}
	return edges
}

func (e *VueSFCExtractor) extractImports(bt *gotreesitter.BoundTree, tree *gotreesitter.Tree, content []byte, filePath string, lang *gotreesitter.Language, q *gotreesitter.Query) []Edge {
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
			edge := Edge{
				SourceNode: filePath,
				TargetNode: importPath,
				Kind:       EdgeImports,
				SourceFile: filePath,
				Line:       lineForByte(content, cap.Node.StartByte()),
				Language:   "vue",
			}
			if strings.HasSuffix(importPath, ".vue") {
				edge.Metadata = map[string]any{"component": true}
			}
			edges = append(edges, edge)
		}
	}

	edges = append(edges, e.extractRequireImports(bt, tree, content, filePath, lang, seen)...)

	return edges
}

func (e *VueSFCExtractor) extractRequireImports(bt *gotreesitter.BoundTree, tree *gotreesitter.Tree, content []byte, filePath string, lang *gotreesitter.Language, seen map[string]bool) []Edge {
	root := tree.RootNode()
	var edges []Edge
	e.walkRequire(bt, root, content, filePath, lang, seen, &edges)
	return edges
}

func (e *VueSFCExtractor) walkRequire(bt *gotreesitter.BoundTree, node *gotreesitter.Node, content []byte, filePath string, lang *gotreesitter.Language, seen map[string]bool, edges *[]Edge) {
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
						edge := Edge{
							SourceNode: filePath,
							TargetNode: importPath,
							Kind:       EdgeImports,
							SourceFile: filePath,
							Line:       lineForByte(content, fnNode.StartByte()),
							Language:   "vue",
						}
						if strings.HasSuffix(importPath, ".vue") {
							edge.Metadata = map[string]any{"component": true}
						}
						*edges = append(*edges, edge)
					}
				}
			}
		}
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		e.walkRequire(bt, node.Child(i), content, filePath, lang, seen, edges)
	}
}

func (e *VueSFCExtractor) extractCalls(bt *gotreesitter.BoundTree, tree *gotreesitter.Tree, content []byte, filePath string, funcQ, exprQ *gotreesitter.Query) []Edge {
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
				Language:   "vue",
			})
		}
	}
	return edges
}
