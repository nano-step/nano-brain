package graph

import (
	"strings"

	gotreesitter "github.com/odvcencio/gotreesitter"
)

// httpVerbs is the set of standard HTTP method names.
var httpVerbs = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "DELETE": true,
	"PATCH": true, "HEAD": true, "OPTIONS": true,
}

// extractHandlerName derives the bare handler name from a handler argument node.
// For inline closures it returns a synthetic "<inline:METHOD path>" name so the
// edge is never emitted with an empty target.
func extractHandlerName(bt *gotreesitter.BoundTree, node *gotreesitter.Node, lang *gotreesitter.Language, method, path string) string {
	if node == nil {
		return "<inline:" + method + " " + path + ">"
	}
	switch node.Type(lang) {
	case "func_literal":
		return "<inline:" + method + " " + path + ">"
	case "identifier":
		return bt.NodeText(node)
	case "selector_expression":
		field := node.ChildByFieldName("field", lang)
		if field != nil {
			return bt.NodeText(field)
		}
		return bt.NodeText(node)
	case "call_expression":
		fn := node.ChildByFieldName("function", lang)
		if fn != nil {
			return extractHandlerName(bt, fn, lang, method, path)
		}
		return bt.NodeText(node)
	}
	name := bt.NodeText(node)
	if name == "" {
		return "<inline:" + method + " " + path + ">"
	}
	return name
}

// cleanRoutePath normalizes a route path (e.g. strips trailing slashes).
func cleanRoutePath(path string) string {
	return strings.TrimSuffix(path, "/")
}
