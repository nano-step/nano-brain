package graph

import (
	"path/filepath"
	"strings"

	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

var _ Extractor = (*PythonIntegrationExtractor)(nil)

var pyHTTPMethodNames = map[string]bool{
	"get":     true,
	"post":    true,
	"put":     true,
	"patch":   true,
	"delete":  true,
	"head":    true,
	"options": true,
}

var pyPublishMethods = map[string]bool{
	"publish":       true,
	"send":          true,
	"emit":          true,
	"enqueue":       true,
	"dispatch":      true,
	"broadcast":     true,
	"notify":        true,
	"produce":       true,
	"push":          true,
}

var pyConsumeMethods = map[string]bool{
	"subscribe":     true,
	"consume":       true,
	"basic_consume": true,
	"listen":        true,
	"on":            true,
}

type PythonIntegrationExtractor struct {
	lang *gotreesitter.Language
}

func NewPythonIntegrationExtractor() (*PythonIntegrationExtractor, error) {
	return &PythonIntegrationExtractor{lang: grammars.PythonLanguage()}, nil
}

func (x *PythonIntegrationExtractor) Supports(ext string) bool {
	return ext == ".py"
}

func (x *PythonIntegrationExtractor) ExtractEdges(filePath string, content []byte) ([]Edge, error) {
	lang := x.lang
	relFile := filepath.ToSlash(filePath)

	parser := gotreesitter.NewParser(lang)
	tree, err := parser.Parse(content)
	if err != nil {
		return nil, err
	}
	bt := gotreesitter.Bind(tree)
	defer bt.Release()
	root := tree.RootNode()

	type funcRange struct {
		name      string
		startByte uint32
		endByte   uint32
	}
	var funcs []funcRange

	walkNodes(root, lang, "function_definition", func(n *gotreesitter.Node) {
		nameNode := n.ChildByFieldName("name", lang)
		bodyNode := n.ChildByFieldName("body", lang)
		if nameNode == nil || bodyNode == nil {
			return
		}
		funcs = append(funcs, funcRange{
			name:      relFile + "::" + bt.NodeText(nameNode),
			startByte: bodyNode.StartByte(),
			endByte:   bodyNode.EndByte(),
		})
	})

	enclosingFunc := func(offset uint32) string {
		for _, f := range funcs {
			if offset >= f.startByte && offset < f.endByte {
				return f.name
			}
		}
		return ""
	}

	var edges []Edge

	walkNodes(root, lang, "call", func(callNode *gotreesitter.Node) {
		fnNode := callNode.ChildByFieldName("function", lang)
		if fnNode == nil {
			return
		}

		argsNode := callNode.ChildByFieldName("arguments", lang)
		line := lineForByte(content, callNode.StartByte())
		source := enclosingFunc(callNode.StartByte())
		if source == "" {
			// Top-level call, outside any named function. Attribute it to a
			// synthetic module symbol and, on the way out, keep only the
			// pub/sub coupling edges (#586). The defer fires on every return
			// path below, including branches that return early.
			source = relFile + moduleSourceSuffix
			before := len(edges)
			defer func() { edges = keepTopLevelCoupling(edges, before) }()
		}

		switch fnNode.Type(lang) {
		case "attribute":
			objectNode := fnNode.ChildByFieldName("object", lang)
			attrNode := fnNode.ChildByFieldName("attribute", lang)
			if objectNode == nil || attrNode == nil {
				return
			}
			methodName := bt.NodeText(attrNode)
			receiverName := pyReceiverText(bt, objectNode, lang)

			if pyHTTPMethodNames[methodName] {
				url := pyStringArgOrVar(bt, argsNode, lang, 0)
				target := strings.ToUpper(methodName) + " " + url
				edges = append(edges, Edge{
					SourceNode: source,
					TargetNode: target,
					Kind:       EdgeIntegration,
					SourceFile: relFile,
					Line:       line,
					Language:   "python",
					Metadata:   map[string]any{"kind": "http_call", "method": strings.ToUpper(methodName), "url": url},
				})
				return
			}

			if methodName == "basic_publish" {
				topic := pyKeywordArg(bt, argsNode, lang, "routing_key")
				if topic == "" {
					topic = pyStringArgOrVar(bt, argsNode, lang, 1)
				}
				target := "publish:" + topic
				edges = append(edges, Edge{
					SourceNode: source,
					TargetNode: target,
					Kind:       EdgeIntegration,
					SourceFile: relFile,
					Line:       line,
					Language:   "python",
					Metadata:   map[string]any{"kind": "queue_publish", "method": methodName, "receiver": receiverName, "topic": topic},
				})
				return
			}

			if pyPublishMethods[methodName] {
				topic := pyStringArgOrVar(bt, argsNode, lang, 0)
				target := methodName + ":" + topic
				edges = append(edges, Edge{
					SourceNode: source,
					TargetNode: target,
					Kind:       EdgeIntegration,
					SourceFile: relFile,
					Line:       line,
					Language:   "python",
					Metadata:   map[string]any{"kind": "queue_publish", "method": methodName, "receiver": receiverName, "topic": topic},
				})
				return
			}

			if pyConsumeMethods[methodName] {
				topic := pyStringArgOrVar(bt, argsNode, lang, 0)
				consumeNode := "CONSUME " + topic
				edges = append(edges, Edge{
					SourceNode: consumeNode,
					TargetNode: source,
					Kind:       EdgeIntegration,
					SourceFile: relFile,
					Line:       line,
					Language:   "python",
					Metadata:   map[string]any{"kind": "queue_consumer", "method": methodName, "receiver": receiverName, "topic": topic},
				})
				return
			}

		case "identifier":
			funcName := bt.NodeText(fnNode)

			if pyPublishMethods[funcName] {
				topic := pyStringArgOrVar(bt, argsNode, lang, 0)
				target := funcName + ":" + topic
				edges = append(edges, Edge{
					SourceNode: source,
					TargetNode: target,
					Kind:       EdgeIntegration,
					SourceFile: relFile,
					Line:       line,
					Language:   "python",
					Metadata:   map[string]any{"kind": "queue_publish", "method": funcName, "topic": topic},
				})
				return
			}

			if pyConsumeMethods[funcName] {
				topic := pyStringArgOrVar(bt, argsNode, lang, 0)
				consumeNode := "CONSUME " + topic
				edges = append(edges, Edge{
					SourceNode: consumeNode,
					TargetNode: source,
					Kind:       EdgeIntegration,
					SourceFile: relFile,
					Line:       line,
					Language:   "python",
					Metadata:   map[string]any{"kind": "queue_consumer", "method": funcName, "topic": topic},
				})
				return
			}
		}
	})

	return edges, nil
}

func pyReceiverText(bt *gotreesitter.BoundTree, node *gotreesitter.Node, lang *gotreesitter.Language) string {
	if node == nil {
		return ""
	}
	switch node.Type(lang) {
	case "identifier":
		return bt.NodeText(node)
	default:
		return bt.NodeText(node)
	}
}

func pyStringArgOrVar(bt *gotreesitter.BoundTree, argList *gotreesitter.Node, lang *gotreesitter.Language, n int) string {
	node := pyPositionalArg(argList, lang, n)
	if node == nil {
		return ""
	}
	t := node.Type(lang)
	if t == "string" {
		return strings.Trim(bt.NodeText(node), "'\"")
	}
	text := bt.NodeText(node)
	runes := []rune(text)
	if len(runes) > 40 {
		text = string(runes[:40]) + "…"
	}
	return "<var:" + text + ">"
}

func pyPositionalArg(argList *gotreesitter.Node, lang *gotreesitter.Language, n int) *gotreesitter.Node {
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
		if t == "," || t == "(" || t == ")" || t == "keyword_argument" {
			continue
		}
		if idx == n {
			return child
		}
		idx++
	}
	return nil
}

func pyKeywordArg(bt *gotreesitter.BoundTree, argList *gotreesitter.Node, lang *gotreesitter.Language, name string) string {
	if argList == nil {
		return ""
	}
	for i := 0; i < int(argList.ChildCount()); i++ {
		child := argList.Child(i)
		if child == nil || child.Type(lang) != "keyword_argument" {
			continue
		}
		keyNode := child.ChildByFieldName("name", lang)
		valNode := child.ChildByFieldName("value", lang)
		if keyNode == nil || valNode == nil {
			continue
		}
		if bt.NodeText(keyNode) != name {
			continue
		}
		if valNode.Type(lang) == "string" {
			return strings.Trim(bt.NodeText(valNode), "'\"")
		}
		text := bt.NodeText(valNode)
		runes := []rune(text)
		if len(runes) > 40 {
			text = string(runes[:40]) + "…"
		}
		return "<var:" + text + ">"
	}
	return ""
}
