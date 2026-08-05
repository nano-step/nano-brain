package graph

import gotreesitter "github.com/odvcencio/gotreesitter"

type tsCallContext struct {
	bt           *gotreesitter.BoundTree
	content      []byte
	filePath     string
	lang         *gotreesitter.Language
	resolver     *callResolver
	imports      map[string]tsImportBinding
	namespaces   map[string]string
	classMethods map[string]map[string]string
	receivers    map[string]map[string]string
	edges        []Edge
}

func (e *TypeScriptGraphExtractor) extractContextualCalls(bt *gotreesitter.BoundTree, tree *gotreesitter.Tree, content []byte, filePath string, lang *gotreesitter.Language, ic ImportContext) []Edge {
	ctx := tsCallContext{
		bt:           bt,
		content:      content,
		filePath:     filePath,
		lang:         lang,
		resolver:     newCallResolver(filePath, ic),
		imports:      make(map[string]tsImportBinding),
		namespaces:   make(map[string]string),
		classMethods: make(map[string]map[string]string),
		receivers:    make(map[string]map[string]string),
	}
	root := tree.RootNode()
	ctx.seedScope(root, ctx.resolver.module)
	ctx.walk(root, ctx.resolver.module, "", "")
	return ctx.edges
}

func (c *tsCallContext) walk(node *gotreesitter.Node, scope *lexicalScope, owner, className string) {
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
		c.walkFunction(node, scope, owner, "")
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

func (c *tsCallContext) walkFunction(node *gotreesitter.Node, outer *lexicalScope, owner, className string) {
	scope := outer.childFunction()
	if parameters := node.ChildByFieldName("parameters", c.lang); parameters != nil {
		for i := 0; i < int(parameters.ChildCount()); i++ {
			parameter := parameters.Child(i)
			if parameter.Type(c.lang) == "identifier" {
				scope.declareParameter(c.bt.NodeText(parameter))
			}
			if parameter.Type(c.lang) == "required_parameter" {
				if name := firstChildOfType(parameter, c.lang, "identifier"); name != nil {
					scope.declareParameter(c.bt.NodeText(name))
				}
			}
		}
	}
	if body := node.ChildByFieldName("body", c.lang); body != nil {
		c.walk(body, scope, owner, className)
	}
}

func (c *tsCallContext) addCall(node *gotreesitter.Node, scope *lexicalScope, owner, className string) {
	if owner == "" {
		return
	}
	target := unresolvedCallTarget
	fn := node.ChildByFieldName("function", c.lang)
	if fn != nil && fn.Type(c.lang) == "identifier" {
		target = c.resolver.resolveBinding(scope, c.bt.NodeText(fn))
	}
	if fn != nil && fn.Type(c.lang) == "member_expression" {
		target = c.resolveMember(fn, scope, className)
	}
	source := c.filePath + "::" + owner
	for _, edge := range c.edges {
		if edge.SourceNode == source && edge.TargetNode == target {
			return
		}
	}
	c.edges = append(c.edges, Edge{SourceNode: source, TargetNode: target, Kind: EdgeCalls, SourceFile: c.filePath, Line: lineForByte(c.content, node.StartByte()), Language: "typescript"})
}

func (c *tsCallContext) resolveMember(node *gotreesitter.Node, scope *lexicalScope, className string) string {
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
			if !binding.unresolved && binding.target == tsNamespaceBindingTarget {
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
		return c.resolveClassMethod(className, method)
	}
	if object.Type(c.lang) == "member_expression" && className != "" {
		field := object.ChildByFieldName("property", c.lang)
		base := object.ChildByFieldName("object", c.lang)
		if field != nil && base != nil && field.Type(c.lang) == "property_identifier" && c.bt.NodeText(base) == "this" {
			return c.resolveReceiverType(c.receivers[className][c.bt.NodeText(field)], method)
		}
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
				return c.resolveClassMethod(name, method)
			}
		}
	}
	return unresolvedCallTarget
}

func (c *tsCallContext) resolveReceiverType(typeName, method string) string {
	if typeName == "" {
		return unresolvedCallTarget
	}
	if imported, ok := c.imports[typeName]; ok {
		return c.resolver.resolveImported(imported.raw, imported.export, method)
	}
	return c.resolveClassMethod(typeName, method)
}

func (c *tsCallContext) resolveClassMethod(className, method string) string {
	target := c.classMethods[className][method]
	if !IsCanonicalJSCallTarget(target) {
		return unresolvedCallTarget
	}
	return target
}

func (c *tsCallContext) nodeName(node *gotreesitter.Node) string {
	name := node.ChildByFieldName("name", c.lang)
	if name == nil {
		return ""
	}
	return c.bt.NodeText(name)
}
