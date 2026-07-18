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

// jsRedisMethods contains all Redis method names that the extractor recognizes.
var jsRedisMethods = map[string]bool{
	"get":       true,
	"set":       true,
	"setEx":     true,
	"expire":    true,
	"del":       true,
	"ttl":       true,
	"publish":   true,
	"subscribe": true,
	"lpush":     true,
	"rpush":     true,
	"sadd":      true,
	"srem":      true,
	"incr":      true,
	"decr":      true,
	"hget":      true,
	"hset":      true,
}

// jsRedisReadMethods are Redis methods that perform read operations.
var jsRedisReadMethods = map[string]bool{
	"get":  true,
	"ttl":  true,
	"hget": true,
}

// jsRedisWriteMethods are Redis methods that perform write operations.
var jsRedisWriteMethods = map[string]bool{
	"set":    true,
	"setEx":  true,
	"expire": true,
	"lpush":  true,
	"rpush":  true,
	"sadd":   true,
	"srem":   true,
	"incr":   true,
	"decr":   true,
	"hset":   true,
}

// jsRedisDeleteMethods are Redis methods that perform delete operations.
var jsRedisDeleteMethods = map[string]bool{
	"del": true,
}

// jsRedisPubSubMethods are Redis methods that perform pub/sub operations.
var jsRedisPubSubMethods = map[string]bool{
	"publish":   true,
	"subscribe": true,
}

// jsRedisUnambiguousMethods are Redis methods that don't overlap with HTTP/queue method names.
var jsRedisUnambiguousMethods = map[string]bool{
	"setEx":  true,
	"expire": true,
	"ttl":    true,
	"lpush":  true,
	"rpush":  true,
	"sadd":   true,
	"srem":   true,
	"incr":   true,
	"decr":   true,
	"hget":   true,
	"hset":   true,
}

// jsRedisReceivers are known Redis client variable names used to disambiguate
// methods that overlap with HTTP/queue (get, set, del, publish, subscribe).
var jsRedisReceivers = map[string]bool{
	"redis":       true,
	"r":           true,
	"client":      true,
	"redisClient": true,
	"cache":       true,
	"db":          true,
}

