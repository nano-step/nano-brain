package symbol

import (
	"fmt"

	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

const jsQuery = `
(function_declaration name: (identifier) @name) @decl
(method_definition name: (property_identifier) @name) @decl
(class_declaration name: (identifier) @name) @decl
(lexical_declaration (variable_declarator name: (identifier) @name value: (arrow_function))) @decl
`

type JavaScriptExtractor struct {
	lang  *gotreesitter.Language
	query *gotreesitter.Query
}

func NewJavaScriptExtractor() (*JavaScriptExtractor, error) {
	lang := grammars.JavascriptLanguage()
	q, err := gotreesitter.NewQuery(jsQuery, lang)
	if err != nil {
		return nil, fmt.Errorf("js query: %w", err)
	}
	return &JavaScriptExtractor{lang: lang, query: q}, nil
}

func (e *JavaScriptExtractor) Supports(ext string) bool {
	return ext == ".js"
}

func (e *JavaScriptExtractor) Extract(filePath string, content []byte) ([]Symbol, error) {
	parser := gotreesitter.NewParser(e.lang)
	tree, err := parser.Parse(content)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", filePath, err)
	}

	bt := gotreesitter.Bind(tree)
	defer bt.Release()

	matches := e.query.Execute(tree)
	var symbols []Symbol
	seen := map[string]bool{}

	for _, match := range matches {
		var declNode, nameNode *gotreesitter.Node
		for _, cap := range match.Captures {
			switch cap.Name {
			case "decl":
				declNode = cap.Node
			case "name":
				nameNode = cap.Node
			}
		}
		if nameNode == nil || declNode == nil {
			continue
		}
		name := bt.NodeText(nameNode)
		kind := jsNodeKind(bt, declNode)
		key := string(kind) + ":" + name
		if seen[key] {
			continue
		}
		seen[key] = true
		symbols = append(symbols, Symbol{
			Name:      name,
			Kind:      kind,
			File:      filePath,
			Line:      lineForByte(content, nameNode.StartByte()),
			Signature: firstLine(bt.NodeText(declNode)),
			Language:  "javascript",
		})
	}
	return symbols, nil
}

func jsNodeKind(bt *gotreesitter.BoundTree, node *gotreesitter.Node) Kind {
	switch bt.NodeType(node) {
	case "function_declaration":
		return KindFunction
	case "method_definition":
		return KindMethod
	case "class_declaration":
		return KindType
	case "lexical_declaration":
		return KindFunction
	}
	return KindFunction
}
