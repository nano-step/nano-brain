package symbol

import (
	"fmt"
	"path/filepath"

	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

const tsQuery = `
(function_declaration name: (identifier) @name) @decl
(method_definition name: (property_identifier) @name) @decl
(interface_declaration name: (type_identifier) @name) @decl
(type_alias_declaration name: (type_identifier) @name) @decl
(class_declaration name: (type_identifier) @name) @decl
(lexical_declaration (variable_declarator name: (identifier) @name value: (arrow_function))) @decl
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
	return ext == ".ts" || ext == ".tsx"
}

func (e *TypeScriptExtractor) Extract(filePath string, content []byte) ([]Symbol, error) {
	lang, q := e.lang, e.query
	if filepath.Ext(filePath) == ".tsx" {
		lang, q = e.tsxLang, e.tsxQ
	}

	parser := gotreesitter.NewParser(lang)
	tree, err := parser.Parse(content)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", filePath, err)
	}

	bt := gotreesitter.Bind(tree)
	defer bt.Release()

	matches := q.Execute(tree)
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
		kind := tsNodeKind(bt, declNode)
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
			Language:  "typescript",
		})
	}
	return symbols, nil
}

func tsNodeKind(bt *gotreesitter.BoundTree, node *gotreesitter.Node) Kind {
	switch bt.NodeType(node) {
	case "function_declaration":
		return KindFunction
	case "method_definition":
		return KindMethod
	case "interface_declaration":
		return KindInterface
	case "type_alias_declaration":
		return KindType
	case "class_declaration":
		return KindType
	case "lexical_declaration":
		return KindFunction
	}
	return KindFunction
}
