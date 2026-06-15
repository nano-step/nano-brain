package graph

import (
	"fmt"
	"path/filepath"
	"strings"

	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
	zerolog "github.com/rs/zerolog"
)

var _ Extractor = (*RailsExtractor)(nil)

var railsRestfulActions = []struct {
	Action string
	Method string
	Path   string
}{
	{Action: "index", Method: "GET", Path: ""},
	{Action: "create", Method: "POST", Path: ""},
	{Action: "show", Method: "GET", Path: ":id"},
	{Action: "update", Method: "PATCH", Path: ":id"},
	{Action: "destroy", Method: "DELETE", Path: ":id"},
	{Action: "new", Method: "GET", Path: "new"},
	{Action: "edit", Method: "GET", Path: ":id/edit"},
}

type RailsExtractor struct {
	logger zerolog.Logger
}

func NewRailsExtractor(logger zerolog.Logger) (*RailsExtractor, error) {
	return &RailsExtractor{
		logger: logger.With().Str("component", "rails-extractor").Logger(),
	}, nil
}

func (e *RailsExtractor) Supports(ext string) bool {
	return ext == ".rb"
}

func (e *RailsExtractor) RequiresFrameworks() []string {
	return []string{"rails"}
}

func (e *RailsExtractor) ExtractEdges(filePath string, content []byte) ([]Edge, error) {
	if !isRoutesFile(filePath) {
		return nil, nil
	}
	relFile := filepath.ToSlash(filePath)
	lang := grammars.RubyLanguage()

	parser := gotreesitter.NewParser(lang)
	tree, err := parser.Parse(content)
	if err != nil {
		return nil, fmt.Errorf("rails parse %s: %w", filePath, err)
	}
	bt := gotreesitter.Bind(tree)
	defer bt.Release()

	drawBody := findDrawBlockBody(bt, tree.RootNode(), lang)
	if drawBody == nil {
		e.logger.Warn().Str("file", filePath).Msg("no draw block found in routes.rb")
		return nil, nil
	}

	ctx := &railsCtx{}
	edges := e.walkBody(drawBody, lang, bt, ctx, content, relFile)
	if len(edges) == 0 {
		return nil, nil
	}
	return edges, nil
}

type railsCtx struct {
	namespace []string
	scope     []string
}

func (c *railsCtx) clone() *railsCtx {
	ns := make([]string, len(c.namespace))
	copy(ns, c.namespace)
	sc := make([]string, len(c.scope))
	copy(sc, c.scope)
	return &railsCtx{namespace: ns, scope: sc}
}

func (c *railsCtx) urlPrefix() string {
	var parts []string
	for _, ns := range c.namespace {
		parts = append(parts, ns)
	}
	for _, s := range c.scope {
		parts = append(parts, s)
	}
	return strings.Join(parts, "/")
}

func (c *railsCtx) modulePrefix() string {
	var parts []string
	for _, ns := range c.namespace {
		parts = append(parts, camelCase(ns))
	}
	return strings.Join(parts, "::")
}

func (e *RailsExtractor) walkBody(body *gotreesitter.Node, lang *gotreesitter.Language, bt *gotreesitter.BoundTree, ctx *railsCtx, content []byte, relFile string) []Edge {
	if body == nil {
		return nil
	}
	var edges []Edge

	for i := 0; i < int(body.ChildCount()); i++ {
		child := body.Child(i)
		if child == nil || child.Type(lang) != "call" {
			continue
		}
		methodName := rubyCallMethod(bt, child, lang)
		if methodName == "" {
			continue
		}

		switch methodName {
		case "get", "post", "put", "patch", "delete":
			edges = append(edges, e.extractDirectRoute(bt, child, lang, strings.ToUpper(methodName), ctx, content, relFile)...)
		case "resources":
			edges = append(edges, e.extractResources(bt, child, lang, ctx, content, relFile)...)
		case "namespace":
			edges = append(edges, e.extractNamespace(bt, child, lang, ctx, content, relFile)...)
		case "scope":
			edges = append(edges, e.extractScope(bt, child, lang, ctx, content, relFile)...)
		case "mount":
			edges = append(edges, e.extractMount(bt, child, lang, ctx, content, relFile)...)
		case "root":
			edges = append(edges, e.extractRoot(bt, child, lang, ctx, content, relFile)...)
		case "devise_for":
			edges = append(edges, e.extractDeviseFor(bt, child, lang, ctx, content, relFile)...)
		}
	}

	return edges
}

