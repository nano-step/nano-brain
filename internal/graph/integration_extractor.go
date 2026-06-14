package graph

import (
	"path/filepath"
	"strings"

	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

var _ Extractor = (*IntegrationExtractor)(nil) // compile-time interface check

// integrationPublishMethods are method names that indicate publishing to a queue,
// event bus, or message broker.
var integrationPublishMethods = map[string]bool{
	"Publish":   true,
	"Send":      true,
	"Emit":      true,
	"Enqueue":   true,
	"Dispatch":  true,
	"Broadcast": true,
	"Notify":    true,
	"Produce":   true,
	"Push":      true,
}

// httpShorthandMethods are stdlib http package functions that perform outbound calls.
var httpShorthandMethods = map[string]bool{
	"Get":    true,
	"Post":   true,
	"Put":    true,
	"Delete": true,
	"Head":   true,
	"Do":     true,
}

// IntegrationExtractor implements graph.Extractor for outbound integration calls.
// It detects:
//   - http.NewRequest / http.NewRequestWithContext → outbound HTTP edges
//   - http.Get / http.Post / http.Put / http.Delete / http.Do → outbound HTTP edges
//   - <receiver>.Publish / .Send / .Emit / .Enqueue / .Dispatch / etc. → queue/event edges
type IntegrationExtractor struct {
	lang *gotreesitter.Language
}

// NewIntegrationExtractor constructs a ready-to-use IntegrationExtractor.
func NewIntegrationExtractor() (*IntegrationExtractor, error) {
	return &IntegrationExtractor{lang: grammars.GoLanguage()}, nil
}

// Supports returns true for Go source files.
func (x *IntegrationExtractor) Supports(ext string) bool {
	return ext == ".go"
}

// ExtractEdges emits EdgeIntegration edges for outbound HTTP and queue calls.
func (x *IntegrationExtractor) ExtractEdges(filePath string, content []byte) ([]Edge, error) {
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

	// Build a slice of enclosing function ranges so we can map a call-site byte
	// offset to the function that contains it.
	type funcRange struct {
		name      string
		startByte uint32
		endByte   uint32
	}
	var funcs []funcRange

	walkNodes(root, lang, "function_declaration", func(n *gotreesitter.Node) {
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

	walkNodes(root, lang, "method_declaration", func(n *gotreesitter.Node) {
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

	// enclosingFunc returns the fully-qualified source node (file::Func) for a
	// given byte offset within the file. Returns "" if no match.
	enclosingFunc := func(offset uint32) string {
		for _, f := range funcs {
			if offset >= f.startByte && offset < f.endByte {
				return f.name
			}
		}
		return ""
	}

	var edges []Edge

	walkNodes(root, lang, "call_expression", func(callNode *gotreesitter.Node) {
		fnNode := callNode.ChildByFieldName("function", lang)
		if fnNode == nil {
			return
		}

		argsNode := callNode.ChildByFieldName("arguments", lang)
		line := lineForByte(content, callNode.StartByte())
		source := enclosingFunc(callNode.StartByte())
		if source == "" {
			return // top-level call, skip
		}

		switch fnNode.Type(lang) {
		case "selector_expression":
			operandNode := fnNode.ChildByFieldName("operand", lang)
			fieldNode := fnNode.ChildByFieldName("field", lang)
			if operandNode == nil || fieldNode == nil {
				return
			}
			methodName := bt.NodeText(fieldNode)
			receiverName := leafIdentText(bt, operandNode, lang)

			// http.NewRequest(method, url, body) — URL at arg[1]
			// http.NewRequestWithContext(ctx, method, url, body) — URL at arg[2]
			if receiverName == "http" && (methodName == "NewRequest" || methodName == "NewRequestWithContext") {
				urlIdx := 1
				if methodName == "NewRequestWithContext" {
					urlIdx = 2
				}
				method := stringArgOrVar(bt, argsNode, lang, urlIdx-1)
				url := stringArgOrVar(bt, argsNode, lang, urlIdx)
				target := integrationHTTPTarget(method, url)
				edges = append(edges, Edge{
					SourceNode: source,
					TargetNode: target,
					Kind:       EdgeIntegration,
					SourceFile: relFile,
					Line:       line,
					Language:   "go",
					Metadata:   map[string]any{"kind": "http_call", "method": method, "url": url},
				})
				return
			}

			// http.Get(url) / http.Post(url, ...) / http.Do(req)
			if receiverName == "http" && httpShorthandMethods[methodName] {
				url := stringArgOrVar(bt, argsNode, lang, 0)
				target := integrationHTTPTarget(strings.ToUpper(methodName), url)
				edges = append(edges, Edge{
					SourceNode: source,
					TargetNode: target,
					Kind:       EdgeIntegration,
					SourceFile: relFile,
					Line:       line,
					Language:   "go",
					Metadata:   map[string]any{"kind": "http_call", "method": strings.ToUpper(methodName), "url": url},
				})
				return
			}

			// <receiver>.Publish/Send/Emit/Enqueue/... (topic, payload)
			if integrationPublishMethods[methodName] {
				topic := stringArgOrVar(bt, argsNode, lang, 0)
				target := integrationQueueTarget(methodName, receiverName, topic)
				edges = append(edges, Edge{
					SourceNode: source,
					TargetNode: target,
					Kind:       EdgeIntegration,
					SourceFile: relFile,
					Line:       line,
					Language:   "go",
					Metadata:   map[string]any{"kind": "queue_publish", "method": methodName, "receiver": receiverName, "topic": topic},
				})
			}

		case "identifier":
			// Bare function call: Publish(...) / Send(...) — less common but handle it.
			methodName := bt.NodeText(fnNode)
			if integrationPublishMethods[methodName] {
				topic := stringArgOrVar(bt, argsNode, lang, 0)
				target := integrationQueueTarget(methodName, "", topic)
				edges = append(edges, Edge{
					SourceNode: source,
					TargetNode: target,
					Kind:       EdgeIntegration,
					SourceFile: relFile,
					Line:       line,
					Language:   "go",
					Metadata:   map[string]any{"kind": "queue_publish", "method": methodName, "topic": topic},
				})
			}
		}
	})

	return edges, nil
}

// stringArgOrVar returns the string value of the nth argument if it is a string
// literal, or a "<var:name>" placeholder for identifiers and expressions.
func stringArgOrVar(bt *gotreesitter.BoundTree, argList *gotreesitter.Node, lang *gotreesitter.Language, n int) string {
	node := echoArgNode(argList, lang, n)
	if node == nil {
		return ""
	}
	t := node.Type(lang)
	if t == "interpreted_string_literal" || t == "raw_string_literal" {
		return strings.Trim(bt.NodeText(node), "`\"")
	}
	// Variable or expression: return a readable placeholder.
	text := bt.NodeText(node)
	if len(text) > 40 {
		text = text[:40] + "…"
	}
	return "<var:" + text + ">"
}

// integrationHTTPTarget constructs the target node id for an outbound HTTP edge.
func integrationHTTPTarget(method, url string) string {
	if method == "" {
		method = "HTTP"
	}
	if url == "" {
		url = "<unknown>"
	}
	// Strip scheme for brevity: "https://api.example.com/v1" → "api.example.com/v1"
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")
	return method + " " + url
}

// integrationQueueTarget constructs the target node id for a queue/event edge.
func integrationQueueTarget(method, receiver, topic string) string {
	if topic != "" {
		return method + ":" + topic
	}
	if receiver != "" {
		return method + "(" + receiver + ")"
	}
	return method + "(<unknown>)"
}
