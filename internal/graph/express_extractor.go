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

func (e *ExpressExtractor) RequiresFrameworks() []string {
	return []string{"express"}
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

		argsNode := n.ChildByFieldName("arguments", lang)
		// A single-argument use() call (app.use(corsMiddleware)) registers global
		// middleware, not a route — there is no path/handler pair to extract, and
		// tsExtractPath would otherwise misread the middleware reference as an
		// (unresolvable) path argument and skip the call entirely.
		if method == "USE" && tsCountArgs(argsNode, lang) == 1 {
			arg := tsArgNode(argsNode, lang, 0)
			if arg == nil {
				return
			}
			if t := arg.Type(lang); t == "string" || t == "template_string" {
				return
			}
			name := tsResolveMiddlewareName(bt, arg, lang)
			if name == "" {
				return
			}
			edgeKey := "middleware:" + name + "-><" + receiver + ">"
			if seen[edgeKey] {
				return
			}
			seen[edgeKey] = true
			edges = append(edges, Edge{
				SourceNode: name,
				TargetNode: "<" + receiver + ">",
				Kind:       EdgeMiddleware,
				SourceFile: relFile,
				Line:       lineForByte(content, n.StartByte()),
				Language:   langStr,
			})
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
			edgeKey := "middleware:" + mw + "->" + handler
			if !seen[edgeKey] {
				seen[edgeKey] = true
				edges = append(edges, Edge{
					SourceNode: mw,
					TargetNode: handler,
					Kind:       EdgeMiddleware,
					SourceFile: relFile,
					Line:       lineForByte(content, n.StartByte()),
					Language:   langStr,
				})
			}
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
