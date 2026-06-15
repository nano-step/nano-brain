package graph

import (
	"strings"

	gotreesitter "github.com/odvcencio/gotreesitter"
)

// tsExtractHTTPMethod extracts the HTTP method from a call expression like app.get(...) or router.post(...).
func tsExtractHTTPMethod(bt *gotreesitter.BoundTree, callNode *gotreesitter.Node, lang *gotreesitter.Language) string {
	if callNode == nil {
		return ""
	}
	fnNode := callNode.ChildByFieldName("function", lang)
	if fnNode == nil || fnNode.Type(lang) != "member_expression" {
		return ""
	}
	propertyNode := fnNode.ChildByFieldName("property", lang)
	if propertyNode == nil {
		return ""
	}
	method := bt.NodeText(propertyNode)
	switch method {
	case "get", "post", "put", "delete", "patch", "all", "head", "options", "use":
		return strings.ToUpper(method)
	}
	return ""
}

// tsExtractReceiverName extracts the receiver name from a member expression (e.g., "app" from app.get).
func tsExtractReceiverName(bt *gotreesitter.BoundTree, callNode *gotreesitter.Node, lang *gotreesitter.Language) string {
	if callNode == nil {
		return ""
	}
	fnNode := callNode.ChildByFieldName("function", lang)
	if fnNode == nil || fnNode.Type(lang) != "member_expression" {
		return ""
	}
	objectNode := fnNode.ChildByFieldName("object", lang)
	if objectNode == nil {
		return ""
	}
	return bt.NodeText(objectNode)
}

// tsExtractPath extracts the string path argument from an Express route call.
func tsExtractPath(bt *gotreesitter.BoundTree, callNode *gotreesitter.Node, lang *gotreesitter.Language) string {
	if callNode == nil {
		return ""
	}
	argsNode := callNode.ChildByFieldName("arguments", lang)
	if argsNode == nil {
		return ""
	}
	node := tsArgNode(argsNode, lang, 0)
	if node == nil {
		return ""
	}
	t := node.Type(lang)
	if t == "string" || (t == "template_string" && !strings.Contains(bt.NodeText(node), "${")) {
		path := strings.Trim(bt.NodeText(node), "\"'`")
		return cleanRoutePath(path)
	}
	text := bt.NodeText(node)
	runes := []rune(text)
	if len(runes) > 40 {
		text = string(runes[:40]) + "…"
	}
	return "<var:" + text + ">"
}

// tsExtractHandlerName extracts the handler function/variable name from a call expression.
func tsExtractHandlerName(bt *gotreesitter.BoundTree, callNode *gotreesitter.Node, lang *gotreesitter.Language, method, path string) string {
	if callNode == nil {
		return "<anonymous_" + method + " " + path + ">"
	}
	argsNode := callNode.ChildByFieldName("arguments", lang)
	if argsNode == nil {
		return "<anonymous_" + method + " " + path + ">"
	}
	var handlerNode *gotreesitter.Node
	argCount := tsCountArgs(argsNode, lang)
	if argCount > 0 {
		handlerNode = tsArgNode(argsNode, lang, argCount-1)
	}
	if handlerNode == nil {
		return "<anonymous_" + method + " " + path + ">"
	}
	return tsResolveHandlerName(bt, handlerNode, lang, method, path)
}

// tsResolveHandlerName resolves a handler name from an expression node.
func tsResolveHandlerName(bt *gotreesitter.BoundTree, node *gotreesitter.Node, lang *gotreesitter.Language, method, path string) string {
	if node == nil {
		return "<anonymous_" + method + " " + path + ">"
	}

	switch node.Type(lang) {
	case "identifier":
		return bt.NodeText(node)
	case "member_expression":
		objectNode := node.ChildByFieldName("object", lang)
		propertyNode := node.ChildByFieldName("property", lang)
		if objectNode != nil && propertyNode != nil {
			return bt.NodeText(objectNode) + "." + bt.NodeText(propertyNode)
		}
		return bt.NodeText(node)
	case "arrow_function", "function":
		return "<anonymous_" + method + " " + path + ">"
	case "call_expression":
		fnNode := node.ChildByFieldName("function", lang)
		if fnNode != nil {
			return tsResolveHandlerName(bt, fnNode, lang, method, path)
		}
	}

	name := bt.NodeText(node)
	if name == "" {
		return "<anonymous_" + method + " " + path + ">"
	}
	return name
}

