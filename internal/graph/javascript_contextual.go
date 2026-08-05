package graph

import gotreesitter "github.com/odvcencio/gotreesitter"

type jsCallContext struct {
	bt           *gotreesitter.BoundTree
	content      []byte
	filePath     string
	lang         *gotreesitter.Language
	resolver     *callResolver
	imports      map[string]jsImportBinding
	namespaces   map[string]string
	classMethods map[string]map[string]string
	edges        []Edge
}

func (e *JavaScriptGraphExtractor) extractContextualCalls(bt *gotreesitter.BoundTree, tree *gotreesitter.Tree, content []byte, filePath string, ic ImportContext) []Edge {
	ctx := jsCallContext{
		bt:           bt,
		content:      content,
		filePath:     filePath,
		lang:         e.lang,
		resolver:     newCallResolver(filePath, ic),
		imports:      make(map[string]jsImportBinding),
		namespaces:   make(map[string]string),
		classMethods: make(map[string]map[string]string),
	}
	root := tree.RootNode()
	ctx.seedScope(root, ctx.resolver.module)
	ctx.walk(root, ctx.resolver.module, "", "")
	return ctx.edges
}

func (c *jsCallContext) walk(node *gotreesitter.Node, scope *lexicalScope, owner, className string) {
	if node == nil {
		return
	}
	switch node.Type(c.lang) {
	case "source_file", "program":
		for i := 0; i < int(node.ChildCount()); i++ {
			c.walk(node.Child(i), scope, owner, className)
		}
	case "export_statement":
		for i := 0; i < int(node.ChildCount()); i++ {
			c.walk(node.Child(i), scope, owner, className)
		}
	case "statement_block":
		block := scope.childBlock()
		c.seedScope(node, block)
		for i := 0; i < int(node.ChildCount()); i++ {
			c.walk(node.Child(i), block, owner, className)
		}
	case "catch_clause":
		catchScope := scope.childBlock()
		if parameter := node.ChildByFieldName("parameter", c.lang); parameter != nil && parameter.Type(c.lang) == "identifier" {
			catchScope.declareParameter(c.bt.NodeText(parameter))
		}
		for i := 0; i < int(node.ChildCount()); i++ {
			c.walk(node.Child(i), catchScope, owner, className)
		}
	case "function_declaration":
		c.walkFunction(node, scope, c.nodeName(node), "")
	case "function", "function_expression":
		c.walkFunction(node, scope, "", "")
	case "method_definition":
		method := c.nodeName(node)
		if className != "" {
			method = className + "." + method
		}
		c.walkFunction(node, scope, method, className)
	case "class_declaration":
		name := c.nodeName(node)
		if body := firstDescendantOfType(node, c.lang, "class_body"); body != nil {
			c.walk(body, scope, owner, name)
		}
	case "lexical_declaration":
		for i := 0; i < int(node.ChildCount()); i++ {
			decl := node.Child(i)
			if decl == nil || decl.Type(c.lang) != "variable_declarator" {
				continue
			}
			value := decl.ChildByFieldName("value", c.lang)
			if value == nil {
				continue
			}
			switch value.Type(c.lang) {
			case "arrow_function":
				c.walkFunction(value, scope, c.nodeName(decl), className)
			case "function", "function_expression":
				c.walkFunction(value, scope, c.nodeName(decl), "")
			default:
				c.walk(value, scope, owner, className)
			}
		}
	case "call_expression":
		c.addCall(node, scope, owner, className)
		for i := 0; i < int(node.ChildCount()); i++ {
			c.walk(node.Child(i), scope, owner, className)
		}
	default:
		for i := 0; i < int(node.ChildCount()); i++ {
			c.walk(node.Child(i), scope, owner, className)
		}
	}
}

func (c *jsCallContext) walkFunction(node *gotreesitter.Node, outer *lexicalScope, owner, className string) {
	scope := outer.childFunction()
	if parameters := node.ChildByFieldName("parameters", c.lang); parameters != nil {
		for i := 0; i < int(parameters.ChildCount()); i++ {
			param := parameters.Child(i)
			if param.Type(c.lang) == "identifier" {
				scope.declareParameter(c.bt.NodeText(param))
			}
		}
	}
	if body := node.ChildByFieldName("body", c.lang); body != nil {
		c.walk(body, scope, owner, className)
	}
}

func (c *jsCallContext) addCall(node *gotreesitter.Node, scope *lexicalScope, owner, className string) {
	if owner == "" {
		return
	}
	fn := node.ChildByFieldName("function", c.lang)
	target := unresolvedCallTarget
	if fn != nil && fn.Type(c.lang) == "identifier" {
		target = c.resolver.resolveBinding(scope, c.bt.NodeText(fn))
	}
	if fn != nil && fn.Type(c.lang) == "member_expression" {
		target = c.resolveMember(fn, scope, className)
	}
	key := owner + "->" + target
	for _, edge := range c.edges {
		if edge.SourceNode+"->"+edge.TargetNode == c.filePath+"::"+key {
			return
		}
	}
	c.edges = append(c.edges, Edge{SourceNode: c.filePath + "::" + owner, TargetNode: target, Kind: EdgeCalls, SourceFile: c.filePath, Line: lineForByte(c.content, node.StartByte()), Language: "javascript"})
}

func (c *jsCallContext) resolveMember(node *gotreesitter.Node, scope *lexicalScope, className string) string {
	property := node.ChildByFieldName("property", c.lang)
	object := node.ChildByFieldName("object", c.lang)
	if property == nil || object == nil || property.Type(c.lang) != "property_identifier" {
		return unresolvedCallTarget
	}
	method := c.bt.NodeText(property)
	if object.Type(c.lang) == "identifier" {
		name := c.bt.NodeText(object)
		if raw, ok := c.namespaces[name]; ok {
			binding := scope.lookup(name)
			if !binding.unresolved && binding.target == jsNamespaceBindingTarget {
				return c.resolver.resolveImported(raw, method, "")
			}
			return unresolvedCallTarget
		}
		if binding, ok := c.imports[name]; ok {
			active := scope.lookup(name)
			if !active.unresolved && active.target == binding.target {
				return c.resolver.resolveImported(binding.raw, binding.export, method)
			}
			return unresolvedCallTarget
		}
	}
	if c.bt.NodeText(object) == "this" && className != "" {
		if target, ok := c.classMethods[className][method]; ok && IsCanonicalJSCallTarget(target) {
			return target
		}
		return unresolvedCallTarget
	}
	if object.Type(c.lang) == "new_expression" {
		constructor := firstChildOfType(object, c.lang, "identifier")
		if constructor != nil {
			name := c.bt.NodeText(constructor)
			if binding, ok := c.imports[name]; ok {
				active := scope.lookup(name)
				if !active.unresolved && active.target == binding.target {
					return c.resolver.resolveImported(binding.raw, binding.export, method)
				}
				return unresolvedCallTarget
			}
			if active := scope.lookup(name); !active.unresolved && active.target == c.filePath+"::"+name {
				if target, ok := c.classMethods[name][method]; ok && IsCanonicalJSCallTarget(target) {
					return target
				}
				return unresolvedCallTarget
			}
		}
	}
	return unresolvedCallTarget
}

func (c *jsCallContext) nodeName(node *gotreesitter.Node) string {
	name := node.ChildByFieldName("name", c.lang)
	if name == nil {
		return ""
	}
	return c.bt.NodeText(name)
}
