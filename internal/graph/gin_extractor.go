package graph

import (
	"fmt"
	"path/filepath"

	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

// ginVerbs is the set of Gin HTTP method names (including Any for all-methods).
var ginVerbs = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "DELETE": true,
	"PATCH": true, "HEAD": true, "OPTIONS": true,
	"Any": true,
}

// GinExtractor implements graph.Extractor for Gin HTTP route registration.
// It emits EdgeHTTP and EdgeMiddleware edges by walking the Go AST via tree-sitter.
type GinExtractor struct {
	lang *gotreesitter.Language
}

func NewGinExtractor() (*GinExtractor, error) {
	return &GinExtractor{lang: grammars.GoLanguage()}, nil
}

func (e *GinExtractor) Supports(ext string) bool {
	return ext == ".go"
}

func (e *GinExtractor) RequiresFrameworks() []string {
	return []string{"gin"}
}

func (e *GinExtractor) ExtractEdges(filePath string, content []byte) ([]Edge, error) {
	parser := gotreesitter.NewParser(e.lang)
	tree, err := parser.Parse(content)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", filePath, err)
	}
	bt := gotreesitter.Bind(tree)
	defer bt.Release()

	relFile := filepath.ToSlash(filePath)
	lang := e.lang
	root := tree.RootNode()

	callToVar := map[uint32]string{}
	walkNodes(root, lang, "short_var_declaration", func(n *gotreesitter.Node) {
		left := n.ChildByFieldName("left", lang)
		right := n.ChildByFieldName("right", lang)
		if left == nil || right == nil {
			return
		}
		varName := ""
		for i := 0; i < int(left.ChildCount()); i++ {
			child := left.Child(i)
			if child == nil {
				continue
			}
			if child.Type(lang) == "identifier" {
				varName = bt.NodeText(child)
				break
			}
		}
		if varName == "" {
			return
		}
		for i := 0; i < int(right.ChildCount()); i++ {
			child := right.Child(i)
			if child == nil {
				continue
			}
			if child.Type(lang) == "call_expression" {
				callToVar[child.StartByte()] = varName
				break
			}
		}
	})

	groupPrefix := map[string]string{}
	groupMiddleware := map[string][]string{}

	var edges []Edge

	walkNodes(root, lang, "call_expression", func(callNode *gotreesitter.Node) {
		fnNode := callNode.ChildByFieldName("function", lang)
		if fnNode == nil || fnNode.Type(lang) != "selector_expression" {
			return
		}

		fieldNode := fnNode.ChildByFieldName("field", lang)
		operandNode := fnNode.ChildByFieldName("operand", lang)
		if fieldNode == nil || operandNode == nil {
			return
		}

		methodName := bt.NodeText(fieldNode)
		recvName := leafIdentText(bt, operandNode, lang)

		argsNode := callNode.ChildByFieldName("arguments", lang)
		if argsNode == nil {
			return
		}

		switch {
		case methodName == "Group":
			varName, ok := callToVar[callNode.StartByte()]
			if !ok {
				return
			}
			prefix := echoStringArg(bt, argsNode, lang, 0)
			groupPrefix[varName] = groupPrefix[recvName] + prefix

		case methodName == "Use":
			mws := echoArgNames(bt, argsNode, lang, 0)
			groupMiddleware[recvName] = append(groupMiddleware[recvName], mws...)

		case ginVerbs[methodName]:
			rawPath := echoStringArg(bt, argsNode, lang, 0)
			if rawPath == "" {
				return
			}
			fullPath := groupPrefix[recvName] + rawPath
			line := lineForByte(content, callNode.StartByte())

			handlerNode := echoArgNode(argsNode, lang, 1)
			handlerName := extractHandlerName(bt, handlerNode, lang, methodName, fullPath)

			if methodName == "Any" {
				for _, verb := range []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"} {
					entryNode := verb + " " + fullPath
					edges = append(edges, Edge{
						SourceNode: entryNode,
						TargetNode: handlerName,
						Kind:       EdgeHTTP,
						SourceFile: relFile,
						Line:       line,
						Language:   "go",
						Metadata: map[string]any{
							"method": verb,
							"path":   fullPath,
						},
					})
				}
			} else {
				entryNode := methodName + " " + fullPath
				edges = append(edges, Edge{
					SourceNode: entryNode,
					TargetNode: handlerName,
					Kind:       EdgeHTTP,
					SourceFile: relFile,
					Line:       line,
					Language:   "go",
					Metadata: map[string]any{
						"method": methodName,
						"path":   fullPath,
					},
				})
			}

			allMWs := append([]string{}, groupMiddleware[recvName]...)
			for _, mw := range allMWs {
				if mw == "" {
					continue
				}
				edges = append(edges, Edge{
					SourceNode: mw,
					TargetNode: handlerName,
					Kind:       EdgeMiddleware,
					SourceFile: relFile,
					Line:       line,
					Language:   "go",
				})
			}
		}
	})

	return edges, nil
}