func (e *RailsExtractor) walkResourceBlock(body *gotreesitter.Node, lang *gotreesitter.Language, bt *gotreesitter.BoundTree, resName string, ctx *railsCtx, content []byte, relFile string) []Edge {
	if body == nil {
		return nil
	}
	var edges []Edge

	for i := 0; i < int(body.ChildCount()); i++ {
		child := body.Child(i)
		if child == nil || child.Type(lang) != "call" {
			continue
		}
		methodName := rubyCallMethod(bt, child, lang)
		if methodName == "" {
			continue
		}

		switch methodName {
		case "collection":
			if blkBody := rubyCallBlockBody(child, lang); blkBody != nil {
				edges = append(edges, e.walkCollectionBlock(blkBody, lang, bt, resName, ctx, content, relFile)...)
			}
		case "member":
			if blkBody := rubyCallBlockBody(child, lang); blkBody != nil {
				edges = append(edges, e.walkMemberBlock(blkBody, lang, bt, resName, ctx, content, relFile)...)
			}
		}
	}

	return edges
}

func (e *RailsExtractor) walkCollectionBlock(body *gotreesitter.Node, lang *gotreesitter.Language, bt *gotreesitter.BoundTree, resName string, ctx *railsCtx, content []byte, relFile string) []Edge {
	if body == nil {
		return nil
	}
	var edges []Edge
	prefix := ctx.urlPrefix()
	if prefix != "" {
		prefix += "/"
	}
	prefix += resName

	ctrlName := controllerName(ctx.modulePrefix(), resName)

	for i := 0; i < int(body.ChildCount()); i++ {
		child := body.Child(i)
		if child == nil || child.Type(lang) != "call" {
			continue
		}
		methodName := rubyCallMethod(bt, child, lang)
		if methodName == "" {
			continue
		}
		switch methodName {
		case "get", "post", "put", "patch", "delete":
			action := rubyFirstStringOrSymbol(bt, child, lang)
			if action == "" {
				continue
			}
			fullPath := "/" + prefix + "/" + action
			source := strings.ToUpper(methodName) + " " + fullPath
			handler := ctrlName + "#" + action
			edges = append(edges, Edge{
				SourceNode: source,
				TargetNode: handler,
				Kind:       EdgeHTTP,
				SourceFile: relFile,
				Line:       lineForByte(content, child.StartByte()),
				Language:   "ruby",
				Metadata:   map[string]any{"method": strings.ToUpper(methodName), "path": fullPath},
			})
		}
	}

	return edges
}

func (e *RailsExtractor) walkMemberBlock(body *gotreesitter.Node, lang *gotreesitter.Language, bt *gotreesitter.BoundTree, resName string, ctx *railsCtx, content []byte, relFile string) []Edge {
	if body == nil {
		return nil
	}
	var edges []Edge
	prefix := ctx.urlPrefix()
	if prefix != "" {
		prefix += "/"
	}
	prefix += resName + "/:id"

	ctrlName := controllerName(ctx.modulePrefix(), resName)

	for i := 0; i < int(body.ChildCount()); i++ {
		child := body.Child(i)
		if child == nil || child.Type(lang) != "call" {
			continue
		}
		methodName := rubyCallMethod(bt, child, lang)
		if methodName == "" {
			continue
		}
		switch methodName {
		case "get", "post", "put", "patch", "delete":
			action := rubyFirstStringOrSymbol(bt, child, lang)
			if action == "" {
				continue
			}
			fullPath := "/" + prefix + "/" + action
			source := strings.ToUpper(methodName) + " " + fullPath
			handler := ctrlName + "#" + action
			edges = append(edges, Edge{
				SourceNode: source,
				TargetNode: handler,
				Kind:       EdgeHTTP,
				SourceFile: relFile,
				Line:       lineForByte(content, child.StartByte()),
				Language:   "ruby",
				Metadata:   map[string]any{"method": strings.ToUpper(methodName), "path": fullPath},
			})
		}
	}

	return edges
}

