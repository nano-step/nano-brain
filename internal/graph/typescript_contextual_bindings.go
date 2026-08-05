package graph

import (
	"strings"

	gotreesitter "github.com/odvcencio/gotreesitter"
)

type tsImportBinding struct {
	raw    string
	export string
	target string
}

const tsNamespaceBindingTarget = "\x00namespace-import"

func (c *tsCallContext) seedScope(node *gotreesitter.Node, scope *lexicalScope) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type(c.lang) {
		case "import_statement":
			c.bindImport(child, scope)
		case "export_statement":
			c.seedExport(child, scope)
		case "function_declaration":
			c.declareFunction(child, scope)
		case "class_declaration":
			c.declareClass(child, scope)
		case "lexical_declaration":
			c.declareLexical(child, scope)
		case "variable_declaration":
			c.declareVar(child, scope)
		}
	}
}

func (c *tsCallContext) seedExport(node *gotreesitter.Node, scope *lexicalScope) {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		switch child.Type(c.lang) {
		case "function_declaration":
			c.declareFunction(child, scope)
		case "class_declaration":
			c.declareClass(child, scope)
		case "lexical_declaration":
			c.declareLexical(child, scope)
		}
	}
}

func (c *tsCallContext) bindImport(node *gotreesitter.Node, scope *lexicalScope) {
	source := node.ChildByFieldName("source", c.lang)
	if source == nil {
		return
	}
	raw := strings.Trim(c.bt.NodeText(source), "\"'`")
	clause := firstDescendantOfType(node, c.lang, "import_clause")
	if clause == nil {
		return
	}
	forEachImportBinding(clause, c.lang, c.content, func(local, exported string, namespace bool) {
		if namespace {
			c.namespaces[local] = raw
			scope.declareLexical(local, tsNamespaceBindingTarget)
			return
		}
		c.bindImported(scope, local, raw, exported)
	})
}

func (c *tsCallContext) bindImported(scope *lexicalScope, local, raw, export string) {
	target := c.resolver.resolveImported(raw, export, "")
	c.imports[local] = tsImportBinding{raw: raw, export: export, target: target}
	scope.declareLexical(local, target)
}

func (c *tsCallContext) declareFunction(node *gotreesitter.Node, scope *lexicalScope) {
	name := node.ChildByFieldName("name", c.lang)
	if name != nil && isIdentifier(c.bt.NodeText(name)) {
		scope.declareFunction(c.bt.NodeText(name), c.filePath+"::"+c.bt.NodeText(name))
	}
}

func (c *tsCallContext) declareClass(node *gotreesitter.Node, scope *lexicalScope) {
	name := node.ChildByFieldName("name", c.lang)
	if name == nil || !isIdentifier(c.bt.NodeText(name)) {
		return
	}
	className := c.bt.NodeText(name)
	target := c.filePath + "::" + className
	scope.declareClass(className, target)
	methods := make(map[string]string)
	receiver := make(map[string]string)
	if body := firstDescendantOfType(node, c.lang, "class_body"); body != nil {
		for i := 0; i < int(body.ChildCount()); i++ {
			member := body.Child(i)
			if member.Type(c.lang) == "method_definition" {
				methodName := member.ChildByFieldName("name", c.lang)
				if methodName != nil && isIdentifier(c.bt.NodeText(methodName)) {
					methods[c.bt.NodeText(methodName)] = target + "." + c.bt.NodeText(methodName)
				}
			}
			if member.Type(c.lang) == "public_field_definition" {
				c.declareTypedField(member, receiver)
			}
			if member.Type(c.lang) == "method_definition" && c.bt.NodeText(member.ChildByFieldName("name", c.lang)) == "constructor" {
				c.declareParameterProperties(member, receiver)
			}
		}
	}
	c.classMethods[className] = methods
	c.receivers[className] = receiver
}

func (c *tsCallContext) declareTypedField(node *gotreesitter.Node, receiver map[string]string) {
	if hasDescendantOfType(node, c.lang, "decorator") {
		return
	}
	name := node.ChildByFieldName("name", c.lang)
	typeName := c.simpleNamedType(node)
	if name == nil || typeName == "" || !isIdentifier(c.bt.NodeText(name)) {
		return
	}
	receiver[c.bt.NodeText(name)] = typeName
}

func (c *tsCallContext) declareParameterProperties(node *gotreesitter.Node, receiver map[string]string) {
	parameters := node.ChildByFieldName("parameters", c.lang)
	if parameters == nil {
		return
	}
	for i := 0; i < int(parameters.ChildCount()); i++ {
		parameter := parameters.Child(i)
		if parameter.Type(c.lang) != "required_parameter" || hasDescendantOfType(parameter, c.lang, "decorator") || !hasDescendantOfType(parameter, c.lang, "accessibility_modifier") {
			continue
		}
		name := firstChildOfType(parameter, c.lang, "identifier")
		typeName := c.simpleNamedType(parameter)
		if name != nil && typeName != "" {
			receiver[c.bt.NodeText(name)] = typeName
		}
	}
}

func (c *tsCallContext) simpleNamedType(node *gotreesitter.Node) string {
	annotation := firstChildOfType(node, c.lang, "type_annotation")
	if annotation == nil {
		return ""
	}
	typeName := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(c.bt.NodeText(annotation)), ":"))
	if !isIdentifier(typeName) {
		return ""
	}
	return typeName
}

func (c *tsCallContext) declareLexical(node *gotreesitter.Node, scope *lexicalScope) {
	for i := 0; i < int(node.ChildCount()); i++ {
		decl := node.Child(i)
		if decl.Type(c.lang) != "variable_declarator" {
			continue
		}
		name := decl.ChildByFieldName("name", c.lang)
		if name != nil && isIdentifier(c.bt.NodeText(name)) {
			scope.declareLexical(c.bt.NodeText(name), "")
		}
	}
}

func (c *tsCallContext) declareVar(node *gotreesitter.Node, scope *lexicalScope) {
	for i := 0; i < int(node.ChildCount()); i++ {
		decl := node.Child(i)
		if decl.Type(c.lang) != "variable_declarator" {
			continue
		}
		name := decl.ChildByFieldName("name", c.lang)
		if name != nil && isIdentifier(c.bt.NodeText(name)) {
			scope.declareVar(c.bt.NodeText(name))
		}
	}
}

func hasDescendantOfType(node *gotreesitter.Node, lang *gotreesitter.Language, want string) bool {
	return firstDescendantOfType(node, lang, want) != nil
}
