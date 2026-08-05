package graph

import (
	"strings"

	gotreesitter "github.com/odvcencio/gotreesitter"
)

type jsImportBinding struct {
	raw    string
	export string
	target string
}

const jsNamespaceBindingTarget = "\x00namespace-import"

func (c *jsCallContext) seedScope(node *gotreesitter.Node, scope *lexicalScope) {
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

func (c *jsCallContext) seedExport(node *gotreesitter.Node, scope *lexicalScope) {
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

func (c *jsCallContext) bindImport(node *gotreesitter.Node, scope *lexicalScope) {
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
			scope.declareLexical(local, jsNamespaceBindingTarget)
			return
		}
		c.bindImported(scope, local, raw, exported)
	})
}

func (c *jsCallContext) bindImported(scope *lexicalScope, local, raw, export string) {
	target := c.resolver.resolveImported(raw, export, "")
	c.imports[local] = jsImportBinding{raw: raw, export: export, target: target}
	scope.declareLexical(local, target)
}

func (c *jsCallContext) declareFunction(node *gotreesitter.Node, scope *lexicalScope) {
	name := node.ChildByFieldName("name", c.lang)
	if name != nil && isIdentifier(c.bt.NodeText(name)) {
		scope.declareFunction(c.bt.NodeText(name), c.filePath+"::"+c.bt.NodeText(name))
	}
}

func (c *jsCallContext) declareClass(node *gotreesitter.Node, scope *lexicalScope) {
	name := node.ChildByFieldName("name", c.lang)
	if name == nil || !isIdentifier(c.bt.NodeText(name)) {
		return
	}
	className := c.bt.NodeText(name)
	target := c.filePath + "::" + className
	scope.declareClass(className, target)
	methods := make(map[string]string)
	if body := firstDescendantOfType(node, c.lang, "class_body"); body != nil {
		for i := 0; i < int(body.ChildCount()); i++ {
			method := body.Child(i)
			if method.Type(c.lang) != "method_definition" {
				continue
			}
			methodName := method.ChildByFieldName("name", c.lang)
			if methodName != nil && isIdentifier(c.bt.NodeText(methodName)) {
				methods[c.bt.NodeText(methodName)] = target + "." + c.bt.NodeText(methodName)
			}
		}
	}
	c.classMethods[className] = methods
}

func (c *jsCallContext) declareLexical(node *gotreesitter.Node, scope *lexicalScope) {
	for i := 0; i < int(node.ChildCount()); i++ {
		decl := node.Child(i)
		if decl.Type(c.lang) != "variable_declarator" {
			continue
		}
		name := decl.ChildByFieldName("name", c.lang)
		if name == nil || !isIdentifier(c.bt.NodeText(name)) {
			continue
		}
		target := ""
		if value := decl.ChildByFieldName("value", c.lang); value != nil && (value.Type(c.lang) == "arrow_function" || value.Type(c.lang) == "function") {
			target = c.filePath + "::" + c.bt.NodeText(name)
		}
		scope.declareLexical(c.bt.NodeText(name), target)
	}
}

func (c *jsCallContext) declareVar(node *gotreesitter.Node, scope *lexicalScope) {
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
