package graph

import (
	"strings"

	gotreesitter "github.com/odvcencio/gotreesitter"
)

func extractJSDeclarationIdentities(bt *gotreesitter.BoundTree, tree *gotreesitter.Tree, content []byte, filePath string, lang *gotreesitter.Language) []Edge {
	var edges []Edge
	var walk func(*gotreesitter.Node, bool)
	add := func(node *gotreesitter.Node, target string) {
		edges = append(edges, Edge{SourceNode: filePath, TargetNode: target, Kind: EdgeContains, SourceFile: filePath, Line: lineForByte(content, node.StartByte()), Language: "javascript"})
	}
	walk = func(node *gotreesitter.Node, defaultExport bool) {
		if node == nil {
			return
		}
		if node.Type(lang) == "export_statement" && strings.HasPrefix(strings.TrimSpace(bt.NodeText(node)), "export default") {
			defaultExport = true
		}
		if node.Type(lang) == "class_declaration" {
			name := node.ChildByFieldName("name", lang)
			if name != nil {
				className := bt.NodeText(name)
				if body := firstDescendantOfType(node, lang, "class_body"); body != nil {
					for i := 0; i < int(body.ChildCount()); i++ {
						method := body.Child(i)
						methodName := method.ChildByFieldName("name", lang)
						if method.Type(lang) == "method_definition" && methodName != nil && isIdentifier(bt.NodeText(methodName)) {
							add(methodName, filePath+"::"+className+"."+bt.NodeText(methodName))
							if defaultExport {
								add(methodName, filePath+"::default."+bt.NodeText(methodName))
							}
						}
					}
				}
			}
		}
		if defaultExport && node.Type(lang) == "export_statement" {
			add(node, filePath+"::default")
		}
		for i := 0; i < int(node.ChildCount()); i++ {
			walk(node.Child(i), defaultExport)
		}
	}
	walk(tree.RootNode(), false)
	return edges
}
