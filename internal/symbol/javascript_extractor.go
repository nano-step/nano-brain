package symbol

import (
	"fmt"

	"github.com/nano-brain/nano-brain/internal/tsparse"
	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

const jsQuery = `
(function_declaration name: (identifier) @name) @decl
(generator_function_declaration name: (identifier) @name) @decl
(method_definition name: (property_identifier) @name) @decl
(field_definition property: [(property_identifier) (private_property_identifier)] @name value: (_) @val) @decl
(class_declaration name: (identifier) @name) @decl
(lexical_declaration (variable_declarator name: (identifier) @name value: (_) @val)) @decl
(variable_declaration (variable_declarator name: (identifier) @name value: (_) @val)) @decl
(pair key: (property_identifier) @name value: (_) @val) @decl
(assignment_expression left: (member_expression property: (property_identifier) @name) right: (_) @val) @decl
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
	switch ext {
	case ".js", ".mjs", ".cjs", ".jsx":
		return true
	default:
		return false
	}
}

func (e *JavaScriptExtractor) Extract(filePath string, content []byte) ([]Symbol, error) {
	parser := tsparse.NewParser(e.lang)
	tree, err := parser.Parse(content)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", filePath, err)
	}

	bt := gotreesitter.Bind(tree)
	defer bt.Release()

	matches := e.query.Execute(tree)
	var symbols []Symbol
	seen := make(map[string]rankedSymbolIndex, len(matches))
	appendJSTSSymbols(&symbols, seen, bt, e.lang, matches, content, filePath, "javascript")
	return symbols, nil
}
