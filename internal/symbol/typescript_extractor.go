package symbol

import (
	"fmt"
	"path/filepath"

	"github.com/nano-brain/nano-brain/internal/tsparse"
	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

const tsQuery = `
(function_declaration name: (identifier) @name) @decl
(generator_function_declaration name: (identifier) @name) @decl
(method_definition name: (property_identifier) @name) @decl
(public_field_definition name: [(property_identifier) (private_property_identifier)] @name value: (_) @val) @decl
(interface_declaration name: (type_identifier) @name) @decl
(type_alias_declaration name: (type_identifier) @name) @decl
(enum_declaration name: (identifier) @name) @decl
(class_declaration name: (type_identifier) @name) @decl
(lexical_declaration (variable_declarator name: (identifier) @name value: (_) @val)) @decl
(variable_declaration (variable_declarator name: (identifier) @name value: (_) @val)) @decl
(pair key: (property_identifier) @name value: (_) @val) @decl
(assignment_expression left: (member_expression property: (property_identifier) @name) right: (_) @val) @decl
`

type TypeScriptExtractor struct {
	lang    *gotreesitter.Language
	tsxLang *gotreesitter.Language
	query   *gotreesitter.Query
	tsxQ    *gotreesitter.Query
}

func NewTypeScriptExtractor() (*TypeScriptExtractor, error) {
	lang := grammars.TypescriptLanguage()
	q, err := gotreesitter.NewQuery(tsQuery, lang)
	if err != nil {
		return nil, fmt.Errorf("ts query: %w", err)
	}
	tsxLang := grammars.TsxLanguage()
	tsxQ, err := gotreesitter.NewQuery(tsQuery, tsxLang)
	if err != nil {
		return nil, fmt.Errorf("tsx query: %w", err)
	}
	return &TypeScriptExtractor{lang: lang, tsxLang: tsxLang, query: q, tsxQ: tsxQ}, nil
}

func (e *TypeScriptExtractor) Supports(ext string) bool {
	switch ext {
	case ".ts", ".tsx", ".mts", ".cts":
		return true
	default:
		return false
	}
}

func (e *TypeScriptExtractor) Extract(filePath string, content []byte) ([]Symbol, error) {
	lang, q := e.lang, e.query
	if filepath.Ext(filePath) == ".tsx" {
		lang, q = e.tsxLang, e.tsxQ
	}

	parser := tsparse.NewParser(lang)
	tree, err := parser.Parse(content)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", filePath, err)
	}

	bt := gotreesitter.Bind(tree)
	defer bt.Release()

	matches := q.Execute(tree)
	var symbols []Symbol
	seen := make(map[string]rankedSymbolIndex, len(matches))
	appendJSTSSymbols(&symbols, seen, bt, lang, matches, content, filePath, "typescript")
	return symbols, nil
}

type rankedSymbolIndex struct {
	index int
	rank  int
}

func appendJSTSSymbols(
	symbols *[]Symbol,
	seen map[string]rankedSymbolIndex,
	bt *gotreesitter.BoundTree,
	lang *gotreesitter.Language,
	matches []gotreesitter.QueryMatch,
	content []byte,
	filePath string,
	language string,
) {
	for _, match := range matches {
		var declNode, nameNode, valNode *gotreesitter.Node
		for _, cap := range match.Captures {
			switch cap.Name {
			case "decl":
				declNode = cap.Node
			case "name":
				nameNode = cap.Node
			case "val":
				valNode = cap.Node
			}
		}
		if nameNode == nil || declNode == nil {
			continue
		}
		if valNode != nil && !isFunctionInitializer(bt, lang, valNode) {
			continue
		}

		name := bt.NodeText(nameNode)
		kind := jsTSNodeKind(bt, declNode)
		key := string(kind) + ":" + name
		rank := symbolDeclRank(bt, declNode)

		endByte := declNode.EndByte()
		declType := bt.NodeType(declNode)
		if valNode != nil && (declType == "lexical_declaration" || declType == "variable_declaration") && declNode.NamedChildCount() > 1 {
			if declarator := valNode.Parent(); declarator != nil {
				endByte = declarator.EndByte()
			}
		}

		sym := Symbol{
			Name:      name,
			Kind:      kind,
			File:      filePath,
			Line:      lineForByte(content, nameNode.StartByte()),
			EndLine:   lineForByte(content, endByte),
			Signature: firstLine(bt.NodeText(declNode)),
			Language:  language,
		}

		if prev, ok := seen[key]; ok {
			if rank > prev.rank {
				(*symbols)[prev.index] = sym
				seen[key] = rankedSymbolIndex{index: prev.index, rank: rank}
			}
			continue
		}
		seen[key] = rankedSymbolIndex{index: len(*symbols), rank: rank}
		*symbols = append(*symbols, sym)
	}
}

func jsTSNodeKind(bt *gotreesitter.BoundTree, node *gotreesitter.Node) Kind {
	switch bt.NodeType(node) {
	case "function_declaration", "generator_function_declaration":
		return KindFunction
	case "method_definition", "public_field_definition", "field_definition", "pair":
		return KindMethod
	case "interface_declaration":
		return KindInterface
	case "type_alias_declaration", "enum_declaration", "class_declaration":
		return KindType
	case "lexical_declaration", "variable_declaration", "assignment_expression":
		return KindFunction
	}
	return KindFunction
}