func (e *RailsExtractor) extractDirectRoute(bt *gotreesitter.BoundTree, call *gotreesitter.Node, lang *gotreesitter.Language, method string, ctx *railsCtx, content []byte, relFile string) []Edge {
	routePath, handler := extractRouteArgs(bt, call, lang)
	if routePath == "" || handler == "" {
		return nil
	}
	if strings.HasPrefix(handler, "redirect") {
		return nil
	}

	prefix := ctx.urlPrefix()
	fullPath := routePath
	if prefix != "" {
		fullPath = "/" + prefix + "/" + strings.TrimPrefix(routePath, "/")
	}
	ctrlHandler := qualifyHandler(ctx.modulePrefix(), handler)

	source := method + " " + fullPath
	return []Edge{{
		SourceNode: source,
		TargetNode: ctrlHandler,
		Kind:       EdgeHTTP,
		SourceFile: relFile,
		Line:       lineForByte(content, call.StartByte()),
		Language:   "ruby",
		Metadata:   map[string]any{"method": method, "path": fullPath},
	}}
}

func (e *RailsExtractor) extractResources(bt *gotreesitter.BoundTree, call *gotreesitter.Node, lang *gotreesitter.Language, ctx *railsCtx, content []byte, relFile string) []Edge {
	resName := rubyFirstSymbol(bt, call, lang)
	if resName == "" {
		return nil
	}
	only := rubyResourceOnly(bt, call, lang)

	prefix := ctx.urlPrefix()
	urlPrefix := prefix
	if urlPrefix != "" {
		urlPrefix += "/"
	}
	urlPrefix += resName

	ctrlName := controllerName(ctx.modulePrefix(), resName)
	line := lineForByte(content, call.StartByte())

	var edges []Edge
	for _, ra := range railsRestfulActions {
		if only != nil && !only[ra.Action] {
			continue
		}
		path := "/" + urlPrefix
		if ra.Path != "" {
			path += "/" + ra.Path
		}
		source := ra.Method + " " + path
		handler := ctrlName + "#" + ra.Action
		edges = append(edges, Edge{
			SourceNode: source,
			TargetNode: handler,
			Kind:       EdgeHTTP,
			SourceFile: relFile,
			Line:       line,
			Language:   "ruby",
			Metadata:   map[string]any{"method": ra.Method, "path": path},
		})
	}

	if blkBody := rubyCallBlockBody(call, lang); blkBody != nil {
		edges = append(edges, e.walkResourceBlock(blkBody, lang, bt, resName, ctx, content, relFile)...)
	}

	return edges
}

func (e *RailsExtractor) extractNamespace(bt *gotreesitter.BoundTree, call *gotreesitter.Node, lang *gotreesitter.Language, ctx *railsCtx, content []byte, relFile string) []Edge {
	nsName := rubyFirstSymbol(bt, call, lang)
	if nsName == "" {
		return nil
	}
	blkBody := rubyCallBlockBody(call, lang)
	if blkBody == nil {
		return nil
	}

	subCtx := ctx.clone()
	subCtx.namespace = append(subCtx.namespace, nsName)
	return e.walkBody(blkBody, lang, bt, subCtx, content, relFile)
}

func (e *RailsExtractor) extractScope(bt *gotreesitter.BoundTree, call *gotreesitter.Node, lang *gotreesitter.Language, ctx *railsCtx, content []byte, relFile string) []Edge {
	scopePath := rubyFirstStringOrSymbol(bt, call, lang)
	if scopePath == "" {
		return nil
	}
	blkBody := rubyCallBlockBody(call, lang)
	if blkBody == nil {
		return nil
	}

	subCtx := ctx.clone()
	subCtx.scope = append(subCtx.scope, scopePath)
	return e.walkBody(blkBody, lang, bt, subCtx, content, relFile)
}

