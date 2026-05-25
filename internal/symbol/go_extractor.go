package symbol

import (
	"fmt"

	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

const goQuery = `
(function_declaration name: (identifier) @name) @decl
(method_declaration name: (field_identifier) @name) @decl
(type_declaration (type_spec name: (type_identifier) @name)) @decl
(const_declaration (const_spec name: (identifier) @name)) @decl
(var_declaration (var_spec name: (identifier) @name)) @decl
`

type GoExtractor struct {
	lang  *gotreesitter.Language
	query *gotreesitter.Query
}

func NewGoExtractor() (*GoExtractor, error) {
	lang := grammars.GoLanguage()
	q, err := gotreesitter.NewQuery(goQuery, lang)
	if err != nil {
		return nil, fmt.Errorf("go query: %w", err)
	}
	return &GoExtractor{lang: lang, query: q}, nil
}

func (e *GoExtractor) Supports(ext string) bool {
	return ext == ".go"
}

func (e *GoExtractor) Extract(filePath string, content []byte) ([]Symbol, error) {
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
		kind := goNodeKind(bt, declNode)
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
			Language:  "go",
		})
	}
	return symbols, nil
}

func goNodeKind(bt *gotreesitter.BoundTree, node *gotreesitter.Node) Kind {
	switch bt.NodeType(node) {
	case "function_declaration":
		return KindFunction
	case "method_declaration":
		return KindMethod
	case "const_declaration":
		return KindConst
	case "var_declaration":
		return KindVar
	case "type_declaration":
		text := bt.NodeText(node)
		if contains(text, "interface") {
			return KindInterface
		}
		if contains(text, "struct") {
			return KindStruct
		}
		return KindType
	}
	return KindType
}
