package graph

import (
	"path/filepath"
	"strings"

	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

var _ Extractor = (*JSIntegrationExtractor)(nil)

// jsPublishMethods are method names that indicate publishing to a queue,
// event bus, or message broker in JS/TS code.
var jsPublishMethods = map[string]bool{
	"publish":   true,
	"send":      true,
	"emit":      true,
	"enqueue":   true,
	"dispatch":  true,
	"broadcast": true,
	"notify":    true,
	"produce":   true,
	"push":      true,
}

// jsConsumeMethods are method names that indicate consuming from a queue,
// event bus, or message broker in JS/TS code.
var jsConsumeMethods = map[string]bool{
	"subscribe": true,
	"consume":   true,
	"listen":    true,
	"on":        true,
}

// jsHTTPMethodNames are HTTP method names used in shorthand calls like get(url).
var jsHTTPMethodNames = map[string]bool{
	"get":     true,
	"post":    true,
	"put":     true,
	"patch":   true,
	"delete":  true,
	"head":    true,
	"options": true,
}

// JSIntegrationExtractor implements graph.Extractor for JS/TS outbound
// integration calls (HTTP, queue publish, event emit, consumer patterns).
type JSIntegrationExtractor struct {
	jsLang  *gotreesitter.Language
	tsLang  *gotreesitter.Language
	tsxLang *gotreesitter.Language
}

// NewJSIntegrationExtractor constructs a ready-to-use JSIntegrationExtractor.
func NewJSIntegrationExtractor() (*JSIntegrationExtractor, error) {
	return &JSIntegrationExtractor{
		jsLang:  grammars.JavascriptLanguage(),
		tsLang:  grammars.TypescriptLanguage(),
		tsxLang: grammars.TsxLanguage(),
	}, nil
}

// Supports returns true for JS/TS source files.
func (x *JSIntegrationExtractor) Supports(ext string) bool {
	return ext == ".js" || ext == ".jsx" || ext == ".ts" || ext == ".tsx"
}

// ExtractEdges emits EdgeIntegration edges for outbound HTTP, queue publish,
// event emit, and consumer patterns in JS/TS code.
func (x *JSIntegrationExtractor) ExtractEdges(filePath string, content []byte) ([]Edge, error) {
	lang, langLabel := x.langForExt(filepath.Ext(filePath))
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

	// function_declaration: function foo() { ... }
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

	// method_definition: { foo() { ... } } or class { foo() { ... } }
	walkNodes(root, lang, "method_definition", func(n *gotreesitter.Node) {
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

	// arrow_function assigned to variable: const foo = () => { ... }
	walkNodes(root, lang, "arrow_function", func(n *gotreesitter.Node) {
		bodyNode := n.ChildByFieldName("body", lang)
		if bodyNode == nil || bodyNode.Type(lang) != "statement_block" {
			return
		}
		parent := n.Parent()
		if parent != nil && parent.Type(lang) == "variable_declarator" {
			nameNode := parent.ChildByFieldName("name", lang)
			if nameNode != nil {
				funcs = append(funcs, funcRange{
					name:      relFile + "::" + bt.NodeText(nameNode),
					startByte: bodyNode.StartByte(),
					endByte:   bodyNode.EndByte(),
				})
			}
		}
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

	walkNodes(root, lang, "call_expression", func(callNode *gotreesitter.Node) {
		fnNode := callNode.ChildByFieldName("function", lang)
		if fnNode == nil {
			return
		}
		argsNode := callNode.ChildByFieldName("arguments", lang)
		line := lineForByte(content, callNode.StartByte())
		source := enclosingFunc(callNode.StartByte())
		if source == "" {
			return
		}

		switch fnNode.Type(lang) {
		case "identifier":
			x.handleIdentifier(bt, callNode, fnNode, argsNode, lang, langLabel, relFile, source, line, &edges)

		case "member_expression":
			x.handleMemberExpression(bt, callNode, fnNode, argsNode, lang, langLabel, relFile, source, line, &edges)
		}
	})

	return edges, nil
}

func (x *JSIntegrationExtractor) langForExt(ext string) (*gotreesitter.Language, string) {
	switch ext {
	case ".ts":
		return x.tsLang, "typescript"
	case ".tsx":
		return x.tsxLang, "typescript"
	default:
		return x.jsLang, "javascript"
	}
}

// handleIdentifier processes bare function calls like fetch(url) or emit("topic").
func (x *JSIntegrationExtractor) handleIdentifier(bt *gotreesitter.BoundTree, callNode, fnNode, argsNode *gotreesitter.Node, lang *gotreesitter.Language, langLabel, relFile, source string, line int, edges *[]Edge) {
	funcName := bt.NodeText(fnNode)

	// fetch(url) → HTTP GET
	if funcName == "fetch" {
		url := jsStringArgOrVar(bt, argsNode, lang, 0)
		target := "GET " + url
		*edges = append(*edges, Edge{
			SourceNode: source,
			TargetNode: target,
			Kind:       EdgeIntegration,
			SourceFile: relFile,
			Line:       line,
			Language:   langLabel,
			Metadata:   map[string]any{"kind": "http_call", "method": "GET", "url": url},
		})
		return
	}

	// Object config pattern: axios({method, url}) or fetch-like lib with object config
	if argsNode != nil {
		firstArg := echoArgNode(argsNode, lang, 0)
		if firstArg != nil && firstArg.Type(lang) == "object" {
			url := extractObjectPair(bt, firstArg, lang, "url")
			if url != "" {
				method := extractObjectPair(bt, firstArg, lang, "method")
				if method == "" {
					method = "GET"
				}
				target := strings.ToUpper(method) + " " + url
				*edges = append(*edges, Edge{
					SourceNode: source,
					TargetNode: target,
					Kind:       EdgeIntegration,
					SourceFile: relFile,
					Line:       line,
					Language:   langLabel,
					Metadata:   map[string]any{"kind": "http_call", "method": strings.ToUpper(method), "url": url},
				})
				return
			}
		}
	}

	// Bare queue publish: emit("topic") / publish("topic")
	if jsPublishMethods[funcName] {
		topic := jsStringArgOrVar(bt, argsNode, lang, 0)
		target := funcName + ":" + topic
		*edges = append(*edges, Edge{
			SourceNode: source,
			TargetNode: target,
			Kind:       EdgeIntegration,
			SourceFile: relFile,
			Line:       line,
			Language:   langLabel,
			Metadata:   map[string]any{"kind": "queue_publish", "method": funcName, "topic": topic},
		})
		return
	}

	// Bare queue consumer: on("event", handler) / subscribe("channel")
	if jsConsumeMethods[funcName] {
		topic := jsStringArgOrVar(bt, argsNode, lang, 0)
		consumeNode := "CONSUME " + topic
		*edges = append(*edges, Edge{
			SourceNode: consumeNode,
			TargetNode: source,
			Kind:       EdgeIntegration,
			SourceFile: relFile,
			Line:       line,
			Language:   langLabel,
			Metadata:   map[string]any{"kind": "queue_consumer", "method": funcName, "topic": topic},
		})
	}
}

// handleMemberExpression processes method calls like axios.get(url) or emitter.emit("topic").
func (x *JSIntegrationExtractor) handleMemberExpression(bt *gotreesitter.BoundTree, callNode, fnNode, argsNode *gotreesitter.Node, lang *gotreesitter.Language, langLabel, relFile, source string, line int, edges *[]Edge) {
	objectNode := fnNode.ChildByFieldName("object", lang)
	propertyNode := fnNode.ChildByFieldName("property", lang)
	if objectNode == nil || propertyNode == nil {
		return
	}
	methodName := bt.NodeText(propertyNode)
	receiverName := jsReceiverText(bt, objectNode, lang)

	// HTTP method shorthand: <any>.get(url) / <any>.post(url)
	if jsHTTPMethodNames[methodName] {
		url := jsStringArgOrVar(bt, argsNode, lang, 0)
		target := strings.ToUpper(methodName) + " " + url
		*edges = append(*edges, Edge{
			SourceNode: source,
			TargetNode: target,
			Kind:       EdgeIntegration,
			SourceFile: relFile,
			Line:       line,
			Language:   langLabel,
			Metadata:   map[string]any{"kind": "http_call", "method": strings.ToUpper(methodName), "url": url},
		})
		return
	}

	// Object config pattern on member expression: <httpLib>({method, url})
	if argsNode != nil {
		firstArg := echoArgNode(argsNode, lang, 0)
		if firstArg != nil && firstArg.Type(lang) == "object" {
			url := extractObjectPair(bt, firstArg, lang, "url")
			if url != "" {
				method := extractObjectPair(bt, firstArg, lang, "method")
				if method == "" {
					method = "GET"
				}
				target := strings.ToUpper(method) + " " + url
				*edges = append(*edges, Edge{
					SourceNode: source,
					TargetNode: target,
					Kind:       EdgeIntegration,
					SourceFile: relFile,
					Line:       line,
					Language:   langLabel,
					Metadata:   map[string]any{"kind": "http_call", "method": strings.ToUpper(method), "url": url},
				})
				return
			}
		}
	}

	// Queue publish: <any>.emit("topic") / <any>.publish("routingKey")
	if jsPublishMethods[methodName] {
		var topic string
		if methodName == "publish" && receiverName != "redis" {
			// AMQP-style: channel.publish(exchange, routingKey, payload)
			topic = jsStringArgOrVar(bt, argsNode, lang, 1)
		} else {
			topic = jsStringArgOrVar(bt, argsNode, lang, 0)
		}
		target := methodName + ":" + topic
		*edges = append(*edges, Edge{
			SourceNode: source,
			TargetNode: target,
			Kind:       EdgeIntegration,
			SourceFile: relFile,
			Line:       line,
			Language:   langLabel,
			Metadata:   map[string]any{"kind": "queue_publish", "method": methodName, "receiver": receiverName, "topic": topic},
		})
		return
	}

	// Queue consumer: <any>.on("event", handler) / <any>.consume("queue", handler)
	if jsConsumeMethods[methodName] {
		topic := jsStringArgOrVar(bt, argsNode, lang, 0)
		consumeNode := "CONSUME " + topic
		*edges = append(*edges, Edge{
			SourceNode: consumeNode,
			TargetNode: source,
			Kind:       EdgeIntegration,
			SourceFile: relFile,
			Line:       line,
			Language:   langLabel,
			Metadata:   map[string]any{"kind": "queue_consumer", "method": methodName, "receiver": receiverName, "topic": topic},
		})
	}
}

// jsReceiverText extracts a readable receiver name from an expression.
func jsReceiverText(bt *gotreesitter.BoundTree, node *gotreesitter.Node, lang *gotreesitter.Language) string {
	if node == nil {
		return ""
	}
	switch node.Type(lang) {
	case "identifier", "this":
		return bt.NodeText(node)
	default:
		return bt.NodeText(node)
	}
}

// jsStringArgOrVar returns the string value of the nth argument if it is a string
// literal, or a "<var:name>" placeholder for identifiers and expressions.
func jsStringArgOrVar(bt *gotreesitter.BoundTree, argList *gotreesitter.Node, lang *gotreesitter.Language, n int) string {
	node := echoArgNode(argList, lang, n)
	if node == nil {
		return ""
	}
	t := node.Type(lang)
	if t == "string" || t == "template_string" {
		return strings.Trim(bt.NodeText(node), "\"'`")
	}
	text := bt.NodeText(node)
	runes := []rune(text)
	if len(runes) > 40 {
		text = string(runes[:40]) + "…"
	}
	return "<var:" + text + ">"
}

// extractObjectPair finds a key-value pair in an object literal by key name.
func extractObjectPair(bt *gotreesitter.BoundTree, objNode *gotreesitter.Node, lang *gotreesitter.Language, key string) string {
	if objNode == nil || objNode.Type(lang) != "object" {
		return ""
	}
	for i := 0; i < int(objNode.ChildCount()); i++ {
		child := objNode.Child(i)
		if child == nil || child.Type(lang) != "pair" {
			continue
		}
		keyNode := child.ChildByFieldName("key", lang)
		valNode := child.ChildByFieldName("value", lang)
		if keyNode == nil || valNode == nil {
			continue
		}
		if bt.NodeText(keyNode) != key {
			continue
		}
		if valNode.Type(lang) == "string" || valNode.Type(lang) == "template_string" {
			return strings.Trim(bt.NodeText(valNode), "\"'`")
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
