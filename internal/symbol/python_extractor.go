package symbol

import (
	"fmt"

	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

const pyQuery = `
(function_definition name: (identifier) @name) @decl
(class_definition name: (identifier) @name) @decl
`

type PythonExtractor struct {
	lang  *gotreesitter.Language
	query *gotreesitter.Query
}

func NewPythonExtractor() (*PythonExtractor, error) {
	lang := grammars.PythonLanguage()
	q, err := gotreesitter.NewQuery(pyQuery, lang)
	if err != nil {
		return nil, fmt.Errorf("python query: %w", err)
	}
	return &PythonExtractor{lang: lang, query: q}, nil
}

func (e *PythonExtractor) Supports(ext string) bool {
	return ext == ".py"
}

func (e *PythonExtractor) Extract(filePath string, content []byte) ([]Symbol, error) {
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
		kind := pyNodeKind(bt, declNode, nameNode)
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
			Language:  "python",
		})
	}
	return symbols, nil
}

func pyNodeKind(bt *gotreesitter.BoundTree, decl, name *gotreesitter.Node) Kind {
	if bt.NodeType(decl) == "class_definition" {
		return KindType
	}
	if isInsideClassBody(bt, name) {
		return KindMethod
	}
	return KindFunction
}

func isInsideClassBody(bt *gotreesitter.BoundTree, name *gotreesitter.Node) bool {
	n := name.Parent()
	for n != nil {
		t := bt.NodeType(n)
		if t == "class_definition" {
			return true
		}
		if t == "module" {
			return false
		}
		n = n.Parent()
	}
	return false
}