func symbolDeclRank(bt *gotreesitter.BoundTree, declNode *gotreesitter.Node) int {
	switch bt.NodeType(declNode) {
	case "function_declaration", "generator_function_declaration",
		"lexical_declaration", "variable_declaration",
		"class_declaration", "interface_declaration", "type_alias_declaration", "enum_declaration":
		if isModuleScopeNode(bt, declNode) {
			return 4
		}
		return 2
	case "method_definition", "public_field_definition", "field_definition":
		if parent := declNode.Parent(); parent != nil && bt.NodeType(parent) == "class_body" {
			if classNode := parent.Parent(); classNode != nil {
				ct := bt.NodeType(classNode)
				if (ct == "class_declaration" || ct == "abstract_class_declaration" || ct == "class") && isModuleScopeNode(bt, classNode) {
					return 4
				}
			}
		}
		if !hasEnclosingCallableOrClass(bt, declNode) {
			return 3
		}
		return 2
	case "assignment_expression", "pair":
		if !hasEnclosingCallableOrClass(bt, declNode) {
			return 3
		}
		return 1
	}
	return 2
}

func isModuleScopeNode(bt *gotreesitter.BoundTree, node *gotreesitter.Node) bool {
	for p := node.Parent(); p != nil; p = p.Parent() {
		switch bt.NodeType(p) {
		case "program":
			return true
		case "export_statement":
			continue
		default:
			return false
		}
	}
	return false
}

func hasEnclosingCallableOrClass(bt *gotreesitter.BoundTree, node *gotreesitter.Node) bool {
	for p := node.Parent(); p != nil; p = p.Parent() {
		switch bt.NodeType(p) {
		case "function_declaration", "generator_function_declaration",
			"function_expression", "function", "generator_function",
			"arrow_function", "method_definition", "class_body":
			return true
		}
	}
	return false
}

const maxHOFWrapperDepth = 4

func isFunctionInitializer(bt *gotreesitter.BoundTree, lang *gotreesitter.Language, valNode *gotreesitter.Node) bool {
	n := unwrapJSExpr(bt, valNode)
	if n == nil {
		return false
	}
	nt := bt.NodeType(n)
	if isDirectJSFunctionNode(nt) {
		return true
	}
	if nt == "call_expression" {
		return isHOFWrappedFunction(bt, lang, n, 0)
	}
	return false
}

func unwrapJSExpr(bt *gotreesitter.BoundTree, n *gotreesitter.Node) *gotreesitter.Node {
	for steps := 0; steps < 16 && n != nil; steps++ {
		switch bt.NodeType(n) {
		case "parenthesized_expression", "satisfies_expression", "as_expression", "non_null_expression", "instantiation_expression":
			if n.NamedChildCount() == 0 {
				return n
			}
			n = n.NamedChild(0)
		case "type_assertion":
			cnt := int(n.NamedChildCount())
			if cnt == 0 {
				return n
			}
			n = n.NamedChild(cnt - 1)
		default:
			return n
		}
	}
	return n
}

func isDirectJSFunctionNode(nodeType string) bool {
	switch nodeType {
	case "arrow_function", "function_expression", "function", "generator_function":
		return true
	default:
		return false
	}
}

func isHOFWrappedFunction(bt *gotreesitter.BoundTree, lang *gotreesitter.Language, callNode *gotreesitter.Node, depth int) bool {
	if depth >= maxHOFWrapperDepth || callNode == nil {
		return false
	}
	fnNode := unwrapJSExpr(bt, callNode.ChildByFieldName("function", lang))
	if !isHOFCallee(bt, lang, fnNode) {
		return false
	}
	argsNode := callNode.ChildByFieldName("arguments", lang)
	if argsNode == nil {
		return false
	}
	for i := 0; i < int(argsNode.NamedChildCount()); i++ {
		arg := unwrapJSExpr(bt, argsNode.NamedChild(i))
		if arg == nil {
			continue
		}
		nt := bt.NodeType(arg)
		if isDirectJSFunctionNode(nt) {
			return true
		}
		if nt == "call_expression" && isHOFWrappedFunction(bt, lang, arg, depth+1) {
			return true
		}
	}
	return false
}

func isHOFCallee(bt *gotreesitter.BoundTree, lang *gotreesitter.Language, fnNode *gotreesitter.Node) bool {
	if fnNode == nil {
		return false
	}
	switch bt.NodeType(fnNode) {
	case "identifier":
		switch bt.NodeText(fnNode) {
		case "setTimeout", "setInterval", "setImmediate", "requestAnimationFrame", "requestIdleCallback", "queueMicrotask":
			return false
		default:
			return true
		}
	case "member_expression":
		obj := fnNode.ChildByFieldName("object", lang)
		prop := fnNode.ChildByFieldName("property", lang)
		if obj == nil || prop == nil || bt.NodeType(obj) != "identifier" || bt.NodeType(prop) != "property_identifier" {
			return false
		}
		switch bt.NodeText(prop) {
		case "map", "flatMap", "filter", "reduce", "reduceRight",
			"find", "findIndex", "findLast", "findLastIndex",
			"some", "every", "forEach", "sort", "toSorted",
			"then", "catch", "finally",
			"from", "of",
			"replace", "replaceAll",
			"parse", "stringify",
			"addEventListener", "removeEventListener", "subscribe",
			"transaction", "$transaction":
			return false
		default:
			return true
		}
	default:
		return false
	}
}
