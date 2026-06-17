package graph

import (
	"testing"

	gotreesitter "github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

func TestGotreesitterNodeStructure(t *testing.T) {
	jsLang := grammars.JavascriptLanguage()

	testCode := `
switch (expr) {
  case 1: break;
}
return value;
`

	parser := gotreesitter.NewParser(jsLang)
	tree, err := parser.Parse([]byte(testCode))
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}
	bt := gotreesitter.Bind(tree)
	defer bt.Release()

	root := tree.RootNode()

	// Inspect switch_statement structure
	walkNodes(root, jsLang, "switch_statement", func(n *gotreesitter.Node) {
		t.Logf("Switch statement child count: %d", n.ChildCount())
		for i := 0; i < int(n.ChildCount()); i++ {
			child := n.Child(i)
			if child != nil {
				t.Logf("  Child %d: type=%s, text=%q", i, child.Type(jsLang), bt.NodeText(child))
			}
		}
	})

	// Inspect return_statement structure
	walkNodes(root, jsLang, "return_statement", func(n *gotreesitter.Node) {
		t.Logf("Return statement child count: %d", n.ChildCount())
		for i := 0; i < int(n.ChildCount()); i++ {
			child := n.Child(i)
			if child != nil {
				t.Logf("  Child %d: type=%s, text=%q", i, child.Type(jsLang), bt.NodeText(child))
			}
		}
	})
}
