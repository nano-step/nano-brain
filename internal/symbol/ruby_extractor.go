package symbol

import (
	"fmt"

	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

const rubyQuery = `
(method name: (identifier) @name) @decl
(singleton_method name: (identifier) @name) @decl
(class name: (constant) @name) @decl
(module name: (constant) @name) @decl
(assignment left: (constant) @name) @decl
`

type RubySymbolExtractor struct {
	lang  *gotreesitter.Language
	query *gotreesitter.Query
}

func NewRubySymbolExtractor() (*RubySymbolExtractor, error) {
	lang := grammars.RubyLanguage()
	q, err := gotreesitter.NewQuery(rubyQuery, lang)
	if err != nil {
		return nil, fmt.Errorf("ruby query: %w", err)
	}
	return &RubySymbolExtractor{lang: lang, query: q}, nil
}

func (e *RubySymbolExtractor) Supports(ext string) bool {
	return ext == ".rb"
}

func (e *RubySymbolExtractor) Extract(filePath string, content []byte) ([]Symbol, error) {
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
		kind := rubyNodeKind(bt, declNode)
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
			Language:  "ruby",
		})
	}
	return symbols, nil
}

func rubyNodeKind(bt *gotreesitter.BoundTree, node *gotreesitter.Node) Kind {
	switch bt.NodeType(node) {
	case "class", "module":
		return KindType
	case "method":
		return KindFunction
	case "assignment":
		return KindConst
	}
	return KindFunction
}