// isBullQueueReceiver matches variable/class names that follow the common
// Bull/BullMQ queue naming convention (mainQueue, ScreenshotQueue,
// taskQueue, ...). Bull has no fixed client name to key off of like Redis's
// jsRedisReceivers, so this is a substring heuristic rather than an
// exact-match set.
func isBullQueueReceiver(name string) bool {
	return strings.Contains(strings.ToLower(name), "queue")
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

	// arrow_function assigned to variable: const foo = () => { ... } or const foo = () => expr
	walkNodes(root, lang, "arrow_function", func(n *gotreesitter.Node) {
		bodyNode := n.ChildByFieldName("body", lang)
		if bodyNode == nil {
			return
		}
		bodyType := bodyNode.Type(lang)
		if bodyType != "statement_block" && bodyType != "expression" {
			return
		}
		parent := n.Parent()
		if parent != nil && parent.Type(lang) == "variable_declarator" {
			nameNode := parent.ChildByFieldName("name", lang)
			if nameNode != nil {
				funcs = append(funcs, funcRange{
					name:      relFile + "::" + bt.NodeText(nameNode),
					startByte: n.StartByte(),
					endByte:   n.EndByte(),
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
			// Top-level call, outside any named function. Attribute it to a
			// synthetic module symbol and, on the way out, keep only the
			// pub/sub coupling edges (#586). The defer fires on every return
			// path of a handler, including branches that return early.
			source = relFile + moduleSourceSuffix
			before := len(edges)
			defer func() { edges = keepTopLevelCoupling(edges, before) }()
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
			Metadata:   map[string]any{"kind": "queue_publish", "event_role": "publish", "method": funcName, "topic": topic},
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
			Metadata:   map[string]any{"kind": "queue_consumer", "event_role": "subscribe", "method": funcName, "topic": topic},
		})
	}
}

// handleRedisCall emits a cache integration edge for a Redis method call.
func (x *JSIntegrationExtractor) handleRedisCall(bt *gotreesitter.BoundTree, callNode *gotreesitter.Node, methodName, receiverName string, argsNode *gotreesitter.Node, lang *gotreesitter.Language, langLabel, relFile, source string, line int, edges *[]Edge) string {
	key := jsStringArgOrVar(bt, argsNode, lang, 0)
	target := "REDIS " + methodName + " " + key

	var kind string
	switch {
	case jsRedisReadMethods[methodName]:
		kind = "cache_read"
	case jsRedisWriteMethods[methodName]:
		kind = "cache_write"
	case jsRedisDeleteMethods[methodName]:
		kind = "cache_delete"
	case jsRedisPubSubMethods[methodName]:
		kind = "cache_pubsub"
	default:
		kind = "cache_write"
	}

	if kind == "cache_pubsub" {
		topic := jsStringArgOrVar(bt, argsNode, lang, 0)
		// Redis subscribe/consume: emit a "CONSUME <topic>" consumer entry (same
		// shape as the generic message bus) so flow.Stitch can link it to
		// publishers by topic (#546/#577). Previously this produced a
		// "subscribe:<topic>" leaf that ListConsumerEntryNodesByWorkspace never
		// matched, so redis.subscribe was invisible to the stitcher. The publish
		// side stays a topic-carrying publisher edge (still qualifies as a producer).
		if jsConsumeMethods[methodName] {
			consumeNode := "CONSUME " + topic
			*edges = append(*edges, Edge{
				SourceNode: consumeNode,
				TargetNode: source,
				Kind:       EdgeIntegration,
				SourceFile: relFile,
				Line:       line,
				Language:   langLabel,
				Metadata:   map[string]any{"kind": "queue_consumer", "event_role": "subscribe", "method": methodName, "topic": topic},
			})
			return consumeNode
		}
		target = methodName + ":" + topic
		*edges = append(*edges, Edge{
			SourceNode: source,
			TargetNode: target,
			Kind:       EdgeIntegration,
			SourceFile: relFile,
			Line:       line,
			Language:   langLabel,
			Metadata:   map[string]any{"kind": kind, "event_role": "publish", "method": methodName, "receiver": receiverName, "topic": topic},
		})
		return target
	}

	*edges = append(*edges, Edge{
		SourceNode: source,
		TargetNode: target,
		Kind:       EdgeIntegration,
		SourceFile: relFile,
		Line:       line,
		Language:   langLabel,
		Metadata:   map[string]any{"kind": kind, "method": methodName, "receiver": receiverName, "key": key},
	})
	return target
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

	if jsRedisMethods[methodName] {
		isUnambiguous := jsRedisUnambiguousMethods[methodName]
		isKnownReceiver := jsRedisReceivers[receiverName]
		if isUnambiguous || isKnownReceiver {
			x.handleRedisCall(bt, callNode, methodName, receiverName, argsNode, lang, langLabel, relFile, source, line, edges)
			return
		}
	}

	// Bull/BullMQ queue producer/consumer: queue.add("jobName", data) couples to
	// queue.process("jobName", handler) by the job-name string (#546 E2) — not a
	// call, so it needs the same topic-based edge the generic bus/Redis paths
	// use. Gated on a "queue" naming hint plus a literal job-name argument, to
	// avoid misreading an unrelated Set/Map .add()/.process() call as a queue op.
	if (methodName == "add" || methodName == "process") && isBullQueueReceiver(receiverName) {
		firstArg := echoArgNode(argsNode, lang, 0)
		if firstArg != nil && tsCountArgs(argsNode, lang) >= 2 {
			if t := firstArg.Type(lang); t == "string" || t == "template_string" {
				jobName := jsStringArgOrVar(bt, argsNode, lang, 0)
				if methodName == "add" {
					target := "produce:" + jobName
					*edges = append(*edges, Edge{
						SourceNode: source,
						TargetNode: target,
						Kind:       EdgeIntegration,
						SourceFile: relFile,
						Line:       line,
						Language:   langLabel,
						Metadata:   map[string]any{"kind": "queue_publish", "event_role": "publish", "method": methodName, "receiver": receiverName, "topic": jobName},
					})
				} else {
					consumeNode := "CONSUME " + jobName
					*edges = append(*edges, Edge{
						SourceNode: consumeNode,
						TargetNode: source,
						Kind:       EdgeIntegration,
						SourceFile: relFile,
						Line:       line,
						Language:   langLabel,
						Metadata:   map[string]any{"kind": "queue_consumer", "event_role": "subscribe", "method": methodName, "receiver": receiverName, "topic": jobName},
					})
				}
				return
			}
		}
	}

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
			Metadata:   map[string]any{"kind": "queue_publish", "event_role": eventRole(methodName), "method": methodName, "receiver": receiverName, "topic": topic},
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
			Metadata:   map[string]any{"kind": "queue_consumer", "event_role": "subscribe", "method": methodName, "receiver": receiverName, "topic": topic},
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

func eventRole(method string) string {
	if method == "emit" || method == "broadcast" || method == "notify" {
		return "emit"
	}
	return "publish"
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
