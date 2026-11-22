package symbol

import (
	"fmt"
	"path/filepath"
	"strings"

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
		declType := bt.NodeType(declNode)
		allowHOF := declType == "lexical_declaration" || declType == "variable_declaration"
		if valNode != nil && !isFunctionInitializer(bt, lang, valNode, allowHOF) {
			continue
		}

		name := bt.NodeText(nameNode)
		kind := jsTSNodeKind(bt, declNode)
		key := string(kind) + ":" + name
		rank := symbolDeclRank(bt, declNode)

		sigNode := declNode
		endByte := declNode.EndByte()
		if valNode != nil && allowHOF && hasMultipleVariableDeclarators(bt, declNode) {
			if declarator := valNode.Parent(); declarator != nil {
				sigNode = declarator
				endByte = declarator.EndByte()
			}
		}

		sym := Symbol{
			Name:      name,
			Kind:      kind,
			File:      filePath,
			Line:      lineForByte(content, nameNode.StartByte()),
			EndLine:   lineForByte(content, endByte),
			Signature: jsTSSymbolSignature(bt, content, sigNode, nameNode),
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

func hasMultipleVariableDeclarators(bt *gotreesitter.BoundTree, declNode *gotreesitter.Node) bool {
	count := 0
	for i := 0; i < int(declNode.NamedChildCount()); i++ {
		if c := declNode.NamedChild(i); c != nil && bt.NodeType(c) == "variable_declarator" {
			count++
			if count > 1 {
				return true
			}
		}
	}
	return false
}

func jsTSSymbolSignature(bt *gotreesitter.BoundTree, content []byte, sigNode, nameNode *gotreesitter.Node) string {
	sigStart := sigNode.StartByte()
	sigEnd := sigNode.EndByte()
	nameStart := nameNode.StartByte()
	if lineForByte(content, sigStart) < lineForByte(content, nameStart) &&
		int(sigEnd) <= len(content) && sigStart <= nameStart && nameStart <= sigEnd {
		lineStart := sigStart
		for i := int(nameStart); i > int(sigStart); i-- {
			if content[i-1] == '\n' {
				lineStart = uint32(i)
				break
			}
		}
		lineEnd := sigEnd
		for i := int(nameStart); i < int(sigEnd); i++ {
			if content[i] == '\n' {
				lineEnd = uint32(i)
				break
			}
		}
		return strings.TrimSpace(string(content[lineStart:lineEnd]))
	}
	return firstLine(bt.NodeText(sigNode))
}

func symbolDeclRank(bt *gotreesitter.BoundTree, declNode *gotreesitter.Node) int {
	switch bt.NodeType(declNode) {
	case "function_declaration", "generator_function_declaration",
		"lexical_declaration", "variable_declaration",
		"class_declaration", "interface_declaration", "type_alias_declaration", "enum_declaration":
		if atModule, exported := moduleScopeInfo(bt, declNode); atModule {
			if exported {
				return 5
			}
			return 4
		}
		return 2
	case "method_definition", "public_field_definition", "field_definition":
		if parent := declNode.Parent(); parent != nil && bt.NodeType(parent) == "class_body" {
			if classNode := parent.Parent(); classNode != nil {
				if atModule, exported := classScopeInfo(bt, classNode); atModule {
					if exported {
						return 5
					}
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

func moduleScopeInfo(bt *gotreesitter.BoundTree, node *gotreesitter.Node) (atModuleScope bool, exported bool) {
	allExported := true
	cur := node
	for cur != nil {
		p := cur.Parent()
		if p == nil {
			return false, false
		}
		if bt.NodeType(p) == "export_statement" {
			cur = p
			p = cur.Parent()
			if p == nil {
				return false, false
			}
		} else {
			allExported = false
		}

		switch bt.NodeType(p) {
		case "program":
			return true, allExported
		case "statement_block":
			ns := p.Parent()
			for ns != nil && (bt.NodeType(ns) == "internal_module" || bt.NodeType(ns) == "module") {
				cur = ns
				if next := ns.Parent(); next != nil && (bt.NodeType(next) == "internal_module" || bt.NodeType(next) == "module") {
					ns = next
					continue
				}
				break
			}
			if cur != ns || ns == nil {
				return false, false
			}
		default:
			return false, false
		}
	}
	return false, false
}

func classScopeInfo(bt *gotreesitter.BoundTree, classNode *gotreesitter.Node) (atModuleScope bool, exported bool) {
	ct := bt.NodeType(classNode)
	if ct != "class_declaration" && ct != "abstract_class_declaration" && ct != "class" {
		return false, false
	}
	if atModule, exp := moduleScopeInfo(bt, classNode); atModule {
		return true, exp
	}
	if ct != "class" {
		return false, false
	}
	cur := classNode
	for steps := 0; steps < 16 && cur != nil; steps++ {
		p := cur.Parent()
		if p == nil {
			return false, false
		}
		switch bt.NodeType(p) {
		case "parenthesized_expression", "satisfies_expression", "as_expression", "type_assertion", "non_null_expression":
			cur = p
		case "variable_declarator":
			if decl := p.Parent(); decl != nil {
				dt := bt.NodeType(decl)
				if dt == "lexical_declaration" || dt == "variable_declaration" {
					return moduleScopeInfo(bt, decl)
				}
			}
			return false, false
		case "assignment_expression":
			if stmt := p.Parent(); stmt != nil && bt.NodeType(stmt) == "expression_statement" {
				atMod, _ := moduleScopeInfo(bt, stmt)
				return atMod, false
			}
			return false, false
		default:
			return false, false
		}
	}
	return false, false
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

const (
	maxHOFWrapperDepth    = 4
	maxHOFCalleeChainStep = 8
)

func isFunctionInitializer(bt *gotreesitter.BoundTree, lang *gotreesitter.Language, valNode *gotreesitter.Node, allowHOF bool) bool {
	n := unwrapJSExpr(bt, valNode)
	if n == nil {
		return false
	}
	nt := bt.NodeType(n)
	if isDirectJSFunctionNode(nt) {
		return true
	}
	if allowHOF && nt == "call_expression" {
		return isHOFWrappedFunction(bt, lang, n, 0)
	}
	return false
}

func unwrapJSExpr(bt *gotreesitter.BoundTree, n *gotreesitter.Node) *gotreesitter.Node {
	for steps := 0; steps < 16 && n != nil; steps++ {
		switch bt.NodeType(n) {
		case "parenthesized_expression", "satisfies_expression", "as_expression", "non_null_expression", "instantiation_expression":
			next := firstNonCommentNamedChild(bt, n)
			if next == nil {
				return n
			}
			n = next
		case "type_assertion":
			next := lastNonCommentNamedChild(bt, n)
			if next == nil {
				return n
			}
			n = next
		default:
			return n
		}
	}
	return n
}

func isJSCommentNode(nodeType string) bool {
	return nodeType == "comment" || nodeType == "html_comment"
}

func firstNonCommentNamedChild(bt *gotreesitter.BoundTree, n *gotreesitter.Node) *gotreesitter.Node {
	for i := 0; i < int(n.NamedChildCount()); i++ {
		if c := n.NamedChild(i); c != nil && !isJSCommentNode(bt.NodeType(c)) {
			return c
		}
	}
	return nil
}

func lastNonCommentNamedChild(bt *gotreesitter.BoundTree, n *gotreesitter.Node) *gotreesitter.Node {
	for i := int(n.NamedChildCount()) - 1; i >= 0; i-- {
		if c := n.NamedChild(i); c != nil && !isJSCommentNode(bt.NodeType(c)) {
			return c
		}
	}
	return nil
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
	cur := unwrapJSExpr(bt, fnNode)
	for steps := 0; steps < maxHOFCalleeChainStep && cur != nil; steps++ {
		switch bt.NodeType(cur) {
		case "identifier":
			return !isNonHOFCalleeName(bt.NodeText(cur))
		case "member_expression":
			prop := cur.ChildByFieldName("property", lang)
			if prop == nil || bt.NodeType(prop) != "property_identifier" || isNonHOFCalleeName(bt.NodeText(prop)) {
				return false
			}
			cur = unwrapJSExpr(bt, cur.ChildByFieldName("object", lang))
		case "call_expression":
			cur = unwrapJSExpr(bt, cur.ChildByFieldName("function", lang))
		default:
			return false
		}
	}
	return false
}

func isNonHOFCalleeName(name string) bool {
	switch name {
	case "setTimeout", "setInterval", "setImmediate", "requestAnimationFrame", "requestIdleCallback", "queueMicrotask",
		"map", "flatMap", "filter", "reduce", "reduceRight",
		"find", "findIndex", "findLast", "findLastIndex",
		"some", "every", "forEach", "sort", "toSorted",
		"from", "of",
		"groupBy", "keyBy", "countBy", "sortBy", "orderBy", "uniqBy", "partition",
		"replace", "replaceAll", "split",
		"then", "catch", "finally",
		"parse", "stringify",
		"transaction", "$transaction",
		"addEventListener", "removeEventListener", "subscribe",
		"on", "once", "off", "addListener", "removeListener", "listen",
		"describe", "it", "test":
		return true
	default:
		return false
	}
}