func (e *RailsExtractor) extractMount(bt *gotreesitter.BoundTree, call *gotreesitter.Node, lang *gotreesitter.Language, ctx *railsCtx, content []byte, relFile string) []Edge {
	path := extractMountPath(bt, call, lang)
	if path == "" {
		return nil
	}
	sourceName := extractMountName(bt, call, lang)
	if sourceName == "" {
		sourceName = "(mounted)"
	}

	source := "ALL " + path
	return []Edge{{
		SourceNode: source,
		TargetNode: sourceName,
		Kind:       EdgeHTTP,
		SourceFile: relFile,
		Line:       lineForByte(content, call.StartByte()),
		Language:   "ruby",
		Metadata:   map[string]any{"method": "ALL", "path": path},
	}}
}

func (e *RailsExtractor) extractRoot(bt *gotreesitter.BoundTree, call *gotreesitter.Node, lang *gotreesitter.Language, ctx *railsCtx, content []byte, relFile string) []Edge {
	handler := extractRootHandler(bt, call, lang)
	if handler == "" {
		return nil
	}
	fullPath := "/"
	if p := ctx.urlPrefix(); p != "" {
		fullPath = "/" + p + "/"
	}
	ctrlHandler := qualifyHandler(ctx.modulePrefix(), handler)
	source := "GET " + fullPath
	return []Edge{{
		SourceNode: source,
		TargetNode: ctrlHandler,
		Kind:       EdgeHTTP,
		SourceFile: relFile,
		Line:       lineForByte(content, call.StartByte()),
		Language:   "ruby",
		Metadata:   map[string]any{"method": "GET", "path": fullPath},
	}}
}

func extractRootHandler(bt *gotreesitter.BoundTree, call *gotreesitter.Node, lang *gotreesitter.Language) string {
	args := call.ChildByFieldName("arguments", lang)
	if args == nil {
		return ""
	}
	for i := 0; i < int(args.ChildCount()); i++ {
		child := args.Child(i)
		if child == nil || child.Type(lang) != "pair" {
			continue
		}
		keyNode := child.ChildByFieldName("key", lang)
		valNode := child.ChildByFieldName("value", lang)
		if keyNode == nil || valNode == nil {
			continue
		}
		if bt.NodeText(keyNode) == "to" {
			if valNode.Type(lang) == "string" {
				return unquote(bt.NodeText(valNode))
			}
		}
	}
	return ""
}

func (e *RailsExtractor) extractDeviseFor(bt *gotreesitter.BoundTree, call *gotreesitter.Node, lang *gotreesitter.Language, ctx *railsCtx, content []byte, relFile string) []Edge {
	resName := rubyFirstSymbol(bt, call, lang)
	if resName == "" {
		return nil
	}
	prefix := ctx.urlPrefix()
	fullPrefix := prefix
	if fullPrefix != "" {
		fullPrefix += "/"
	}
	fullPrefix += resName
	line := lineForByte(content, call.StartByte())

	deviseRoutes := []struct {
		Method string
		Path   string
		Action string
	}{
		{"GET", "/new", "new"},
		{"POST", "", "create"},
		{"GET", "/:id/edit", "edit"},
		{"PATCH", "/:id", "update"},
		{"DELETE", "/:id", "destroy"},
		{"GET", "", "index"},
		{"GET", "/:id", "show"},
		{"POST", "/sign_in", "create"},
		{"DELETE", "/sign_out", "destroy"},
		{"POST", "/password", "create"},
		{"GET", "/password/new", "new"},
		{"GET", "/password/edit", "edit"},
		{"PATCH", "/password", "update"},
		{"PUT", "/password", "update"},
		{"POST", "/confirmation", "create"},
		{"GET", "/confirmation/new", "new"},
		{"GET", "/confirmation", "show"},
		{"POST", "/unlock", "create"},
		{"GET", "/unlock/new", "new"},
		{"GET", "/unlock", "show"},
		{"GET", "/registration/new", "new"},
		{"POST", "/registration", "create"},
		{"DELETE", "/registration", "destroy"},
		{"GET", "/registration", "edit"},
		{"PATCH", "/registration", "update"},
		{"PUT", "/registration", "update"},
	}

	var edges []Edge
	for _, dr := range deviseRoutes {
		path := "/" + fullPrefix + dr.Path
		source := dr.Method + " " + path
		handler := controllerName(ctx.modulePrefix(), resName) + "#" + dr.Action
		edges = append(edges, Edge{
			SourceNode: source,
			TargetNode: handler,
			Kind:       EdgeHTTP,
			SourceFile: relFile,
			Line:       line,
			Language:   "ruby",
			Metadata:   map[string]any{"method": dr.Method, "path": path},
		})
	}
	return edges
}

