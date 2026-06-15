package graph

import (
	"fmt"
	"path/filepath"
	"strings"

	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
	zerolog "github.com/rs/zerolog"
)

var _ Extractor = (*ExpressExtractor)(nil)

type ExpressExtractor struct {
	logger zerolog.Logger
}

func NewExpressExtractor(logger zerolog.Logger) (*ExpressExtractor, error) {
	return &ExpressExtractor{
		logger: logger.With().Str("component", "express-extractor").Logger(),
	}, nil
}

func (e *ExpressExtractor) Supports(ext string) bool {
	return ext == ".ts" || ext == ".tsx" || ext == ".js" || ext == ".jsx"
}

func (e *ExpressExtractor) ExtractEdges(filePath string, content []byte) ([]Edge, error) {
	ext := filepath.Ext(filePath)
	lang := grammars.TypescriptLanguage()
	langStr := "typescript"
	if ext == ".js" || ext == ".jsx" {
		lang = grammars.JavascriptLanguage()
		langStr = "javascript"
	}
	relFile := filepath.ToSlash(filePath)

	parser := gotreesitter.NewParser(lang)
	tree, err := parser.Parse(content)
	if err != nil {
		return nil, fmt.Errorf("express parse %s: %w", filePath, err)
	}
	bt := gotreesitter.Bind(tree)
	defer bt.Release()

	rootNode := tree.RootNode()
	if !tsHasExpressPatterns(rootNode, lang, bt) && !tsIsExpressImport(rootNode, lang, bt) {
		return nil, nil
	}

	var edges []Edge
	seen := make(map[string]bool)

	extractHTTP := func(n *gotreesitter.Node) {
		method := tsExtractHTTPMethod(bt, n, lang)
		if method == "" || strings.HasPrefix(method, "<") {
			return
		}

		receiver := tsExtractReceiverName(bt, n, lang)
		if receiver == "" {
			return
		}

		path := tsExtractPath(bt, n, lang)
		if path == "" {
			e.logger.Warn().Str("file", filePath).Str("method", method).Msg("empty path in route")
			return
		}
		if strings.HasPrefix(path, "<var:") {
			e.logger.Warn().Str("file", filePath).Str("method", method).Msg("template string route (skipping)")
			return
		}

		handler := tsExtractHandlerName(bt, n, lang, method, path)
		middleware := tsExtractMiddleware(bt, n, lang)

		source := strings.TrimSpace(method + " " + path)
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
			Metadata:   map[string]any{"method": method, "path": path},
		})

		for _, mw := range middleware {
			edgeKey := "middleware:" + source + "->" + mw
			if !seen[edgeKey] {
				seen[edgeKey] = true
				edges = append(edges, Edge{
					SourceNode: source,
					TargetNode: mw,
					Kind:       EdgeMiddleware,
					SourceFile: relFile,
					Line:       lineForByte(content, n.StartByte()),
					Language:   langStr,
				})
			}
		}
	}

	extractMiddleware := func(n *gotreesitter.Node) {
		fnNode := n.ChildByFieldName("function", lang)
		if fnNode == nil || fnNode.Type(lang) != "member_expression" {
			return
		}
		objectNode := fnNode.ChildByFieldName("object", lang)
		if objectNode == nil {
			return
		}
		receiver := bt.NodeText(objectNode)
		if receiver == "" {
			return
		}
		method := tsExtractHTTPMethod(bt, n, lang)
		if method != "USE" {
			return
		}

		argsNode := n.ChildByFieldName("arguments", lang)
		if argsNode == nil {
			return
		}
		argCount := tsCountArgs(argsNode, lang)
		if argCount == 0 {
			return
		}

		// Determine if first arg is a path string (path-prefix form).
		// app.use("/api", mw1, mw2) → skip path string and handler (last arg)
		// app.use(mw1, mw2)         → emit all args
		hasPathPrefix := false
		if firstNode := tsArgNode(argsNode, lang, 0); firstNode != nil {
			t := firstNode.Type(lang)
			hasPathPrefix = t == "string" || t == "template_string"
		}
		maxIdx := argCount
		if hasPathPrefix && argCount > 1 {
			maxIdx = argCount - 1
		}

		for i := 0; i < maxIdx; i++ {
			handlerNode := tsArgNode(argsNode, lang, i)
			if handlerNode == nil {
				continue
			}
			handlerName := tsResolveMiddlewareName(bt, handlerNode, lang)
			if handlerName == "" {
				continue
			}

			source := "<" + receiver + ".use>"
			edgeKey := "middleware:" + source + "->" + handlerName
			if seen[edgeKey] {
				continue
			}
			seen[edgeKey] = true

			edges = append(edges, Edge{
				SourceNode: source,
				TargetNode: handlerName,
				Kind:       EdgeMiddleware,
				SourceFile: relFile,
				Line:       lineForByte(content, n.StartByte()),
				Language:   langStr,
			})
		}
	}

	walkNodes(rootNode, lang, "call_expression", func(n *gotreesitter.Node) {
		fnNode := n.ChildByFieldName("function", lang)
		if fnNode == nil {
			return
		}
		if fnNode.Type(lang) == "member_expression" {
			method := tsExtractHTTPMethod(bt, n, lang)
			if method != "" {
				extractHTTP(n)
			}
			if method == "USE" {
				extractMiddleware(n)
			}
		} else if fnNode.Type(lang) == "identifier" && bt.NodeText(fnNode) == "express" {
			handler := tsExtractHandlerName(bt, n, lang, "USE", "/")
			if strings.HasPrefix(handler, "<anonymous_") {
				return
			}
			edgeKey := "express:default->" + handler
			if !seen[edgeKey] {
				seen[edgeKey] = true
				edges = append(edges, Edge{
					SourceNode: "<express.use>",
					TargetNode: handler,
					Kind:       EdgeMiddleware,
					SourceFile: relFile,
					Line:       lineForByte(content, n.StartByte()),
					Language:   langStr,
				})
			}
		}
	})

	if len(edges) == 0 {
		return nil, nil
	}
	return edges, nil
}
