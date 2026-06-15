package graph

import (
	"fmt"
	"path/filepath"
	"strings"

	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
	zerolog "github.com/rs/zerolog"
)

var _ Extractor = (*NestJSExtractor)(nil)

var nestJSHTTPDecorators = map[string]string{
	"Get":     "GET",
	"Post":    "POST",
	"Put":     "PUT",
	"Delete":  "DELETE",
	"Patch":   "PATCH",
	"Head":    "HEAD",
	"Options": "OPTIONS",
	"All":     "ALL",
}

type NestJSExtractor struct {
	logger zerolog.Logger
}

func NewNestJSExtractor(logger zerolog.Logger) (*NestJSExtractor, error) {
	return &NestJSExtractor{
		logger: logger.With().Str("component", "nestjs-extractor").Logger(),
	}, nil
}

func (e *NestJSExtractor) Supports(ext string) bool {
	return ext == ".ts" || ext == ".tsx"
}

func (e *NestJSExtractor) RequiresFrameworks() []string {
	return []string{"nestjs"}
}

func (e *NestJSExtractor) ExtractEdges(filePath string, content []byte) ([]Edge, error) {
	ext := filepath.Ext(filePath)
	lang := grammars.TypescriptLanguage()
	langStr := "typescript"
	if ext == ".tsx" {
		langStr = "typescript"
	}
	relFile := filepath.ToSlash(filePath)

	parser := gotreesitter.NewParser(lang)
	tree, err := parser.Parse(content)
	if err != nil {
		return nil, fmt.Errorf("nestjs parse %s: %w", filePath, err)
	}
	bt := gotreesitter.Bind(tree)
	defer bt.Release()

	rootNode := tree.RootNode()

	if !tsHasNestJSPatterns(rootNode, lang, bt) {
		return nil, nil
	}

	controllers := make(map[string]string)

	walkNodes(rootNode, lang, "decorator", func(n *gotreesitter.Node) {
		callExpr := decoratorCallExpr(n, lang)
		if callExpr == nil {
			return
		}
		funcName := callExprFuncName(bt, callExpr, lang)
		if funcName != "Controller" {
			return
		}
		className := classFromDecorator(n, lang, bt)
		if className == "" {
			return
		}
		prefix := callExprStringArg(bt, callExpr, lang)
		controllers[className] = prefix
	})

	if len(controllers) == 0 {
		return nil, nil
	}

	var edges []Edge
	seen := make(map[string]bool)

	walkNodes(rootNode, lang, "decorator", func(n *gotreesitter.Node) {
		callExpr := decoratorCallExpr(n, lang)
		if callExpr == nil {
			return
		}
		funcName := callExprFuncName(bt, callExpr, lang)
		method, ok := nestJSHTTPDecorators[funcName]
		if !ok {
			return
		}

		methodDef := methodFromDecorator(n, lang)
		if methodDef == nil {
			return
		}
		methodName := methodDefName(bt, methodDef, lang)
		if methodName == "" {
			return
		}

		className := classFromMethod(methodDef, lang, bt)
		if className == "" {
			return
		}
		prefix, isController := controllers[className]
		if !isController {
			return
		}

		pathArg := callExprStringArg(bt, callExpr, lang)
		fullPath := combineNestJSPaths(prefix, pathArg)

		handler := className + "." + methodName
		source := strings.TrimSpace(method + " " + fullPath)
		if seen[source] {
			return
		}
		seen[source] = true

		edges = append(edges, Edge{
			SourceNode: source,
			TargetNode: handler,
			Kind:       EdgeHTTP,
			SourceFile: relFile,
			Line:       lineForByte(content, n.StartByte()),
			Language:   langStr,
			Metadata:   map[string]any{"method": method, "path": fullPath},
		})
	})

	if len(edges) == 0 {
		return nil, nil
	}
	return edges, nil
}

func tsHasNestJSPatterns(root *gotreesitter.Node, lang *gotreesitter.Language, bt *gotreesitter.BoundTree) bool {
	found := false
	walkNodes(root, lang, "decorator", func(n *gotreesitter.Node) {
		if found {
			return
		}
		callExpr := decoratorCallExpr(n, lang)
		if callExpr == nil {
			return
		}
		name := callExprFuncName(bt, callExpr, lang)
		if _, ok := nestJSHTTPDecorators[name]; ok {
			found = true
			return
		}
		if name == "Controller" {
			found = true
		}
	})
	return found
}