func isRoutesFile(filePath string) bool {
	return strings.HasSuffix(filepath.ToSlash(filePath), "config/routes.rb")
}

func findDrawBlockBody(bt *gotreesitter.BoundTree, root *gotreesitter.Node, lang *gotreesitter.Language) *gotreesitter.Node {
	if root == nil {
		return nil
	}
	for i := 0; i < int(root.ChildCount()); i++ {
		child := root.Child(i)
		if child == nil || child.Type(lang) != "call" {
			continue
		}
		method := rubyCallMethod(bt, child, lang)
		if method != "draw" {
			continue
		}
		blkBody := rubyCallBlockBody(child, lang)
		if blkBody != nil {
			return blkBody
		}
	}
	return nil
}

func rubyCallMethod(bt *gotreesitter.BoundTree, call *gotreesitter.Node, lang *gotreesitter.Language) string {
	if methodNode := call.ChildByFieldName("method", lang); methodNode != nil {
		return bt.NodeText(methodNode)
	}
	return ""
}

func rubyCallBlockBody(call *gotreesitter.Node, lang *gotreesitter.Language) *gotreesitter.Node {
	blk := call.ChildByFieldName("block", lang)
	if blk == nil {
		return nil
	}
	body := blk.ChildByFieldName("body", lang)
	return body
}

func rubyFirstSymbol(bt *gotreesitter.BoundTree, call *gotreesitter.Node, lang *gotreesitter.Language) string {
	args := call.ChildByFieldName("arguments", lang)
	if args == nil {
		return ""
	}
	for i := 0; i < int(args.ChildCount()); i++ {
		child := args.Child(i)
		if child == nil {
			continue
		}
		if child.Type(lang) == "simple_symbol" {
			return strings.TrimPrefix(bt.NodeText(child), ":")
		}
	}
	return ""
}

func rubyFirstStringOrSymbol(bt *gotreesitter.BoundTree, call *gotreesitter.Node, lang *gotreesitter.Language) string {
	args := call.ChildByFieldName("arguments", lang)
	if args == nil {
		return ""
	}
	for i := 0; i < int(args.ChildCount()); i++ {
		child := args.Child(i)
		if child == nil {
			continue
		}
		t := child.Type(lang)
		if t == "string" {
			return unquote(bt.NodeText(child))
		}
		if t == "simple_symbol" {
			return strings.TrimPrefix(bt.NodeText(child), ":")
		}
	}
	return ""
}

func rubyResourceOnly(bt *gotreesitter.BoundTree, call *gotreesitter.Node, lang *gotreesitter.Language) map[string]bool {
	args := call.ChildByFieldName("arguments", lang)
	if args == nil {
		return nil
	}
	for i := 0; i < int(args.ChildCount()); i++ {
		child := args.Child(i)
		if child == nil || child.Type(lang) != "pair" {
			continue
		}
		keyNode := child.ChildByFieldName("key", lang)
		if keyNode == nil || keyNode.Type(lang) != "hash_key_symbol" {
			continue
		}
		keyText := bt.NodeText(keyNode)
		if keyText != "only" {
			continue
		}
		valNode := child.ChildByFieldName("value", lang)
		if valNode == nil || valNode.Type(lang) != "array" {
			continue
		}
		result := make(map[string]bool)
		for j := 0; j < int(valNode.ChildCount()); j++ {
			elem := valNode.Child(j)
			if elem == nil || elem.Type(lang) != "simple_symbol" {
				continue
			}
			result[strings.TrimPrefix(bt.NodeText(elem), ":")] = true
		}
		return result
	}
	return nil
}

