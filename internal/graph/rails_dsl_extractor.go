package graph

import (
	"fmt"
	"path/filepath"
	"strings"

	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

var _ Extractor = (*RailsDSLEdgeExtractor)(nil)

var dslAssociationMethods = map[string]bool{
	"has_many":                true,
	"has_one":                 true,
	"belongs_to":              true,
	"has_and_belongs_to_many": true,
}

var dslCallbackMethods = map[string]bool{
	"before_action":  true,
	"after_action":   true,
	"around_action":  true,
	"before_save":    true,
	"after_save":     true,
	"before_create":  true,
	"after_create":   true,
	"before_update":  true,
	"after_update":   true,
	"before_destroy": true,
	"after_destroy":  true,
	"after_commit":   true,
}

var dslConcernMethods = map[string]bool{
	"include": true,
	"extend":  true,
}

var sidekiqSuperclasses = map[string]bool{
	"Sidekiq::Worker": true,
	"Sidekiq::Job":    true,
}

var sidekiqPerformMethods = map[string]bool{
	"perform_async": true,
	"perform_in":    true,
}

type RailsDSLEdgeExtractor struct {
	lang *gotreesitter.Language
}

func NewRailsDSLEdgeExtractor() (*RailsDSLEdgeExtractor, error) {
	return &RailsDSLEdgeExtractor{lang: grammars.RubyLanguage()}, nil
}

func (x *RailsDSLEdgeExtractor) Supports(ext string) bool {
	return ext == ".rb"
}

func (x *RailsDSLEdgeExtractor) RequiresFrameworks() []string {
	return []string{"rails"}
}

func (x *RailsDSLEdgeExtractor) Language() string {
	return "ruby"
}

func (x *RailsDSLEdgeExtractor) ExtractEdges(filePath string, content []byte) ([]Edge, error) {
	parser := gotreesitter.NewParser(x.lang)
	tree, err := parser.Parse(content)
	if err != nil {
		return nil, fmt.Errorf("rails_dsl parse %s: %w", filePath, err)
	}
	bt := gotreesitter.Bind(tree)
	defer bt.Release()

	relFile := filepath.ToSlash(filePath)

	sidekiqWorkers := x.detectSidekiqWorkers(bt, tree.RootNode())

	var edges []Edge
	edges = append(edges, x.extractClassBodyDSL(bt, tree.RootNode(), relFile, content)...)
	edges = append(edges, x.extractSidekiqPerformCalls(bt, tree.RootNode(), relFile, content, sidekiqWorkers)...)

	if len(edges) == 0 {
		return nil, nil
	}
	return edges, nil
}

func (x *RailsDSLEdgeExtractor) detectSidekiqWorkers(bt *gotreesitter.BoundTree, root *gotreesitter.Node) map[string]bool {
	workers := make(map[string]bool)
	walkNodes(root, x.lang, "class", func(n *gotreesitter.Node) {
		className := x.classNameText(bt, n)
		if className == "" {
			return
		}
		if superclass := x.classSuperclassText(bt, n); sidekiqSuperclasses[superclass] {
			workers[className] = true
			return
		}
		body := x.classBody(n)
		if body == nil {
			return
		}
		for i := 0; i < int(body.ChildCount()); i++ {
			child := body.Child(i)
			if child == nil || child.Type(x.lang) != "call" {
				continue
			}
			methodName := rubyCallMethod(bt, child, x.lang)
			if methodName != "include" {
				continue
			}
			if x.callIncludesSidekiq(bt, child) {
				workers[className] = true
				return
			}
		}
	})
	return workers
}

func (x *RailsDSLEdgeExtractor) callIncludesSidekiq(bt *gotreesitter.BoundTree, call *gotreesitter.Node) bool {
	args := call.ChildByFieldName("arguments", x.lang)
	if args == nil {
		return false
	}
	for i := 0; i < int(args.ChildCount()); i++ {
		child := args.Child(i)
		if child == nil {
			continue
		}
		var name string
		t := child.Type(x.lang)
		if t == "constant" {
			name = bt.NodeText(child)
		} else if t == "scope_resolution" {
			name = scopeResolutionText(bt, child)
		}
		if sidekiqSuperclasses[name] {
			return true
		}
	}
	return false
}

func (x *RailsDSLEdgeExtractor) classNameText(bt *gotreesitter.BoundTree, classNode *gotreesitter.Node) string {
	for i := 0; i < int(classNode.ChildCount()); i++ {
		child := classNode.Child(i)
		if child == nil {
			continue
		}
		t := child.Type(x.lang)
		if t == "constant" {
			return bt.NodeText(child)
		}
		if t == "scope_resolution" {
			return scopeResolutionText(bt, child)
		}
	}
	return ""
}

func (x *RailsDSLEdgeExtractor) classSuperclassText(bt *gotreesitter.BoundTree, classNode *gotreesitter.Node) string {
	for i := 0; i < int(classNode.ChildCount()); i++ {
		child := classNode.Child(i)
		if child == nil {
			continue
		}
		if child.Type(x.lang) == "superclass" {
			for j := 0; j < int(child.ChildCount()); j++ {
				inner := child.Child(j)
				if inner == nil {
					continue
				}
				t := inner.Type(x.lang)
				if t == "constant" {
					return bt.NodeText(inner)
				}
				if t == "scope_resolution" {
					return scopeResolutionText(bt, inner)
				}
			}
		}
	}
	return ""
}

func (x *RailsDSLEdgeExtractor) extractClassBodyDSL(bt *gotreesitter.BoundTree, root *gotreesitter.Node, relFile string, content []byte) []Edge {
	var edges []Edge

	walkNodes(root, x.lang, "class", func(n *gotreesitter.Node) {
		className := x.classNameText(bt, n)
		if className == "" {
			return
		}
		body := x.classBody(n)
		if body == nil {
			return
		}
		edges = append(edges, x.walkDSLBody(bt, body, className, relFile, content)...)
	})

	return edges
}

func (x *RailsDSLEdgeExtractor) classBody(classNode *gotreesitter.Node) *gotreesitter.Node {
	for i := 0; i < int(classNode.ChildCount()); i++ {
		child := classNode.Child(i)
		if child == nil {
			continue
		}
		if child.Type(x.lang) == "body_statement" {
			return child
		}
	}
	return nil
}

func (x *RailsDSLEdgeExtractor) walkDSLBody(bt *gotreesitter.BoundTree, body *gotreesitter.Node, className string, relFile string, content []byte) []Edge {
	var edges []Edge

	for i := 0; i < int(body.ChildCount()); i++ {
		child := body.Child(i)
		if child == nil || child.Type(x.lang) != "call" {
			continue
		}
		methodName := rubyCallMethod(bt, child, x.lang)
		if methodName == "" {
			continue
		}

		switch {
		case dslAssociationMethods[methodName]:
			edges = append(edges, x.extractAssociation(bt, child, className, methodName, relFile, content)...)
		case dslCallbackMethods[methodName]:
			edges = append(edges, x.extractCallback(bt, child, className, methodName, relFile, content)...)
		case dslConcernMethods[methodName]:
			edges = append(edges, x.extractConcern(bt, child, className, methodName, relFile, content)...)
		}
	}

	return edges
}

func (x *RailsDSLEdgeExtractor) extractAssociation(bt *gotreesitter.BoundTree, call *gotreesitter.Node, className string, methodName string, relFile string, content []byte) []Edge {
	target := x.firstSymbolArg(bt, call)
	if target == "" {
		return nil
	}
	// Resolve symbol to model class name: :users → User, :user → User
	modelClass := resolveAssociationTarget(target)
	return []Edge{{
		SourceNode: relFile + "::" + className + "#" + methodName,
		TargetNode: modelClass,
		Kind:       EdgeCalls,
		SourceFile: relFile,
		Line:       lineForByte(content, call.StartByte()),
		Language:   "ruby",
		Metadata: map[string]any{
			"dsl":              true,
			"type":             "association",
			"association_type": methodName,
			"original_symbol":  target,
		},
	}}
}

func (x *RailsDSLEdgeExtractor) firstSymbolArg(bt *gotreesitter.BoundTree, call *gotreesitter.Node) string {
	args := call.ChildByFieldName("arguments", x.lang)
	if args == nil {
		return ""
	}
	for i := 0; i < int(args.ChildCount()); i++ {
		child := args.Child(i)
		if child != nil && child.Type(x.lang) == "simple_symbol" {
			return strings.TrimPrefix(bt.NodeText(child), ":")
		}
	}
	return ""
}

// resolveAssociationTarget converts a Rails association symbol to a model class name.
// Example: "users" → "User", "profile" → "Profile"
func resolveAssociationTarget(symbol string) string {
	singular := singularize(symbol)
	return capitalize(singular)
}

// singularize converts a plural noun to singular using common Rails inflection rules.
func singularize(word string) string {
	if strings.HasSuffix(word, "ies") {
		return strings.TrimSuffix(word, "ies") + "y"
	}
	if strings.HasSuffix(word, "ses") || strings.HasSuffix(word, "xes") || strings.HasSuffix(word, "zes") {
		return strings.TrimSuffix(word, "es")
	}
	if strings.HasSuffix(word, "s") && !strings.HasSuffix(word, "ss") && !strings.HasSuffix(word, "us") && !strings.HasSuffix(word, "is") {
		return strings.TrimSuffix(word, "s")
	}
	return word
}

// capitalize converts the first letter of a word to uppercase.
func capitalize(word string) string {
	if word == "" {
		return word
	}
	runes := []rune(word)
	return strings.ToUpper(string(runes[0])) + string(runes[1:])
}

func (x *RailsDSLEdgeExtractor) extractCallback(bt *gotreesitter.BoundTree, call *gotreesitter.Node, className string, methodName string, relFile string, content []byte) []Edge {
	action := x.firstSymbolArg(bt, call)
	if action == "" {
		return nil
	}
	// Qualify callback method name with controller class: :authenticate → UsersController#authenticate
	qualifiedAction := className + "#" + action
	return []Edge{{
		SourceNode: relFile + "::" + className + "#" + methodName,
		TargetNode: qualifiedAction,
		Kind:       EdgeMiddleware,
		SourceFile: relFile,
		Line:       lineForByte(content, call.StartByte()),
		Language:   "ruby",
		Metadata: map[string]any{
			"dsl":           true,
			"type":          "callback",
			"callback_type": methodName,
			"original_method": action,
		},
	}}
}

func (x *RailsDSLEdgeExtractor) extractConcern(bt *gotreesitter.BoundTree, call *gotreesitter.Node, className string, methodName string, relFile string, content []byte) []Edge {
	moduleName := x.firstConstantArg(bt, call)
	if moduleName == "" {
		return nil
	}
	return []Edge{{
		SourceNode: relFile + "::" + className + "#" + methodName,
		TargetNode: moduleName,
		Kind:       EdgeCalls,
		SourceFile: relFile,
		Line:       lineForByte(content, call.StartByte()),
		Language:   "ruby",
		Metadata: map[string]any{
			"dsl":          true,
			"type":         "concern",
			"concern_type": methodName,
		},
	}}
}

func (x *RailsDSLEdgeExtractor) firstConstantArg(bt *gotreesitter.BoundTree, call *gotreesitter.Node) string {
	args := call.ChildByFieldName("arguments", x.lang)
	if args == nil {
		return ""
	}
	for i := 0; i < int(args.ChildCount()); i++ {
		child := args.Child(i)
		if child == nil {
			continue
		}
		t := child.Type(x.lang)
		if t == "constant" {
			return bt.NodeText(child)
		}
		if t == "scope_resolution" {
			return scopeResolutionText(bt, child)
		}
	}
	return ""
}

func (x *RailsDSLEdgeExtractor) extractSidekiqPerformCalls(bt *gotreesitter.BoundTree, root *gotreesitter.Node, relFile string, content []byte, sidekiqWorkers map[string]bool) []Edge {
	var edges []Edge

	walkNodes(root, x.lang, "call", func(n *gotreesitter.Node) {
		methodName := rubyCallMethod(bt, n, x.lang)
		if !sidekiqPerformMethods[methodName] {
			return
		}

		receiver := x.callReceiver(bt, n)
		if receiver == "" {
			return
		}

		workerClass := resolveWorkerClass(receiver, sidekiqWorkers)
		if workerClass == "" {
			return
		}

		callerMethod := x.findEnclosingMethodName(bt, n)

		sourceNode := relFile + "::" + callerMethod + "#" + methodName
		targetNode := workerClass + "#perform"

		edges = append(edges, Edge{
			SourceNode: sourceNode,
			TargetNode: targetNode,
			Kind:       EdgeIntegration,
			SourceFile: relFile,
			Line:       lineForByte(content, n.StartByte()),
			Language:   "ruby",
			Metadata: map[string]any{
				"kind":         "sidekiq_job",
				"worker_class": workerClass,
			},
		})
	})

	return edges
}

func (x *RailsDSLEdgeExtractor) callReceiver(bt *gotreesitter.BoundTree, call *gotreesitter.Node) string {
	methodNode := call.ChildByFieldName("method", x.lang)
	if methodNode == nil {
		return ""
	}
	for i := 0; i < int(call.ChildCount()); i++ {
		child := call.Child(i)
		if child == nil || child == methodNode {
			break
		}
		t := child.Type(x.lang)
		if t == "constant" {
			return bt.NodeText(child)
		}
		if t == "scope_resolution" {
			return scopeResolutionText(bt, child)
		}
		if t == "identifier" {
			return bt.NodeText(child)
		}
	}
	return ""
}

func (x *RailsDSLEdgeExtractor) findEnclosingMethodName(bt *gotreesitter.BoundTree, n *gotreesitter.Node) string {
	current := n.Parent()
	for current != nil {
		t := current.Type(x.lang)
		if t == "method" || t == "singleton_method" {
			nameNode := current.ChildByFieldName("name", x.lang)
			if nameNode != nil {
				return bt.NodeText(nameNode)
			}
		}
		current = current.Parent()
	}
	return ""
}

func resolveWorkerClass(receiver string, sidekiqWorkers map[string]bool) string {
	if sidekiqWorkers[receiver] {
		return receiver
	}
	if idx := strings.LastIndex(receiver, "::"); idx >= 0 {
		suffix := receiver[idx+2:]
		if sidekiqWorkers[suffix] {
			return suffix
		}
	}
	return ""
}

func scopeResolutionText(bt *gotreesitter.BoundTree, n *gotreesitter.Node) string {
	var parts []string
	for i := 0; i < int(n.ChildCount()); i++ {
		child := n.Child(i)
		if child == nil {
			continue
		}
		text := bt.NodeText(child)
		if text != "::" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "::")
}
