package graph

import (
	"fmt"
	"path/filepath"
	"strings"

	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

// NetHTTPExtractor implements graph.Extractor for Go net/http route
// registrations (http.HandleFunc, http.Handle, and gorilla/mux-style
// .HandleFunc(...).Methods(...) chains).
type NetHTTPExtractor struct {
	lang *gotreesitter.Language
}

func NewNetHTTPExtractor() (*NetHTTPExtractor, error) {
	return &NetHTTPExtractor{lang: grammars.GoLanguage()}, nil
}

func (e *NetHTTPExtractor) Supports(ext string) bool {
	return ext == ".go"
}

func (e *NetHTTPExtractor) ExtractEdges(filePath string, content []byte) ([]Edge, error) {
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

	handledHandle := map[uint32]bool{}
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
		argsNode := callNode.ChildByFieldName("arguments", lang)
		if argsNode == nil {
			return
		}

		switch {
		case methodName == "Methods":
			if operandNode.Type(lang) != "call_expression" {
				return
			}
			innerFn := operandNode.ChildByFieldName("function", lang)
			if innerFn == nil || innerFn.Type(lang) != "selector_expression" {
				return
			}
			innerField := innerFn.ChildByFieldName("field", lang)
			if innerField == nil {
				return
			}
			innerMethod := bt.NodeText(innerField)
			if innerMethod != "HandleFunc" && innerMethod != "Handle" {
				return
			}
			handledHandle[operandNode.StartByte()] = true

			innerArgs := operandNode.ChildByFieldName("arguments", lang)
			if innerArgs == nil {
				return
			}
			rawPath := echoStringArg(bt, innerArgs, lang, 0)
			if rawPath == "" {
				return
			}
			handlerNode := echoArgNode(innerArgs, lang, 1)
			handlerName := extractHandlerName(bt, handlerNode, lang, "", rawPath)
			line := lineForByte(content, callNode.StartByte())

			methods := extractMethodArgs(bt, argsNode, lang)
			for _, verb := range methods {
				entryNode := verb + " " + rawPath
				edges = append(edges, Edge{
					SourceNode: entryNode,
					TargetNode: handlerName,
					Kind:       EdgeHTTP,
					SourceFile: relFile,
					Line:       line,
					Language:   "go",
					Metadata: map[string]any{
						"method": verb,
						"path":   rawPath,
					},
				})
			}

		case methodName == "Handle" || methodName == "HandleFunc":
			if handledHandle[callNode.StartByte()] {
				return
			}
			rawPath := echoStringArg(bt, argsNode, lang, 0)
			if rawPath == "" {
				return
			}
			line := lineForByte(content, callNode.StartByte())
			handlerNode := echoArgNode(argsNode, lang, 1)
			handlerName := extractHandlerName(bt, handlerNode, lang, "", rawPath)

			entryNode := "HTTP " + rawPath
			edges = append(edges, Edge{
				SourceNode: entryNode,
				TargetNode: handlerName,
				Kind:       EdgeHTTP,
				SourceFile: relFile,
				Line:       line,
				Language:   "go",
				Metadata: map[string]any{
					"method": "HTTP",
					"path":   rawPath,
				},
			})
		}
	})

	return edges, nil
}

// extractMethodArgs extracts string-literal values from a .Methods("GET", "POST") argument list.
func extractMethodArgs(bt *gotreesitter.BoundTree, argList *gotreesitter.Node, lang *gotreesitter.Language) []string {
	var methods []string
	for i := 0; ; i++ {
		node := echoArgNode(argList, lang, i)
		if node == nil {
			break
		}
		t := node.Type(lang)
		if t == "interpreted_string_literal" || t == "raw_string_literal" {
			methods = append(methods, strings.Trim(bt.NodeText(node), "`\""))
		}
	}
	return methods
}