func extractRouteArgs(bt *gotreesitter.BoundTree, call *gotreesitter.Node, lang *gotreesitter.Language) (path, handler string) {
	args := call.ChildByFieldName("arguments", lang)
	if args == nil {
		return "", ""
	}

	for i := 0; i < int(args.ChildCount()); i++ {
		child := args.Child(i)
		if child == nil || child.Type(lang) != "pair" {
			continue
		}
		keyNode := child.ChildByFieldName("key", lang)
		valNode := child.ChildByFieldName("value", lang)
		if keyNode == nil || valNode == nil {
			continue
		}
		if keyNode.Type(lang) == "string" {
			path = unquote(bt.NodeText(keyNode))
			if valNode.Type(lang) == "string" || valNode.Type(lang) == "simple_symbol" {
				handler = unquote(bt.NodeText(valNode))
			} else if valNode.Type(lang) == "scope_resolution" {
				handler = bt.NodeText(valNode)
			}
			return path, handler
		}
	}

	path = ""
	for i := 0; i < int(args.ChildCount()); i++ {
		child := args.Child(i)
		if child == nil {
			continue
		}
		if child.Type(lang) == "string" && path == "" {
			path = unquote(bt.NodeText(child))
		}
		if child.Type(lang) == "pair" {
			keyNode := child.ChildByFieldName("key", lang)
			valNode := child.ChildByFieldName("value", lang)
			if keyNode == nil || valNode == nil {
				continue
			}
			keyText := bt.NodeText(keyNode)
			if keyText == "to" || keyText == "action" || keyText == "controller" {
				if valNode.Type(lang) == "string" {
					if keyText == "to" || keyText == "action" {
						if handler == "" {
							handler = unquote(bt.NodeText(valNode))
						}
					}
				}
			}
		}
	}
	if path != "" && handler != "" {
		return path, handler
	}
	if path != "" && handler == "" {
		return path, handler
	}

	return "", ""
}

func extractMountPath(bt *gotreesitter.BoundTree, call *gotreesitter.Node, lang *gotreesitter.Language) string {
	args := call.ChildByFieldName("arguments", lang)
	if args == nil {
		return ""
	}
	for i := 0; i < int(args.ChildCount()); i++ {
		child := args.Child(i)
		if child == nil || child.Type(lang) != "pair" {
			continue
		}
		valNode := child.ChildByFieldName("value", lang)
		if valNode == nil || valNode.Type(lang) != "string" {
			continue
		}
		return unquote(bt.NodeText(valNode))
	}
	return ""
}

func extractMountName(bt *gotreesitter.BoundTree, call *gotreesitter.Node, lang *gotreesitter.Language) string {
	args := call.ChildByFieldName("arguments", lang)
	if args == nil {
		return ""
	}
	for i := 0; i < int(args.ChildCount()); i++ {
		child := args.Child(i)
		if child == nil || child.Type(lang) != "pair" {
			continue
		}
		keyNode := child.ChildByFieldName("key", lang)
		if keyNode == nil {
			continue
		}
		return bt.NodeText(keyNode)
	}
	return ""
}

func controllerName(modulePrefix, resourceName string) string {
	ctrl := camelCase(resourceName) + "Controller"
	if modulePrefix != "" {
		return modulePrefix + "::" + ctrl
	}
	return ctrl
}

func qualifyHandler(modulePrefix, handler string) string {
	parts := strings.SplitN(handler, "#", 2)
	if len(parts) != 2 {
		return handler
	}
	ctrlPart := parts[0]
	action := parts[1]
	ctrl := controllerName(modulePrefix, ctrlPart)
	return ctrl + "#" + action
}

func camelCase(name string) string {
	parts := strings.Split(name, "_")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, "")
}

func unquote(s string) string {
	if len(s) >= 2 {
		quote := s[0]
		if quote == s[len(s)-1] && (quote == '"' || quote == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