// tsExtractMiddleware extracts middleware names from a call expression's arguments.
func tsExtractMiddleware(bt *gotreesitter.BoundTree, callNode *gotreesitter.Node, lang *gotreesitter.Language) []string {
	if callNode == nil {
		return nil
	}
	argsNode := callNode.ChildByFieldName("arguments", lang)
	if argsNode == nil {
		return nil
	}

	argCount := tsCountArgs(argsNode, lang)
	if argCount <= 1 {
		return nil
	}

	var middleware []string
	for i := 0; i < argCount-1; i++ {
		node := tsArgNode(argsNode, lang, i)
		if node == nil {
			continue
		}
		name := tsResolveMiddlewareName(bt, node, lang)
		if name != "" {
			middleware = append(middleware, name)
		}
	}
	return middleware
}

// tsResolveMiddlewareName resolves a middleware name from an expression node.
func tsResolveMiddlewareName(bt *gotreesitter.BoundTree, node *gotreesitter.Node, lang *gotreesitter.Language) string {
	if node == nil {
		return ""
	}
	switch node.Type(lang) {
	case "identifier":
		return bt.NodeText(node)
	case "member_expression":
		objectNode := node.ChildByFieldName("object", lang)
		propertyNode := node.ChildByFieldName("property", lang)
		if objectNode != nil && propertyNode != nil {
			return bt.NodeText(objectNode) + "." + bt.NodeText(propertyNode)
		}
		return bt.NodeText(node)
	case "call_expression":
		return ""
	case "arrow_function", "function":
		return ""
	}
	return ""
}

// tsArgNode returns the nth value node inside an argument list (skipping punctuation).
func tsArgNode(argList *gotreesitter.Node, lang *gotreesitter.Language, n int) *gotreesitter.Node {
	if argList == nil {
		return nil
	}
	idx := 0
	for i := 0; i < int(argList.ChildCount()); i++ {
		child := argList.Child(i)
		if child == nil {
			continue
		}
		t := child.Type(lang)
		if t == "," || t == "(" || t == ")" || t == ";" || t == "comment" {
			continue
		}
		if idx == n {
			return child
		}
		idx++
	}
	return nil
}

// tsCountArgs counts the number of value arguments in an argument list (skipping punctuation and comments).
func tsCountArgs(argList *gotreesitter.Node, lang *gotreesitter.Language) int {
	if argList == nil {
		return 0
	}
	count := 0
	for i := 0; i < int(argList.ChildCount()); i++ {
		child := argList.Child(i)
		if child == nil {
			continue
		}
		t := child.Type(lang)
		if t == "," || t == "(" || t == ")" || t == ";" || t == "comment" {
			continue
		}
		count++
	}
	return count
}

// tsIsExpressImport checks if a file has an Express import or require statement.
func tsIsExpressImport(root *gotreesitter.Node, lang *gotreesitter.Language, bt *gotreesitter.BoundTree) bool {
	found := false
	walkNodes(root, lang, "import_statement", func(n *gotreesitter.Node) {
		sourceNode := n.ChildByFieldName("source", lang)
		if sourceNode != nil {
			source := strings.Trim(bt.NodeText(sourceNode), "\"'")
			if source == "express" {
				found = true
			}
		}
	})
	if found {
		return true
	}

	walkNodes(root, lang, "call_expression", func(n *gotreesitter.Node) {
		fnNode := n.ChildByFieldName("function", lang)
		if fnNode != nil && bt.NodeText(fnNode) == "require" {
			argsNode := n.ChildByFieldName("arguments", lang)
			if argsNode != nil {
				arg := tsArgNode(argsNode, lang, 0)
				if arg != nil {
					t := arg.Type(lang)
					if t == "string" {
						source := strings.Trim(bt.NodeText(arg), "\"'")
						if source == "express" {
							found = true
						}
					}
				}
			}
		}
	})
	return found
}

// tsHasExpressPatterns checks if a file has Express-like route patterns.
func tsHasExpressPatterns(root *gotreesitter.Node, lang *gotreesitter.Language, bt *gotreesitter.BoundTree) bool {
	found := false
	walkNodes(root, lang, "call_expression", func(n *gotreesitter.Node) {
		fnNode := n.ChildByFieldName("function", lang)
		if fnNode == nil || fnNode.Type(lang) != "member_expression" {
			return
		}
		propertyNode := fnNode.ChildByFieldName("property", lang)
		if propertyNode == nil {
			return
		}
		method := bt.NodeText(propertyNode)
		switch method {
		case "get", "post", "put", "delete", "patch", "all", "head", "options", "use", "route":
			objectNode := fnNode.ChildByFieldName("object", lang)
			if objectNode != nil {
				receiver := bt.NodeText(objectNode)
				switch receiver {
				case "app", "router", "server", "express":
					found = true
				}
			}
		}
	})
	return found
}