func decoratorCallExpr(decorator *gotreesitter.Node, lang *gotreesitter.Language) *gotreesitter.Node {
	for i := 0; i < int(decorator.ChildCount()); i++ {
		child := decorator.Child(i)
		if child != nil && child.Type(lang) == "call_expression" {
			return child
		}
	}
	return nil
}

func callExprFuncName(bt *gotreesitter.BoundTree, callExpr *gotreesitter.Node, lang *gotreesitter.Language) string {
	for i := 0; i < int(callExpr.ChildCount()); i++ {
		child := callExpr.Child(i)
		if child != nil && child.Type(lang) == "identifier" {
			return bt.NodeText(child)
		}
	}
	return ""
}

func callExprStringArg(bt *gotreesitter.BoundTree, callExpr *gotreesitter.Node, lang *gotreesitter.Language) string {
	argsNode := callExpr.ChildByFieldName("arguments", lang)
	if argsNode == nil {
		return ""
	}
	arg := tsArgNode(argsNode, lang, 0)
	if arg == nil {
		return ""
	}
	if arg.Type(lang) == "string" {
		return cleanRoutePath(strings.Trim(bt.NodeText(arg), "\"'"))
	}
	return ""
}

func classFromDecorator(decorator *gotreesitter.Node, lang *gotreesitter.Language, bt *gotreesitter.BoundTree) string {
	parent := decorator.Parent()
	if parent == nil {
		return ""
	}
	for i := 0; i < int(parent.ChildCount()); i++ {
		child := parent.Child(i)
		if child != nil && child.Type(lang) == "class_declaration" {
			return className(bt, child, lang)
		}
	}
	return ""
}

func className(bt *gotreesitter.BoundTree, classDecl *gotreesitter.Node, lang *gotreesitter.Language) string {
	nameNode := classDecl.ChildByFieldName("name", lang)
	if nameNode != nil {
		return bt.NodeText(nameNode)
	}
	for i := 0; i < int(classDecl.ChildCount()); i++ {
		child := classDecl.Child(i)
		if child != nil && child.Type(lang) == "type_identifier" {
			return bt.NodeText(child)
		}
	}
	return ""
}

func methodFromDecorator(decorator *gotreesitter.Node, lang *gotreesitter.Language) *gotreesitter.Node {
	parent := decorator.Parent()
	if parent == nil {
		return nil
	}
	decIdx := -1
	for i := 0; i < int(parent.ChildCount()); i++ {
		if parent.Child(i) == decorator {
			decIdx = i
			break
		}
	}
	if decIdx < 0 {
		return nil
	}
	for i := decIdx + 1; i < int(parent.ChildCount()); i++ {
		child := parent.Child(i)
		if child != nil && child.Type(lang) == "method_definition" {
			return child
		}
	}
	return nil
}

func methodDefName(bt *gotreesitter.BoundTree, methodDef *gotreesitter.Node, lang *gotreesitter.Language) string {
	nameNode := methodDef.ChildByFieldName("name", lang)
	if nameNode != nil {
		return bt.NodeText(nameNode)
	}
	for i := 0; i < int(methodDef.ChildCount()); i++ {
		child := methodDef.Child(i)
		if child != nil && child.Type(lang) == "property_identifier" {
			return bt.NodeText(child)
		}
	}
	return ""
}

func classFromMethod(methodDef *gotreesitter.Node, lang *gotreesitter.Language, bt *gotreesitter.BoundTree) string {
	parent := methodDef.Parent()
	if parent == nil || parent.Type(lang) != "class_body" {
		return ""
	}
	classDecl := parent.Parent()
	if classDecl == nil || classDecl.Type(lang) != "class_declaration" {
		return ""
	}
	return className(bt, classDecl, lang)
}

func combineNestJSPaths(prefix, methodPath string) string {
	prefix = strings.Trim(prefix, "/")
	methodPath = strings.Trim(methodPath, "/")

	var parts []string
	if prefix != "" {
		parts = append(parts, prefix)
	}
	if methodPath != "" {
		parts = append(parts, methodPath)
	}
	if len(parts) == 0 {
		return "/"
	}
	return "/" + strings.Join(parts, "/")
}
